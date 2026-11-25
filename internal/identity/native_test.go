package identity

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeCredentialsEncryptedAndStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	store, err := OpenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Register("NativeOne", "fixturepwd")
	if err != nil {
		t.Fatal(err)
	}
	credential, err := store.SNAPCredentials("nativeone")
	if err != nil {
		t.Fatal(err)
	}
	expected := sha1.Sum([]byte("fixturepwd" + credential.Seed))
	if credential.Account != *owner || !bytes.Equal(expected[:], credential.MagicKey) {
		t.Fatal("native material belongs to another identity")
	}
	data, _ := os.ReadFile(path)
	if bytes.Contains(data, []byte(credential.Seed)) || bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(credential.MagicKey))) {
		t.Fatal("native challenge secret not wrapped at rest")
	}
	key, err := os.Stat(path + ".native-key")
	if err != nil || key.Mode().Perm() != 0600 {
		t.Fatal("native key not private")
	}
	reopened, err := OpenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := reopened.SNAPCredentials(owner.NativeAlias)
	if err != nil || again.Account != credential.Account || again.Seed != credential.Seed || !bytes.Equal(again.MagicKey, credential.MagicKey) {
		t.Fatal("restart changed native identity", err)
	}
	os.Remove(path + ".native-key")
	if _, err := OpenLocal(path); err == nil {
		t.Fatal("missing wrapping key silently regenerated")
	}
}

func TestLegacyUpgradeProvesPasswordAndPreservesIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	store, err := OpenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Register("LegacyOne", "fixturepwd")
	if err != nil {
		t.Fatal(err)
	}
	// Reconstruct the earlier standalone format without native material.
	store.records[0].NativeSealed = nil
	raw, err := json.Marshal(localFile{Version: 1, Accounts: store.records})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := OpenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.SNAPCredentials(owner.NativeAlias); err == nil {
		t.Fatal("native material guessed from primary hash")
	}
	if _, err := legacy.PrepareSNAP(owner.Username, "wrongpass"); err == nil {
		t.Fatal("unproved legacy upgrade")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, raw) {
		t.Fatal("wrong proof changed stored identity")
	}
	prepared, err := legacy.PrepareSNAP(owner.Username, "fixturepwd")
	if err != nil || *prepared != *owner {
		t.Fatal("legacy upgrade changed owner/alias/HN", err)
	}
	first, err := legacy.SNAPCredentials(owner.NativeAlias)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.PrepareSNAP(owner.Username, "fixturepwd"); err != nil {
		t.Fatal(err)
	}
	second, err := legacy.SNAPCredentials(owner.NativeAlias)
	if err != nil || first.Seed != second.Seed || !bytes.Equal(first.MagicKey, second.MagicKey) {
		t.Fatal("repeated proof regenerated native credentials")
	}
}
