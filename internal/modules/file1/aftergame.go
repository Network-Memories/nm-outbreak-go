package file1

import "errors"

// AftergameProfile marks a proven Japanese return in the File #1 after-game lobby.
// Measured (go-f1-us-us-aftergame.pcap, 23/11/2026): each original US client resends
// its 240-byte property on reaching the MEETING ROOM with byte 212 changed from 0 to 1;
// the File #2 equivalent is byte 221. Without it the US host counts the guest out (1P).
func AftergameProfile(profile []byte) ([]byte, error) {
	if len(profile) != USProfileSize {
		return nil, errors.New("file1: expected western profile")
	}
	out := append([]byte(nil), profile...)
	out[212] = 1
	return out, nil
}
