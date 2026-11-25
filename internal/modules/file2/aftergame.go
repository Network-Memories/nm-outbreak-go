package file2

import "errors"

// AftergameProfile projects the native File2 meeting-room marker at +0xdd.
// Original PAL and US uploads change it to one after returning (capture27);
// the PAL property callback at 0x5bc504 compares this byte against one.
// The common engine must prove the Japanese player's return before calling.
// No result words, play time or event log are fabricated here.
func AftergameProfile(profile []byte) ([]byte, error) {
	if len(profile) != WesternProfileSz {
		return nil, errors.New("file2: expected western profile")
	}
	out := append([]byte(nil), profile...)
	out[221] = 1
	return out, nil
}
