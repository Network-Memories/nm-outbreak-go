package file1

import "testing"

func TestAftergameProfileSetsMeasuredMarker(t *testing.T) {
	out, err := AftergameProfile(make([]byte, USProfileSize))
	if err != nil || out[212] != 1 || out[221] != 0 {
		t.Fatal("measured File #1 after-game marker is byte 212")
	}
	if _, err := AftergameProfile(make([]byte, JPProfileSize)); err == nil {
		t.Fatal("JP size must be refused")
	}
}
