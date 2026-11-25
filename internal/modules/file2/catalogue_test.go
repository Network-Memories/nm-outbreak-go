package file2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestJPCatalogueMeasuredReads(t *testing.T) {
	def := Definition()
	raw := bytes.Repeat([]byte{0x42}, JPCatalogueSize)
	if err := ConfigureJPCatalogue(&def, raw); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	if !bytes.Equal(def.KDDIDownloadManifest, []byte{3, 0, 0, 2, 93, 0, 0, 0, 4, 0, 0, 2, 93}) {
		t.Fatal("unexpected manifest")
	}
	for _, tc := range []struct {
		block                   byte
		offset, request, actual uint32
		length                  int
	}{
		{0, 0, 512, 512, 616}, {0, 512, 512, 93, 616}, {1, 0, 512, 4, 15}, {2, 0, 512, 512, 616}, {2, 512, 512, 93, 616},
		{3, 0, 512, 0, 0}, {0, 605, 512, 0, 0}, {0, 0, 513, 0, 0},
	} {
		q := make([]byte, 9)
		q[0] = tc.block
		binary.BigEndian.PutUint32(q[1:], tc.offset)
		binary.BigEndian.PutUint32(q[5:], tc.request)
		r := def.KDDIDownloadBlock(q)
		if len(r) != tc.length {
			t.Fatalf("block %d offset %d: length %d", tc.block, tc.offset, len(r))
		}
		if tc.length == 0 {
			continue
		}
		if !bytes.Equal(r[:5], q[:5]) || binary.BigEndian.Uint32(r[5:]) != tc.actual || binary.BigEndian.Uint16(r[9:]) != uint16(tc.actual) ||
			!bytes.Equal(r[11:11+tc.actual], bytes.Repeat([]byte{0x42}, int(tc.actual))) ||
			!bytes.Equal(r[11+tc.actual:], make([]byte, len(r)-11-int(tc.actual))) {
			t.Fatalf("block %d offset %d: unexpected response", tc.block, tc.offset)
		}
	}
}
