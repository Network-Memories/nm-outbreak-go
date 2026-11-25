package config

import "testing"

func clearConfig(t *testing.T) {
	t.Helper()
	for _, key := range []string{"OPENSNAP_BOOTSTRAP_PORT", "OPENSNAP_ADVERTISE_BOOTSTRAP_PORT", "OPENSNAP_FILE1_PORT", "OPENSNAP_FILE2_PORT", "OPENSNAP_CAPCOM_APP_PORT", "KDDI_FILE1_LOBBY_PORT", "KDDI_FILE1_GAME_PORT", "KDDI_FILE2_LOBBY_PORT", "KDDI_FILE2_GAME_PORT", "NM_STUN_INTERVAL_SEC", "NM_STUN_TIMEOUT_SEC", "NM_IDENTITY_MODE", "NM_OUTBREAK_STATE_ADDR", "NM_STUN_ADDR"} {
		t.Setenv(key, "")
	}
}
func TestConfigurationDefaultsAndInvalidValues(t *testing.T) {
	clearConfig(t)
	c, err := Load()
	if err != nil || c.IdentityMode != "standalone" {
		t.Fatal("standalone defaults invalid", err)
	}
	for _, item := range []struct{ key, value string }{{"OPENSNAP_FILE1_PORT", "bad"}, {"OPENSNAP_FILE1_PORT", "9090"}, {"KDDI_FILE2_GAME_PORT", "8200"}, {"OPENSNAP_FILE1_PORT", "65536"}, {"NM_STUN_INTERVAL_SEC", "0"}, {"NM_STUN_TIMEOUT_SEC", "-1"}, {"NM_STUN_INTERVAL_SEC", "9223372036854775807"}, {"NM_IDENTITY_MODE", "unknown"}, {"NM_OUTBREAK_STATE_ADDR", "127.0.0.1"}} {
		t.Run(item.key+item.value, func(t *testing.T) {
			t.Setenv(item.key, item.value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid value silently accepted")
			}
		})
	}
}

func TestSNAPAdvertisedBootstrapPort(t *testing.T) {
	clearConfig(t)
	t.Setenv("OPENSNAP_BOOTSTRAP_PORT", "30135")
	c, err := Load()
	if err != nil || c.SnapAdvertiseBootstrapPort != c.SnapBootstrapPort {
		t.Fatalf("default advertised port = %d, listening port = %d, error = %v", c.SnapAdvertiseBootstrapPort, c.SnapBootstrapPort, err)
	}
	t.Setenv("OPENSNAP_ADVERTISE_BOOTSTRAP_PORT", "9090")
	c, err = Load()
	if err != nil || c.SnapBootstrapPort != 30135 || c.SnapAdvertiseBootstrapPort != 9090 {
		t.Fatalf("listening port = %d, advertised port = %d, error = %v", c.SnapBootstrapPort, c.SnapAdvertiseBootstrapPort, err)
	}
	t.Setenv("OPENSNAP_ADVERTISE_BOOTSTRAP_PORT", "65536")
	if _, err := Load(); err == nil {
		t.Fatal("out-of-range advertised port accepted")
	}
}
