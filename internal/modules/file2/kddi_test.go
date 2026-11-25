package file2

import "testing"

func TestJapaneseFixedRulesFollowCommonAreaConfiguration(t *testing.T) {
	d := Definition().KDDI
	for _, c := range []struct {
		area, index int
		value       byte
	}{{2, 3, 1}, {3, 3, 1}, {4, 3, 1}, {4, 4, 1}, {5, 5, 1}} {
		r := d.Areas[c.area-1].Rules[c.index]
		if r.Default != c.value || r.Editable {
			t.Fatalf("fixed area %d rule %d differs from common setting", c.area, c.index)
		}
	}
	if !d.Areas[4].Rules[3].Editable || !d.Areas[4].Rules[4].Editable {
		t.Fatal("host choices in infinity area were silently fixed")
	}
}
