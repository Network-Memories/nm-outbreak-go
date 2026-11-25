package core

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

type roomSecret struct {
	salt   [16]byte
	digest [32]byte
}

func newRoomSecret(password []byte) (roomSecret, error) {
	var s roomSecret
	if len(password) > 64 {
		return s, errors.New("native room password too long")
	}
	if _, err := rand.Read(s.salt[:]); err != nil {
		return s, err
	}
	s.digest = sha256.Sum256(append(s.salt[:], password...))
	return s, nil
}
func (s roomSecret) matches(password []byte) bool {
	candidate := sha256.Sum256(append(s.salt[:], password...))
	return subtle.ConstantTimeCompare(candidate[:], s.digest[:]) == 1
}
