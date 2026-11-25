package file2

import "testing"

func TestHostPropertyZones789AndStat(t *testing.T) {
	jp := make([]byte, JPProfileSize)
	jp[200] = 3 // George
	for _, area := range []int{1, 7, 8, 9} {
		out, err := HostPropertyToWestern(jp, area, []byte{2, 0, 1}, 1)
		if err != nil {
			t.Fatalf("area %d failed: %v", area, err)
		}
		if len(out) != WesternProfileSz || out[200] != 3 || out[208] != 1 || out[220] != 1 || out[224] != 4 || out[233] != 2 || out[235] != 1 {
			t.Fatalf("area %d unexpected projection", area)
		}
	}
	stat, err := EncodeStatWord(1, 4, 0)
	if err != nil || stat != 0x020a0402 {
		t.Fatalf("unexpected STAT: 0x%08x err=%v", stat, err)
	}
}
