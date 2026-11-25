package identity

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestStandalonePersistsIdentityWithoutNM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	authority, err := Open(Options{Mode: "standalone", AccountFile: path, URL: "http://unreachable.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	store := authority.(*Local)
	first, err := store.Register("PlayerOne", "secretpass")
	if err != nil {
		t.Fatal(err)
	}
	logged, err := store.Authenticate("playerone", "secretpass")
	if err != nil || *logged != *first {
		t.Fatal("same owner was not retained", err)
	}
	if _, err := store.Authenticate("PlayerOne", "wrong"); !errors.Is(err, ErrAuthentication) {
		t.Fatal("wrong password accepted")
	}
	if _, err := store.Register("PLAYERONE", "otherpass"); !errors.Is(err, ErrNameTaken) {
		t.Fatal("case variant assigned to another owner")
	}
	reopened, err := OpenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := reopened.Authenticate(first.NativeAlias, "secretpass")
	if err != nil || *again != *first {
		t.Fatal("restart changed identity", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secretpass") {
		t.Fatal("cleartext password persisted")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("account file permissions are not private")
	}
}

func TestStandaloneNativeNamesAndReservations(t *testing.T) {
	store, _ := OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
	long, err := store.Register("NameLongerThanNativeFifteen", "passwd-one")
	if err != nil {
		t.Fatal(err)
	}
	if long.Username != "NameLongerThanNativeFifteen" || len(long.NativeAlias) > 15 || len(long.NativeAlias) < 5 || len(long.NativeNickname) > 10 || len(long.Handle) != 6 {
		t.Fatal("native identity adaptation invalid")
	}
	if _, err := store.Register(long.NativeAlias, "passwd-two"); !errors.Is(err, ErrNameTaken) {
		t.Fatal("native alias was not reserved")
	}
	other, err := store.Register("PlayerTwo", "passwd-two")
	if err != nil {
		t.Fatal(err)
	}
	if other.Handle == long.Handle || other.UserID == long.UserID {
		t.Fatal("owners share an assigned identity")
	}
	if _, err := store.Register("tiny", "passwd-tri"); err == nil {
		t.Fatal("new short name accepted")
	}
}

func TestStandaloneConcurrentSameName(t *testing.T) {
	store, _ := OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
	var wg sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := store.Register("SamePlayer", "passwd-one"); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrNameTaken) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("created %d owners", successes)
	}
}

func TestStandaloneRejectsCorruptionAndFailedPersistence(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{"version":99,"accounts":[]}`), 0600)
	if _, err := OpenLocal(bad); err == nil {
		t.Fatal("unsupported data silently replaced")
	}
	target := filepath.Join(dir, "target")
	store, _ := OpenLocal(target)
	os.Mkdir(target, 0700)
	if _, err := store.Register("PlayerOne", "passwd-one"); err == nil {
		t.Fatal("write failure ignored")
	}
	if len(store.records) != 0 || store.nextID != 1 {
		t.Fatal("failed write retained an owner in memory")
	}
	// A duplicate persisted HN must fail closed rather than reassign anyone.
	source := filepath.Join(dir, "source.json")
	good, _ := OpenLocal(source)
	good.Register("PlayerOne", "passwd-one")
	data, _ := os.ReadFile(source)
	var file localFile
	json.Unmarshal(data, &file)
	duplicate := file.Accounts[0]
	duplicate.Account.UserID = 2
	duplicate.Account.Username = "PlayerTwo"
	duplicate.Account.NativeAlias = "PlayerTwo"
	file.Accounts = append(file.Accounts, duplicate)
	data, _ = json.Marshal(file)
	os.WriteFile(source, data, 0600)
	if _, err := OpenLocal(source); err == nil {
		t.Fatal("duplicate HN silently adopted")
	}
}

func TestNMModeNeverFallsBackToLocalAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	if _, err := Open(Options{Mode: "nm", AccountFile: path}); err == nil {
		t.Fatal("missing NM configuration accepted")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("NM failure created a local account file")
	}
	if _, err := Open(Options{Mode: "unknown", AccountFile: path}); err == nil {
		t.Fatal("unknown authority mode accepted")
	}
}

func TestCredentialMinimumCountsCharacters(t *testing.T) {
	if ValidateNewCredentials("PlayerOne", "ééé") == nil {
		t.Fatal("three characters passed a five-character minimum")
	}
	if err := ValidateNewCredentials("PlayerOne", "ééééé"); err != nil {
		t.Fatal(err)
	}
}

func TestNewPasswordBoundsCountCharacters(t *testing.T) {
	for _, tc := range []struct {
		password string
		accepted bool
	}{
		{"abcd", false}, {"abcde", true}, {"abcdefghij", true}, {"abcdefghijk", false},
		{"ééééé", true}, {strings.Repeat("é", 10), true}, {strings.Repeat("é", 11), false},
		{string([]byte{0xff, 0xff, 0xff, 0xff, 0xff}), false},
	} {
		if accepted := ValidateNewCredentials("PlayerOne", tc.password) == nil; accepted != tc.accepted {
			t.Fatalf("password with %d bytes: accepted=%v, want %v", len(tc.password), accepted, tc.accepted)
		}
	}
}

func TestHistoricalPasswordOutsideNewBoundsStillAuthenticates(t *testing.T) {
	for _, password := range []string{"old", "historical-secret-longer-than-ten"} {
		store, err := OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
		if err != nil {
			t.Fatal(err)
		}
		owner, err := store.Register("LegacyOwner", "validpass")
		if err != nil {
			t.Fatal(err)
		}
		// Reconstruct a historical persisted credential, without weakening Register.
		rec := &store.records[0]
		rec.Hash, err = pbkdf2.Key(sha256.New, password, rec.Salt, localIterations, 32)
		if err != nil {
			t.Fatal(err)
		}
		rec.NativeSealed, err = store.sealNativeLocked(rec.Account, password)
		if err != nil {
			t.Fatal(err)
		}
		if err = store.persist(store.records); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(store.path)
		loaded, err := OpenLocal(store.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := loaded.Authenticate(owner.Username, password)
		if err != nil || *got != *owner {
			t.Fatal("historical login changed or rejected", err)
		}
		if _, err = loaded.Authenticate(owner.Username, password[:len(password)-1]); !errors.Is(err, ErrAuthentication) {
			t.Fatal("truncated password accepted", err)
		}
		after, _ := os.ReadFile(store.path)
		if string(before) != string(after) {
			t.Fatal("login rewrote historical credentials")
		}
		if _, err = loaded.Register("AnotherOwner", password); err == nil {
			t.Fatal("historical exception leaked into new account creation")
		}
	}
}
