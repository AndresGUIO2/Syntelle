package rendercontract

// GestureRepetitions keeps one nod and one shake primitive. double_nod is a read-compatible
// alias for historical recordings, never a separate new director capability.
// Zero signals malformed direction; missing counts retain old recordings.
func GestureRepetitions(kind string, count *int) int {
	n := 1
	if kind == "double_nod" {
		n = 2
	}
	if count != nil {
		n = *count
	}
	if kind == "double_nod" && n != 2 {
		return 0
	}
	if n < 1 || n > 8 || (kind != "nod" && kind != "double_nod" && kind != "shake" && n != 1) {
		return 0
	}
	return n
}

const NodCycleSeconds = .62

const ShakeCycleSeconds = .92

const NaturalNodCycleSeconds = .42
const NaturalShakeCycleSeconds = .58

// NaturalGestureLead aligns the first expressive peak, not gesture onset,
// with an approximate acoustic/text anchor. It never claims word alignment.
func NaturalGestureLead(kind string, duration float64) float64 {
	switch kind {
	case "nod", "double_nod":
		return NaturalNodCycleSeconds * .34
	case "shake":
		return NaturalShakeCycleSeconds * .24
	case "emphasis":
		return duration * .28
	case "brows":
		return duration * .28
	case "tilt":
		return duration * .34
	}
	return .15
}
