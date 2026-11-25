// SNAP bootstrap framing ported from openSNAP core/bootstrap/handlers.py (AGPL-3.0).
// Blowfish is required only for compatibility with the unmodified native client.
package snapauth

import (
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"golang.org/x/crypto/blowfish"
	"net"
)

var BootstrapKey = []byte("SNAP-SWAN")

func Encrypt(key, plain []byte) ([]byte, error) {
	block, err := blowfish.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, (len(plain)+7)/8*8)
	copy(out, plain)
	for at := 0; at < len(out); at += 8 {
		block.Encrypt(out[at:at+8], out[at:at+8])
	}
	return out, nil
}
func Decrypt(key, raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw)%8 != 0 {
		return nil, errors.New("invalid native Blowfish block length")
	}
	block, err := blowfish.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), raw...)
	for at := 0; at < len(out); at += 8 {
		block.Decrypt(out[at:at+8], out[at:at+8])
	}
	return out, nil
}
func Challenge(magic []byte, seed, secret string, port uint16) ([]byte, error) {
	if len(magic) != 20 || len(seed) < 1 || len(seed) > 128 || len(secret) < 1 || len(secret) > 127 {
		return nil, errors.New("invalid native challenge material")
	}
	packedSecret := make([]byte, 128)
	copy(packedSecret, secret)
	inner, err := Encrypt(magic, packedSecret)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 280)
	binary.BigEndian.PutUint16(raw[2:], port)
	binary.BigEndian.PutUint32(raw[4:], uint32(len(seed)))
	copy(raw[8:136], seed)
	binary.BigEndian.PutUint32(raw[136:], 128)
	binary.BigEndian.PutUint32(raw[140:], 128)
	copy(raw[148:276], inner)
	return Encrypt(BootstrapKey, raw)
}
func VerifyAnswer(raw []byte, secret string) bool {
	clear, err := Decrypt(BootstrapKey, raw)
	if err != nil || len(clear) < 9 {
		return false
	}
	end := 8
	for end < len(clear) && clear[end] != 0 {
		end++
	}
	return subtle.ConstantTimeCompare(clear[8:end], []byte(secret)) == 1
}
func Success(login, host string, port uint16) ([]byte, error) {
	ip := net.ParseIP(host).To4()
	if ip == nil || len(login) < 1 || len(login) > 39 || port == 0 {
		return nil, errors.New("invalid native game destination")
	}
	raw := make([]byte, 64)
	copy(raw[:40], login)
	copy(raw[40:44], ip)
	binary.BigEndian.PutUint32(raw[44:], uint32(port))
	binary.BigEndian.PutUint32(raw[48:], uint32(port))
	return Encrypt(BootstrapKey, raw)
}
