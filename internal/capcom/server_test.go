package capcom

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
)

func TestPythonReferenceReplies(t *testing.T) {
	var rows []struct {
		Build          byte
		Command        uint16
		Sequence       byte
		Payload, Reply string
	}
	raw, err := os.ReadFile("testdata/python-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer("127.0.0.1:0", "127.0.0.1", nil, nil, file1.AppDefinition(), file2.AppDefinition())
	if err != nil {
		t.Fatal(err)
	}
	pages := map[byte]map[string][]byte{}
	for _, r := range rows {
		if pages[r.Build] == nil {
			pages[r.Build] = map[string][]byte{}
		}
		p, _ := hex.DecodeString(r.Payload)
		want, _ := hex.DecodeString(r.Reply)
		got, err := s.reply(s.titles[r.Build], Frame{Command: r.Command, Sequence: r.Sequence, Payload: p}, "Fixture", pages[r.Build])
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("build %d command %04x: err=%v\ngot %x\nwant %x", r.Build, r.Command, err, got, want)
		}
	}
}
func TestAPPConnectionLifecycleAndOpaqueArchive(t *testing.T) {
	for _, build := range []byte{1, 4, 6} {
		t.Run(string([]byte{'0' + build}), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "private", "reports.jsonl")
			s, err := NewServer("127.0.0.1:0", "127.0.0.1", &FileSink{Path: path}, nil, file1.AppDefinition(), file2.AppDefinition())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a, b := net.Pipe()
			defer b.Close()
			done := make(chan struct{})
			go func() { s.serve(ctx, a); close(done) }()
			b.SetDeadline(time.Now().Add(2 * time.Second))
			opened, err := ReadFrame(b)
			if err != nil || opened.Command != 0x1002 || !bytes.Equal(opened.Payload, []byte{0, 0}) {
				t.Fatal("native opening missing", err)
			}
			seq := byte(3)
			p := append(field([]byte("ClaimedName"), seq), build, 1)
			p = append(p, field([]byte("0404141723"), seq)...)
			p = append(p, make([]byte, 4)...)
			if err = send(b, Frame{Direction: 2, Command: 0x1002, Sequence: seq, Tail: 0xff, Payload: p}); err != nil {
				t.Fatal(err)
			}
			version, err := ReadFrame(b)
			if err != nil || version.Command != 0x6001 || version.Direction != 0x10 || !bytes.Equal(version.Payload, []byte{0}) {
				t.Fatal("must explicitly offer NO executable patch", err)
			}
			// Coalesced native frames remain distinct and preserve request sequence.
			var batch []byte
			for _, f := range []Frame{{Direction: 1, Command: 0x6401, Sequence: 4, Tail: 0xff}, {Direction: 1, Command: 0x6301, Sequence: 5, Tail: 0xff, Payload: []byte{0x12, 0x34}}} {
				raw, _ := f.Encode()
				batch = append(batch, raw...)
			}
			go func() { _, _ = b.Write(batch) }()
			reply, err := ReadFrame(b)
			if err != nil || reply.Command != 0x6401 || reply.Sequence != 4 || reply.Error != 0 {
				t.Fatal("notice reply", err)
			}
			reply, err = ReadFrame(b)
			if err != nil || reply.Command != 0x6301 || reply.Sequence != 5 || reply.Error != 0 {
				t.Fatal("archive reply", err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var report Report
			if json.Unmarshal(raw, &report) != nil || report.ClaimedLogin != "ClaimedName" || !bytes.Equal(report.Payload, []byte{0x12, 0x34}) {
				t.Fatal("opaque report lost")
			}
			st, _ := os.Stat(path)
			if st.Mode().Perm() != 0600 {
				t.Fatal("public report archive")
			}
			if err = send(b, Frame{Direction: 1, Command: 0x1004, Sequence: 6, Tail: 0xff}); err != nil {
				t.Fatal(err)
			}
			reply, err = ReadFrame(b)
			if err != nil || reply.Command != 0x1004 || reply.Sequence != 6 {
				t.Fatal("complete reply", err)
			}
			b.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("connection did not close")
			}
		})
	}
}
func TestFieldsAndMalformedRequests(t *testing.T) {
	for _, seq := range []byte{0, 3, 255} {
		p := []byte{0, 1, 127, 128, 255}
		encoded, err := EncodeField(p, seq)
		if err != nil {
			t.Fatal(err)
		}
		got, n, err := DecodeField(encoded, seq)
		if err != nil || n != len(encoded) || !bytes.Equal(got, p) {
			t.Fatal("field mismatch")
		}
		encoded[2] ^= 1
		if _, _, err = DecodeField(encoded, seq); err == nil {
			t.Fatal("bad checksum accepted")
		}
	}
	s, _ := NewServer("127.0.0.1:0", "127.0.0.1", nil, nil, file2.AppDefinition())
	for _, cmd := range []uint16{0x6101, 0x6102, 0x6204, 0x6501, 0x6503, 0x6504, 0x6510, 0x6301, 0x6002} {
		if _, err := s.reply(s.titles[4], Frame{Command: cmd}, "Claim", map[string][]byte{}); err == nil {
			t.Fatalf("malformed or forbidden command %04x accepted", cmd)
		}
	}
	if _, err := NewServer("127.0.0.1:0", "0.0.0.0", nil, nil, modules.AppDefinition{}); err == nil {
		t.Fatal("unspecified advertise address accepted")
	}
}
func TestFrameFragmentationAndTruncation(t *testing.T) {
	f := Frame{Direction: 1, Command: 0x6101, Sequence: 255, Tail: 0xff, Payload: []byte{1, 2, 3}}
	raw, _ := f.Encode()
	r := &oneByteReader{data: raw}
	got, err := ReadFrame(r)
	if err != nil || got.Command != f.Command || !bytes.Equal(got.Payload, f.Payload) {
		t.Fatal("fragmentation", err)
	}
	for i := 0; i < len(raw); i++ {
		if _, err = ReadFrame(bytes.NewReader(raw[:i])); err == nil {
			t.Fatal("truncated frame accepted")
		}
	}
}

type oneByteReader struct{ data []byte }

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}
