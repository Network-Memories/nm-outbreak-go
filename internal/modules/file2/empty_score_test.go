package file2

import "testing"

func TestEmptyPlayerScoreContainsNoInventedScoreWords(t *testing.T) {
	for n := byte(1); n <= 4; n++ {
		p := EmptyPlayerScore(n)
		if len(p) != 3 || p[0] != n || p[1] != 0 || p[2] != 0 {
			t.Fatal(p)
		}
	}
	for _, n := range []byte{0, 5, 255} {
		if EmptyPlayerScore(n) != nil {
			t.Fatal("invalid seat")
		}
	}
}
