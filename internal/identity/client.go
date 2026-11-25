package identity

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var kddiRegistrationName = regexp.MustCompile(`^[A-Za-z0-9]{5,14}$`)

const (
	// Permanent NM rules: username minimum 5; new passwords 5..10 (21/11/2026).
	MinUsernameLen = 5
	MinPasswordLen = 5
	MaxPasswordLen = 10
)

type HandlePair struct {
	Handle   string `json:"handle"`
	Nickname string `json:"nickname"`
}

type Account struct {
	UserID         int64         `json:"user_id"`
	Username       string        `json:"username"`
	Region         string        `json:"region,omitempty"`
	NativeAlias    string        `json:"native_alias,omitempty"`
	NativeNickname string        `json:"native_nickname,omitempty"`
	Handle         string        `json:"handle,omitempty"`
	HandlePairs    [3]HandlePair `json:"-"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, keyFile string) *Client {
	key := ""
	if keyFile != "" {
		if b, err := os.ReadFile(keyFile); err == nil {
			key = strings.TrimSpace(string(b))
		}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  key,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// ValidateNewCredentials enforces the common NM bounds for new credentials
// for account creation, rename, and password change. Existing legacy accounts are not
// blocked on login by this check.
func ValidateNewCredentials(username, password string) error {
	if !utf8.ValidString(username) || utf8.RuneCountInString(username) < MinUsernameLen {
		return errors.New("username must be at least 5 characters")
	}
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < MinPasswordLen || utf8.RuneCountInString(password) > MaxPasswordLen {
		return errors.New("password must contain 5 to 10 characters")
	}
	return nil
}

// RegisterKDDI provisions the native identity through NM. A failure never
// falls back to creating an account in the standalone store.
func (c *Client) RegisterKDDI(username, password string) (*Account, error) {
	if !kddiRegistrationName.MatchString(username) {
		return nil, errors.New("KDDI username must contain 5 to 14 ASCII letters or digits")
	}
	if err := ValidateNewCredentials(username, password); err != nil {
		return nil, err
	}
	if len(password) > 15 {
		return nil, errors.New("KDDI password exceeds the shared Outbreak credential limit")
	}
	var response struct {
		Status   string `json:"status"`
		NMUserID int64  `json:"nm_user_id"`
		Username string `json:"username"`
	}
	if err := c.call("outbreak/kddi-inscription", map[string]string{
		"game_id": "bioserver", "username": username, "password": password,
	}, &response); err != nil {
		return nil, err
	}
	if response.Status != "ok" || response.NMUserID < 1 || response.Username == "" {
		return nil, errors.New("NM did not confirm KDDI registration")
	}
	return &Account{UserID: response.NMUserID, Username: response.Username}, nil
}

// Authenticate proves an original Japanese (KDDI) client with its historical
// JAP secret through the NM identity service (operation outbreak/kddi-connexion,
// dedicated key). The secret stays in the NM-held bioserver record; it is never
// copied, truncated or accepted as the NM master password.
func (c *Client) Authenticate(username, password string) (*Account, error) {
	if c.baseURL == "" {
		return nil, errors.New("identity service URL not configured")
	}
	var r struct {
		Status    string       `json:"status"`
		NMUserID  int64        `json:"nm_user_id"`
		Username  string       `json:"username"`
		BioUserID string       `json:"bio_userid"`
		Handles   []HandlePair `json:"handles"`
	}
	if err := c.call("outbreak/kddi-connexion", map[string]string{"game_id": "bioserver", "username": username, "password": password}, &r); err != nil {
		return nil, err
	}
	if r.Status != "ok" {
		return nil, errors.New("authentication rejected")
	}
	if r.NMUserID < 1 || r.Username == "" || r.BioUserID == "" || len(r.Handles) == 0 || len(r.Handles[0].Handle) != 6 || r.Handles[0].Nickname == "" {
		return nil, errors.New("identity service returned incomplete native identity")
	}
	h := r.Handles[0]
	account := &Account{UserID: r.NMUserID, Username: r.Username, NativeAlias: r.BioUserID, NativeNickname: h.Nickname, Handle: h.Handle}
	copy(account.HandlePairs[:], r.Handles)
	return account, nil
}

// call posts one operation to the internal NM identity endpoint
// (POST <base>/<operation>, X-NM-Identity-Key), as served in production.
func (c *Client) call(operation string, input any, out any) error {
	body, err := json.Marshal(input)
	if err != nil || len(body) > 4096 {
		return errors.New("identity request too large")
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/"+operation, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NM-Identity-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("identity service rejected the request")
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(out)
}

// SNAPCredentials returns the sealed SN@P challenge material of an original
// Western client and its shared Capcom ID (operation outbreak-snap/go-lookup).
// The material is derived by NM only after proof of the owner's password.
func (c *Client) SNAPCredentials(name string) (SNAPCredential, error) {
	var r struct {
		Status       string `json:"status"`
		NMUserID     int64  `json:"nm_user_id"`
		Username     string `json:"username"`
		Seed         string `json:"seed"`
		BootstrapKey string `json:"bootstrap_key"`
		Handle       string `json:"handle"`
		Nickname     string `json:"nickname"`
	}
	if err := c.call("outbreak-snap/go-lookup", map[string]string{"game_id": "outbreak-snap", "username": name}, &r); err != nil {
		return SNAPCredential{}, err
	}
	if r.Status == "inconnu" {
		return SNAPCredential{}, ErrAuthentication
	}
	if r.Status != "ok" {
		return SNAPCredential{}, ErrNativeMaterial
	}
	magic, err := hex.DecodeString(r.BootstrapKey)
	if err != nil || len(magic) != 20 || len(r.Seed) != 32 || r.NMUserID < 1 || r.Username == "" || len(r.Handle) != 6 || r.Nickname == "" {
		return SNAPCredential{}, ErrNativeMaterial
	}
	acc := Account{UserID: r.NMUserID, Username: r.Username, NativeAlias: r.Username, NativeNickname: r.Nickname, Handle: r.Handle}
	return SNAPCredential{Account: acc, Seed: r.Seed, MagicKey: magic}, nil
}
