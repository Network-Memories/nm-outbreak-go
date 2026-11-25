package identity

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Options struct{ Mode, AccountFile, URL, KeyFile string }

// Open selects one authority explicitly. NM failure must never create a second,
// independent local owner with the same name.
func Open(options Options) (Authority, error) {
	switch options.Mode {
	case "standalone":
		return OpenLocal(options.AccountFile)
	case "nm":
		u, err := url.Parse(options.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
			return nil, errors.New("valid NM identity URL required in nm mode")
		}
		key, err := os.ReadFile(options.KeyFile)
		if err != nil || len(strings.TrimSpace(string(key))) < 32 {
			return nil, errors.New("readable NM identity key required in nm mode")
		}
		c := NewClient(options.URL, options.KeyFile)
		c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		return c, nil
	default:
		return nil, errors.New("identity mode must be standalone or nm")
	}
}
