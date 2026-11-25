package file2

import (
	"encoding/binary"
	"errors"
)

// JournalChunk decodes only the measured aftergame channel: 64x32 western or
// 32x64 Japanese blocks of the same opaque 2048-byte native play journal.
// References: original captures journal-pal-{1,2}, journal-jp-lobby-1;
// NM-OUTBREAK's historical admission_host.py and NativeFile2ForeignAdmission.
func JournalChunk(p []byte, japanese bool) (more bool, chunk []byte, err error) {
	at, step := 0, 64
	if !japanese {
		step = 32
		if len(p) != 56 || binary.BigEndian.Uint32(p) != 1 || binary.LittleEndian.Uint32(p[8:]) != 1 {
			return false, nil, errors.New("file2: invalid western journal")
		}
		at = 12
	}
	if len(p) != at+12+step || binary.LittleEndian.Uint32(p[at:]) != 1 || binary.LittleEndian.Uint32(p[at+4:]) > 1 || binary.LittleEndian.Uint32(p[at+8:]) != uint32(step) {
		return false, nil, errors.New("file2: invalid journal chunk")
	}
	return binary.LittleEndian.Uint32(p[at+4:]) == 1, append([]byte(nil), p[at+12:]...), nil
}

func JournalBlocks(data []byte, japanese bool) ([][]byte, error) {
	if len(data) != 2048 {
		return nil, errors.New("file2: incomplete journal")
	}
	step, at := 32, 12
	if japanese {
		step, at = 64, 0
	}
	var blocks [][]byte
	for pos := 0; pos < len(data); pos += step {
		p := make([]byte, at+12+step)
		if !japanese {
			binary.BigEndian.PutUint32(p, 1)
			binary.LittleEndian.PutUint32(p[8:], 1)
		}
		binary.LittleEndian.PutUint32(p[at:], 1)
		if pos+step < len(data) {
			binary.LittleEndian.PutUint32(p[at+4:], 1)
		}
		binary.LittleEndian.PutUint32(p[at+8:], uint32(step))
		copy(p[at+12:], data[pos:pos+step])
		blocks = append(blocks, p)
	}
	return blocks, nil
}
