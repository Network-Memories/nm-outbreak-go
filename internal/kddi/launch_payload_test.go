package kddi

import "testing"

func TestNativeLaunchNumberRequestZeroIsOpaqueNotAClaimedSeat(t *testing.T) {
	if !validLaunchPayload(0x6912, []byte{0}) || !validLaunchPayload(0x6912, nil) {
		t.Fatal("measured native number request rejected")
	}
	for _, p := range [][]byte{{1}, {2}, {255}, {0, 0}} {
		if validLaunchPayload(0x6912, p) {
			t.Fatalf("invalid request accepted: %x", p)
		}
	}
	if validLaunchPayload(0x6911, []byte{0}) || validLaunchPayload(0x6002, []byte{0}) {
		t.Fatal("one-byte variant leaked into other commands")
	}
}
