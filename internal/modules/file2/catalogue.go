package file2

import (
	"encoding/binary"
	"fmt"

	"networkmemories/nm-outbreak-go/internal/modules"
)

const JPCatalogueSize = 605 + 4 + 605

// ConfigureJPCatalogue installs the three File2 blocks measured with BioServer2.
// The caller supplies the local fixture. The module owns an immutable copy.
func ConfigureJPCatalogue(def *modules.Definition, raw []byte) error {
	if len(raw) != JPCatalogueSize {
		return fmt.Errorf("File2 JAP catalogue has %d bytes, want %d", len(raw), JPCatalogueSize)
	}
	owned := append([]byte(nil), raw...)
	blocks := [3][]byte{owned[:605], owned[605:609], owned[609:]}
	manifest := make([]byte, 13)
	manifest[0] = byte(len(blocks))
	for i, block := range blocks {
		binary.BigEndian.PutUint32(manifest[1+4*i:], uint32(len(block)))
	}
	def.KDDIDownloadManifest = manifest
	def.KDDIDownloadBlock = func(request []byte) []byte {
		if len(request) != 9 || int(request[0]) >= len(blocks) {
			return nil
		}
		block := blocks[request[0]]
		offset := binary.BigEndian.Uint32(request[1:])
		size := binary.BigEndian.Uint32(request[5:])
		if offset >= uint32(len(block)) || size == 0 || size > 512 {
			return nil
		}
		size = min(size, uint32(len(block))-offset)
		// The legacy envelope pads the reply to the full length of the block.
		reply := make([]byte, 11+len(block))
		copy(reply, request[:5])
		binary.BigEndian.PutUint32(reply[5:], size)
		binary.BigEndian.PutUint16(reply[9:], uint16(size))
		copy(reply[11:], block[offset:offset+size])
		return reply
	}
	return nil
}
