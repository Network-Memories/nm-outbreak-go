package file2

import (
	"bytes"
	"testing"
)

func TestJournalRedividesOnlyActualBytes(t *testing.T) {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i * 17)
	}
	for _, jp := range []bool{false, true} {
		blocks, err := JournalBlocks(data, jp)
		if err != nil {
			t.Fatal(err)
		}
		var decoded []byte
		for i, p := range blocks {
			more, chunk, err := JournalChunk(p, jp)
			if err != nil || more != (i < len(blocks)-1) {
				t.Fatal("native framing", jp, i, err)
			}
			decoded = append(decoded, chunk...)
			bad := append([]byte(nil), p...)
			bad[len(bad)-len(chunk)-4] ^= 1
			if _, _, err = JournalChunk(bad, jp); err == nil {
				t.Fatal("invalid size accepted")
			}
			if _, _, err = JournalChunk(p[:len(p)-1], jp); err == nil {
				t.Fatal("truncation accepted")
			}
		}
		if !bytes.Equal(decoded, data) {
			t.Fatal("opaque journal changed")
		}
	}
	if _, err := JournalBlocks(data[:2047], true); err == nil {
		t.Fatal("incomplete journal emitted")
	}
}
