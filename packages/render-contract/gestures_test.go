package rendercontract

import "testing"

func TestGestureRepetitions(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		count      int
		want       int
	}{
		{"one", "nod", 1, 1}, {"seven", "nod", 7, 7}, {"zero", "nod", 0, 0},
		{"repeated shake", "shake", 7, 7}, {"too many shakes", "shake", 9, 0},
		{"too many", "nod", 9, 0}, {"negative", "nod", -1, 0}, {"other", "laugh", 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GestureRepetitions(tc.kind, &tc.count); got != tc.want {
				t.Fatal(got)
			}
		})
	}
	if GestureRepetitions("double_nod", nil) != 2 || GestureRepetitions("nod", nil) != 1 {
		t.Fatal("legacy interpretation changed")
	}
}
