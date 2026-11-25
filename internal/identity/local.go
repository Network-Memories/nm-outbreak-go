package identity

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const localIterations = 600000

var ErrNameTaken = errors.New("name or native alias already assigned")
var ErrAuthentication = errors.New("authentication rejected")

// Authority separates identity storage from the two native transports.
// A standalone installation never contacts NM and never serves as a fallback for NM outages.
type Authority interface {
	Authenticate(username, password string) (*Account, error)
}

type localRecord struct {
	Account      Account `json:"account"`
	Salt         []byte  `json:"salt"`
	Hash         []byte  `json:"hash"`
	NativeSealed []byte  `json:"native_material,omitempty"`
}
type localFile struct {
	Version  int           `json:"version"`
	Accounts []localRecord `json:"accounts"`
}

// Local is the single-process standalone authority. The private file must not be
// shared by concurrent server processes; all game modules use this same instance.
type Local struct {
	mu        sync.RWMutex
	path      string
	records   []localRecord
	names     map[string]int
	handles   map[string]bool
	nextID    int64
	nativeKey []byte
}

func OpenLocal(path string) (*Local, error) {
	if path == "" {
		return nil, errors.New("standalone account file required")
	}
	l := &Local{path: path, names: make(map[string]int), handles: make(map[string]bool), nextID: 1}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var data localFile
	dec := json.NewDecoder(io.LimitReader(f, 16<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("invalid standalone account file: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing account data")
	}
	if data.Version != 1 && data.Version != 2 {
		return nil, errors.New("unsupported standalone account version")
	}
	for _, rec := range data.Accounts {
		a := rec.Account
		if a.UserID < 1 || a.UserID > 0xffffffff || !localName(a.Username) || !nativeName(a.NativeAlias, 15) || !nativeName(a.NativeNickname, 10) || len(a.Handle) != 6 || len(rec.Salt) != 16 || len(rec.Hash) != 32 {
			return nil, errors.New("invalid standalone identity")
		}
		if l.handles[a.Handle] {
			return nil, errors.New("duplicate standalone handle")
		}
		for _, c := range a.Handle {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", c) {
				return nil, errors.New("invalid standalone handle")
			}
		}
		for _, old := range l.records {
			if old.Account.UserID == a.UserID {
				return nil, errors.New("duplicate standalone owner")
			}
		}
		for _, name := range []string{a.Username, a.NativeAlias} {
			k := strings.ToLower(name)
			if index, ok := l.names[k]; ok && index != len(l.records) {
				return nil, errors.New("duplicate standalone name")
			}
			l.names[k] = len(l.records)
		}
		l.handles[a.Handle] = true
		l.records = append(l.records, rec)
		if a.UserID >= l.nextID {
			l.nextID = a.UserID + 1
		}
	}
	for _, record := range l.records {
		if len(record.NativeSealed) > 0 {
			key, err := os.ReadFile(path + ".native-key")
			if err != nil || len(key) != 32 {
				return nil, ErrNativeMaterial
			}
			l.nativeKey = key
			break
		}
	}
	return l, nil
}

func nativeName(name string, max int) bool {
	if len(name) < 1 || len(name) > max {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}
func localName(name string) bool { return len(name) >= MinUsernameLen && nativeName(name, 32) }

// Register persists one owner, alias and random native HN in a single atomic write.
// Credentials remain local and hashed; no clear password or NM credential is imported.
func (l *Local) Register(username, password string) (*Account, error) {
	if err := ValidateNewCredentials(username, password); err != nil {
		return nil, err
	}
	if !localName(username) {
		return nil, errors.New("username must contain 5 to 32 ASCII letters, digits or underscores")
	}
	if strings.ContainsRune(password, 0) {
		return nil, errors.New("password must not contain NUL")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, localIterations, 32)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.names[strings.ToLower(username)]; ok {
		return nil, ErrNameTaken
	}
	if l.nextID > 0xffffffff {
		return nil, errors.New("standalone ID space exhausted")
	}
	alias := username
	if !nativeName(alias, 15) {
		alias = ""
		for i := 0; i < 64; i++ {
			candidate := fmt.Sprintf("NM%013X", l.nextID)
			if i > 0 {
				candidate = fmt.Sprintf("NM%011X%02X", l.nextID, i)
			}
			if _, ok := l.names[strings.ToLower(candidate)]; !ok {
				alias = candidate
				break
			}
		}
		if alias == "" {
			return nil, ErrNameTaken
		}
	}
	handle := ""
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for attempt := 0; attempt < 64; attempt++ {
		var raw [6]byte
		for i := range raw {
			v, err := rand.Int(rand.Reader, big.NewInt(36))
			if err != nil {
				return nil, err
			}
			raw[i] = alphabet[v.Int64()]
		}
		candidate := string(raw[:])
		if !l.handles[candidate] {
			handle = candidate
			break
		}
	}
	if handle == "" {
		return nil, errors.New("native handle allocation unavailable")
	}
	nickname := alias
	if len(nickname) > 10 {
		nickname = fmt.Sprintf("NM%08X", l.nextID)
		if len(nickname) > 10 {
			return nil, errors.New("native nickname space exhausted")
		}
	}
	account := Account{UserID: l.nextID, Username: username, NativeAlias: alias, NativeNickname: nickname, Handle: handle}
	sealed, err := l.sealNativeLocked(account, password)
	if err != nil {
		return nil, err
	}
	record := localRecord{Account: account, Salt: salt, Hash: hash, NativeSealed: sealed}
	records := append(append([]localRecord(nil), l.records...), record)
	if err := l.persist(records); err != nil {
		return nil, err
	}
	index := len(l.records)
	l.records = records
	l.names[strings.ToLower(username)] = index
	l.names[strings.ToLower(alias)] = index
	l.handles[handle] = true
	l.nextID++
	return &account, nil
}

func (l *Local) Authenticate(username, password string) (*Account, error) {
	if len(password) > 128 {
		return nil, ErrAuthentication
	}
	l.mu.RLock()
	index, ok := l.names[strings.ToLower(username)]
	var rec localRecord
	if ok {
		rec = l.records[index]
	}
	l.mu.RUnlock()
	// Match password work for absent users without leaking a name through a fast rejection.
	salt := rec.Salt
	if !ok {
		salt = make([]byte, 16)
	}
	derived, err := pbkdf2.Key(sha256.New, password, salt, localIterations, 32)
	if err != nil {
		return nil, err
	}
	if !ok || subtle.ConstantTimeCompare(derived, rec.Hash) != 1 {
		return nil, ErrAuthentication
	}
	account := rec.Account
	return &account, nil
}

func (l *Local) persist(records []localRecord) error {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".accounts-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	defer tmp.Close()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	if err := json.NewEncoder(tmp).Encode(localFile{Version: 2, Accounts: records}); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, l.path)
}
