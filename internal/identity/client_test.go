package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Contract of the production endpoint (PHP UnifiedIdentity::dispatch,
// operation outbreak/kddi-connexion), reproduced for the Go client.
func TestClientUsesInternalIdentityProtocol(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/identity/outbreak/kddi-connexion" || r.Header.Get("X-NM-Identity-Key") != "cle-de-labo-0123456789abcdef0123456789" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&got)
		if got["password"] != "secretjap" {
			json.NewEncoder(w).Encode(map[string]string{"status": "refuse"})
			return
		}
		w.Write([]byte(`{"status":"ok","nm_user_id":7,"username":"joueur7","bio_userid":"jap7","handles":[{"handle":"AB12CD","nickname":"いいい"},{"handle":"ZZ99ZZ","nickname":"ううう"}]}`))
	}))
	defer srv.Close()
	key := filepath.Join(t.TempDir(), "key")
	os.WriteFile(key, []byte("cle-de-labo-0123456789abcdef0123456789\n"), 0o600)
	a, err := Open(Options{Mode: "nm", URL: srv.URL + "/internal/identity", KeyFile: key})
	if err != nil {
		t.Fatal(err)
	}
	acc, err := a.Authenticate("jap7", "secretjap")
	if err != nil || acc.UserID != 7 || acc.NativeAlias != "jap7" || acc.Handle != "AB12CD" || acc.NativeNickname != "いいい" {
		t.Fatalf("%+v %v", acc, err)
	}
	if got["game_id"] != "bioserver" || got["username"] != "jap7" {
		t.Fatalf("request %v", got)
	}
	if _, err := a.Authenticate("jap7", "faux"); err == nil {
		t.Fatal("rejected secret accepted")
	}
}

func TestClientSNAPCredentialsFromInternalIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		if r.URL.Path != "/internal/identity/outbreak-snap/go-lookup" || in["game_id"] != "outbreak-snap" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if in["username"] != "west7" {
			w.Write([]byte(`{"status":"inconnu"}`))
			return
		}
		w.Write([]byte(`{"status":"ok","nm_user_id":7,"username":"west7","seed":"abababababababababababababababab","bootstrap_key":"0123456789abcdef0123456789abcdef01234567","handle":"AB12CD","nickname":"いいい"}`))
	}))
	defer srv.Close()
	key := filepath.Join(t.TempDir(), "key")
	os.WriteFile(key, []byte("cle-de-labo-0123456789abcdef0123456789"), 0o600)
	a, _ := Open(Options{Mode: "nm", URL: srv.URL + "/internal/identity", KeyFile: key})
	p, ok := a.(SNAPAuthority)
	if !ok {
		t.Fatal("nm authority must provide SN@P credentials")
	}
	cr, err := p.SNAPCredentials("west7")
	if err != nil || cr.Account.NativeAlias != "west7" || cr.Account.Handle != "AB12CD" || len(cr.MagicKey) != 20 || cr.Seed == "" {
		t.Fatalf("%+v %v", cr, err)
	}
	if _, err := p.SNAPCredentials("nobody"); err != ErrAuthentication {
		t.Fatal(err)
	}
}
