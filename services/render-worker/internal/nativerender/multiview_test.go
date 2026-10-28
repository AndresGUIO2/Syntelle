package nativerender

import "testing"

func TestRegisteredViewHasNoFrontalOrOpposingContribution(t *testing.T) {
	v := RegisteredView{Pose: [3]float64{13, 0, 0}, Axis: 0}
	for _, yaw := range []float64{-8, 0, 3} {
		if viewWeight(v, [3]float64{yaw, 0, 0}) != 0 {
			t.Fatal("unsupported view contributed", yaw)
		}
	}
	last := 0.
	for yaw := 3.; yaw <= 13; yaw += .1 {
		w := viewWeight(v, [3]float64{yaw, 0, 0})
		if w < last || w > 1 {
			t.Fatal("view support popped", yaw, w)
		}
		last = w
	}
}
