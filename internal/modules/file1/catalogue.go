package file1

import (
	"encoding/binary"
	"fmt"

	"networkmemories/nm-outbreak-go/internal/modules"
)

// JPCatalogueSize is the single File1 block length (0x125D) measured with BioServer1.
const JPCatalogueSize = 4701

// ConfigureJPCatalogue installs the single File1 block measured with BioServer1.
// The caller supplies the local fixture. The module owns an immutable copy.
func ConfigureJPCatalogue(def *modules.Definition, raw []byte) error {
	if len(raw) != JPCatalogueSize {
		return fmt.Errorf("File1 JAP catalogue has %d bytes, want %d", len(raw), JPCatalogueSize)
	}
	block := append([]byte(nil), raw...)
	manifest := make([]byte, 5)
	manifest[0] = 1
	binary.BigEndian.PutUint32(manifest[1:], uint32(len(block)))
	def.KDDIDownloadManifest = manifest
	def.KDDIDownloadBlock = func(request []byte) []byte {
		if len(request) != 9 || request[0] != 0 {
			return nil
		}
		offset := binary.BigEndian.Uint32(request[1:])
		reqSize := binary.BigEndian.Uint32(request[5:])
		if offset >= uint32(len(block)) || reqSize == 0 || reqSize > 512 {
			return nil
		}
		size := min(reqSize, uint32(len(block))-offset)
		// BioServer1 allocates 11 + requested size and clamps the copied length.
		reply := make([]byte, 11+reqSize)
		copy(reply, request[:5])
		binary.BigEndian.PutUint32(reply[5:], size)
		binary.BigEndian.PutUint16(reply[9:], uint16(size))
		copy(reply[11:], block[offset:offset+size])
		return reply
	}
	return nil
}
