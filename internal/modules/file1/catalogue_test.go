package file1

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestJPCatalogueMeasuredReads(t *testing.T) {
	def := Definition()
	raw := bytes.Repeat([]byte{0x42}, JPCatalogueSize)
	if err := ConfigureJPCatalogue(&def, raw); err != nil {
		t.Fatalf("ConfigureJPCatalogue: %v", err)
	}
	if !bytes.Equal(def.KDDIDownloadManifest, []byte{1, 0, 0, 0x12, 0x5d}) {
		t.Fatalf("manifest = %x, want 010000125d", def.KDDIDownloadManifest)
	}
	req := make([]byte, 9)
	req[0] = 0
	binary.BigEndian.PutUint32(req[1:], 4600)
	binary.BigEndian.PutUint32(req[5:], 256)
	reply := def.KDDIDownloadBlock(req)
	if len(reply) != 11+256 {
		t.Fatalf("reply len = %d, want %d", len(reply), 11+256)
	}
	if got := binary.BigEndian.Uint32(reply[5:9]); got != 101 {
		t.Fatalf("clamped size = %d, want 101", got)
	}
	if got := binary.BigEndian.Uint16(reply[9:11]); got != 101 {
		t.Fatalf("short size = %d, want 101", got)
	}
	if !bytes.Equal(reply[11:11+101], bytes.Repeat([]byte{0x42}, 101)) {
		t.Fatalf("copied tail mismatch")
	}
	req[0] = 1
	if def.KDDIDownloadBlock(req) != nil {
		t.Fatalf("expected nil for out-of-range block index")
	}
}
