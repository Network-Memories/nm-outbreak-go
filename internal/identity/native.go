package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SNAPCredential struct {
	Account  Account `json:"account"`
	Seed     string  `json:"-"`
	MagicKey []byte  `json:"-"`
}
type SNAPAuthority interface {
	SNAPCredentials(string) (SNAPCredential, error)
}

var ErrNativeMaterial = errors.New("native protocol material unavailable; a proved password login is needed")

type nativeMaterial struct {
	Seed     string
	MagicKey []byte
}

func (l *Local) ensureNativeKeyLocked() error {
	if len(l.nativeKey) == 32 {
		return nil
	}
	key, err := os.ReadFile(l.path + ".native-key")
	if err == nil {
		if len(key) != 32 {
			return ErrNativeMaterial
		}
		l.nativeKey = key
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path+".native-key", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	valid := false
	defer func() {
		f.Close()
		if !valid {
			os.Remove(l.path + ".native-key")
		}
	}()
	if _, err := f.Write(key); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	valid = true
	l.nativeKey = key
	return nil
}

func nativeAAD(account Account) []byte {
	return []byte(strconv.FormatInt(account.UserID, 10) + ":" + account.NativeAlias)
}
func (l *Local) sealNativeLocked(account Account, password string) ([]byte, error) {
	if err := l.ensureNativeKeyLocked(); err != nil {
		return nil, err
	}
	var seedRaw [16]byte
	if _, err := rand.Read(seedRaw[:]); err != nil {
		return nil, err
	}
	seed := hex.EncodeToString(seedRaw[:])
	magic := sha1.Sum([]byte(password + seed)) // Required by the original SNAP client, not used for the primary password hash.
	raw, err := json.Marshal(nativeMaterial{seed, magic[:]})
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(l.nativeKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, raw, nativeAAD(account)), nil
}
func (l *Local) SNAPCredentials(name string) (SNAPCredential, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	index, ok := l.names[strings.ToLower(name)]
	if !ok {
		return SNAPCredential{}, ErrAuthentication
	}
	record := l.records[index]
	if len(l.nativeKey) != 32 || len(record.NativeSealed) == 0 {
		return SNAPCredential{}, ErrNativeMaterial
	}
	block, err := aes.NewCipher(l.nativeKey)
	if err != nil {
		return SNAPCredential{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SNAPCredential{}, err
	}
	sealed := record.NativeSealed
	if len(sealed) < gcm.NonceSize() {
		return SNAPCredential{}, ErrNativeMaterial
	}
	raw, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nativeAAD(record.Account))
	if err != nil {
		return SNAPCredential{}, ErrNativeMaterial
	}
	var material nativeMaterial
	if json.Unmarshal(raw, &material) != nil || len(material.Seed) != 32 || len(material.MagicKey) != 20 {
		return SNAPCredential{}, ErrNativeMaterial
	}
	return SNAPCredential{record.Account, material.Seed, material.MagicKey}, nil
}

// PrepareSNAP upgrades an older local record only after proof of its current
// password. Alias, handle, owner and primary password hash are not changed.
func (l *Local) PrepareSNAP(name, password string) (*Account, error) {
	account, err := l.Authenticate(name, password)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	index, ok := l.names[strings.ToLower(name)]
	if !ok {
		return nil, ErrAuthentication
	}
	if len(l.records[index].NativeSealed) > 0 {
		return account, nil
	}
	sealed, err := l.sealNativeLocked(*account, password)
	if err != nil {
		return nil, err
	}
	records := append([]localRecord(nil), l.records...)
	records[index].NativeSealed = sealed
	if err := l.persist(records); err != nil {
		return nil, err
	}
	l.records = records
	return account, nil
}
