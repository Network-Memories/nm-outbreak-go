package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime configuration for the unified NM-OUTBREAK Go service.
type Config struct {
	StunControl       string
	StunBackupControl string
	BindHost          string
	AdvertiseHost     string

	// SNAP UDP ports
	SnapBootstrapPort          int
	SnapAdvertiseBootstrapPort int
	SnapFile1Port              int
	SnapFile2Port              int
	CapcomAppPort              int

	// KDDI / BioServer TCP ports
	KddiFile1LobbyPort int
	KddiFile1GamePort  int
	KddiFile2LobbyPort int
	KddiFile2GamePort  int

	// Unified HTTP state / presence API (site, NM Station, Discord)
	StateHTTPAddr string
	NativeWebAddr string

	// Immutable NM STUN service
	StunAddr          string
	StunCheckInterval time.Duration
	StunTimeout       time.Duration

	// Unified NM Identity service
	IdentityMode    string
	AccountFile     string
	IdentityURL     string
	IdentityKeyFile string
}

func FromEnv() Config {
	return Config{
		StunControl:                envStr("NM_STUN_CONTROLE", ""),
		StunBackupControl:          envStr("NM_STUN_CONTROLE_SECOURS", ""),
		AdvertiseHost:              envStr("NM_OUTBREAK_ADVERTISE_HOST", envStr("NM_OUTBREAK_BIND_HOST", "0.0.0.0")),
		BindHost:                   envStr("NM_OUTBREAK_BIND_HOST", "0.0.0.0"),
		SnapBootstrapPort:          envInt("OPENSNAP_BOOTSTRAP_PORT", 9090),
		SnapAdvertiseBootstrapPort: envInt("OPENSNAP_ADVERTISE_BOOTSTRAP_PORT", envInt("OPENSNAP_BOOTSTRAP_PORT", 9090)),
		SnapFile1Port:              envInt("OPENSNAP_FILE1_PORT", 9091),
		SnapFile2Port:              envInt("OPENSNAP_FILE2_PORT", 9092),
		CapcomAppPort:              envInt("OPENSNAP_CAPCOM_APP_PORT", 10127),
		KddiFile1LobbyPort:         envInt("KDDI_FILE1_LOBBY_PORT", 8300),
		KddiFile1GamePort:          envInt("KDDI_FILE1_GAME_PORT", 8690),
		KddiFile2LobbyPort:         envInt("KDDI_FILE2_LOBBY_PORT", 8200),
		KddiFile2GamePort:          envInt("KDDI_FILE2_GAME_PORT", 8590),
		NativeWebAddr:              envStr("NM_OUTBREAK_WEB_ADDR", "127.0.0.1:18091"),
		StateHTTPAddr:              envStr("NM_OUTBREAK_STATE_ADDR", "127.0.0.1:18090"),
		StunAddr:                   envStr("NM_STUN_ADDR", "127.0.0.1:3478"),
		StunCheckInterval:          time.Duration(envInt("NM_STUN_INTERVAL_SEC", 30)) * time.Second,
		StunTimeout:                time.Duration(envInt("NM_STUN_TIMEOUT_SEC", 3)) * time.Second,
		IdentityMode:               envStr("NM_IDENTITY_MODE", "standalone"),
		AccountFile:                envStr("NM_ACCOUNT_FILE", "state/accounts.json"),
		IdentityURL:                envStr("NM_IDENTITY_URL", ""),
		IdentityKeyFile:            envStr("NM_IDENTITY_KEY_FILE", ""),
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Load rejects invalid environment values rather than silently opening default ports.
func Load() (Config, error) {
	integerKeys := []string{"OPENSNAP_BOOTSTRAP_PORT", "OPENSNAP_ADVERTISE_BOOTSTRAP_PORT", "OPENSNAP_FILE1_PORT", "OPENSNAP_FILE2_PORT", "OPENSNAP_CAPCOM_APP_PORT", "KDDI_FILE1_LOBBY_PORT", "KDDI_FILE1_GAME_PORT", "KDDI_FILE2_LOBBY_PORT", "KDDI_FILE2_GAME_PORT", "NM_STUN_INTERVAL_SEC", "NM_STUN_TIMEOUT_SEC"}
	for _, key := range integerKeys {
		if value := os.Getenv(key); value != "" {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return Config{}, fmt.Errorf("invalid integer configuration: %s", key)
			}
			if strings.HasSuffix(key, "_SEC") && (n < 1 || n > int64(^uint64(0)>>1)/int64(time.Second)) {
				return Config{}, fmt.Errorf("invalid duration configuration: %s", key)
			}
		}
	}
	c := FromEnv()
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}
func (c Config) Validate() error {
	if c.IdentityMode == "nm" && c.StunControl == "" {
		return errors.New("NM mode requires the shared native STUN control")
	}
	if c.StunControl != "" && !filepath.IsAbs(c.StunControl) {
		return errors.New("native STUN control must be an absolute socket path")
	}
	if c.StunBackupControl != "" && (c.StunControl == "" || !filepath.IsAbs(c.StunBackupControl) || filepath.Dir(c.StunBackupControl) != filepath.Dir(c.StunControl) || filepath.Clean(c.StunBackupControl) == filepath.Clean(c.StunControl)) {
		return errors.New("backup STUN control must be a distinct socket in the same directory")
	}

	if c.IdentityMode != "standalone" && c.IdentityMode != "nm" {
		return errors.New("identity mode must be standalone or nm")
	}
	if c.StunCheckInterval <= 0 || c.StunTimeout <= 0 {
		return errors.New("STUN timing must be positive")
	}
	if c.SnapAdvertiseBootstrapPort < 1 || c.SnapAdvertiseBootstrapPort > 65535 {
		return errors.New("advertised SNAP bootstrap port outside 1..65535")
	}
	for _, ports := range [][]int{{c.SnapBootstrapPort, c.SnapFile1Port, c.SnapFile2Port}, {c.KddiFile1LobbyPort, c.KddiFile1GamePort, c.KddiFile2LobbyPort, c.KddiFile2GamePort, c.CapcomAppPort}} {
		seen := make(map[int]bool)
		for _, port := range ports {
			if port < 1 || port > 65535 {
				return errors.New("native port outside 1..65535")
			}
			if seen[port] {
				return errors.New("duplicate native port for one transport")
			}
			seen[port] = true
		}
	}
	usedTCP := map[int]bool{}
	for _, port := range []int{c.KddiFile1LobbyPort, c.KddiFile1GamePort, c.KddiFile2LobbyPort, c.KddiFile2GamePort, c.CapcomAppPort} {
		usedTCP[port] = true
	}
	for _, addr := range []string{c.StateHTTPAddr, c.NativeWebAddr} {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return errors.New("invalid HTTP listener address")
		}
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return errors.New("invalid HTTP listener port")
		}
		if usedTCP[number] {
			return errors.New("duplicate TCP listener port")
		}
		usedTCP[number] = true
	}
	if c.StunAddr == "" {
		return errors.New("STUN service address required")
	}
	return nil
}
