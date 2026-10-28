package rendercontract

import "testing"

func TestGestureWindowReplenishesAndReservesSequences(t *testing.T) {
	var w GestureWindow
	for i := 0; i < 8; i++ {
		if !w.Admit(float64(i)*2.3, .42, 1) {
			t.Fatal("early exhaustion", i)
		}
	}
	if w.Admit(18.4, .42, 1) {
		t.Fatal("ninth cycle admitted inside rolling window")
	}
	if !w.Admit(22.5, .84, 2) {
		t.Fatal("expired cycles not credited")
	}
	if w.Admit(23, .42, 1) {
		t.Fatal("sequence spacing ignored")
	}
}
