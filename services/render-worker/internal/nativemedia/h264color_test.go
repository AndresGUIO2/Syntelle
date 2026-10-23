package nativemedia

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func writeUE(w *h264Writer, value uint32) {
	value++
	n := 0
	for v := value; v > 1; v >>= 1 {
		n++
	}
	w.write(0, n)
	w.write(value, n+1)
}
func fixtureSPS(vui bool) []byte {
	w := &h264Writer{}
	w.write(66, 8)
	w.write(0, 8)
	w.write(31, 8)
	writeUE(w, 0)
	writeUE(w, 0)
	writeUE(w, 2)
	writeUE(w, 1)
	w.write(0, 1)
	writeUE(w, 44)
	writeUE(w, 59)
	w.write(1, 1)
	w.write(1, 1)
	w.write(0, 1)
	if vui {
		w.write(1, 1)
		w.write(1, 1)
		w.write(255, 8)
		w.write(4, 16)
		w.write(3, 16)
		w.write(1, 1)
		w.write(1, 1)
		w.write(1, 1)
		w.write(5, 3)
		w.write(1, 1)
		w.write(1, 1)
		w.write(1, 8)
		w.write(1, 8)
		w.write(1, 8)
		w.write(0, 1)
		w.write(1, 1)
		w.write(1, 32)
		w.write(60, 32)
		w.write(1, 1)
		for range 4 {
			w.write(0, 1)
		}
	} else {
		w.write(0, 1)
	}
	w.write(1, 1)
	nal := []byte{0x67}
	zeros := 0
	for _, b := range w.data {
		if zeros >= 2 && b <= 3 {
			nal = append(nal, 3)
			zeros = 0
		}
		nal = append(nal, b)
		if b == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return nal
}

func inspectSPSColor(t *testing.T, nal []byte) (bool, [3]uint32, string) {
	t.Helper()
	rbsp := []byte{}
	for i := 1; i < len(nal); i++ {
		if i >= 3 && nal[i] == 3 && nal[i-1] == 0 && nal[i-2] == 0 {
			continue
		}
		rbsp = append(rbsp, nal[i])
	}
	r := &h264Bits{data: rbsp}
	_, present, err := spsVUI(r)
	if err != nil || !present {
		t.Fatalf("missing usable SPS VUI: %v", err)
	}
	if r.read(1) != 0 {
		if r.read(8) == 255 {
			r.read(16)
			r.read(16)
		}
	}
	if r.read(1) != 0 {
		r.read(1)
	}
	if r.read(1) == 0 {
		t.Fatal("missing video signal color metadata")
	}
	r.read(3)
	full := r.read(1) != 0
	if r.read(1) == 0 {
		t.Fatal("missing color description")
	}
	colors := [3]uint32{r.read(8), r.read(8), r.read(8)}
	if r.err != nil {
		t.Fatal(r.err)
	}
	var bits strings.Builder
	for _, b := range rbsp {
		fmt.Fprintf(&bits, "%08b", b)
	}
	tail := strings.TrimRight(bits.String()[r.at:], "0")
	return full, colors, tail
}

func TestColorSPSPreservesNativeTimingAndDeclaresMatrix(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		vui  bool
	}{{"missing VUI", false}, {"existing timing VUI", true}} {
		t.Run(tt.name, func(t *testing.T) {
			original := fixtureSPS(tt.vui)
			patched, err := colorSPS(original)
			if err != nil {
				t.Fatal(err)
			}
			full, colors, tail := inspectSPSColor(t, patched)
			if full || colors != [3]uint32{1, 1, 6} {
				t.Fatalf("unexpected output color metadata full=%v colors=%v", full, colors)
			}
			if tt.vui {
				_, _, oldTail := inspectSPSColor(t, original)
				if tail != oldTail {
					t.Fatal("SPS timing/HRD/restriction tail changed")
				}
			}
		})
	}
}

func TestColorH264AccessUnitLeavesEncodedPictureAndPPSUntouched(t *testing.T) {
	t.Parallel()
	other := []byte{0, 0, 1, 0x68, 0xce, 0x06, 0, 0, 0, 1, 0x65, 0x99, 0x88, 0x77}
	source := append([]byte{0, 0, 0, 1}, fixtureSPS(true)...)
	source = append(source, other...)
	patched, err := colorH264AccessUnit(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(patched, other) {
		t.Fatal("color metadata changed PPS or encoded picture bytes")
	}
	_, start := annexStart(patched, 0)
	end, _ := annexStart(patched, start)
	full, colors, _ := inspectSPSColor(t, patched[start:end])
	if full || colors[2] != 6 {
		t.Fatal("access unit did not declare limited601")
	}
	if output, err := colorH264AccessUnit(other); err != nil || !bytes.Equal(output, other) {
		t.Fatal("access unit without SPS changed")
	}
}

func TestColorSPSRejectsUnknownProfile(t *testing.T) {
	t.Parallel()
	source := fixtureSPS(false)
	source[1] = 100
	if _, err := colorSPS(source); err == nil {
		t.Fatal("unknown high-profile syntax was rewritten")
	}
}
