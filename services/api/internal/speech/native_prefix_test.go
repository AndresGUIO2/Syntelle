package speech

import (
	"bytes"
	"syntelle/render-contract"
	"testing"
)

func TestNativePrefixPublishesAtSixHundredThenTwoHundredMilliseconds(t *testing.T) {
	input := Input{Text: "one two", Voice: "fixture", Language: "en-US"}
	pcm := bytes.Repeat([]byte{1, 2}, 24000)
	words := []rendercontract.WordBoundary{{Text: "one", TextOffset: 0, Length: 3, OffsetMS: 100, DurationMS: 200}, {Text: "two", TextOffset: 4, Length: 3, OffsetMS: 650, DurationMS: 200}}
	events := []Viseme{{ID: 0, OffsetMS: 0}, {ID: 1, OffsetMS: 600}}
	first, n, ready := nativePCMPrefix(input, Result{}, pcm[:28800], events, words, 0)
	if !ready || n != 28800 || first.DurationMS != 600 || len(first.Words) != 1 {
		t.Fatalf("initial prefix: ready=%v bytes=%d result=%+v", ready, n, first)
	}
	events = append(events, Viseme{ID: 2, OffsetMS: 800})
	if _, _, ready = nativePCMPrefix(input, Result{}, pcm[:38398], events, words, n); ready {
		t.Fatal("published before 200 ms growth")
	}
	second, n2, ready := nativePCMPrefix(input, Result{}, pcm[:38400], events, words, n)
	if !ready || n2-n != 9600 || second.DurationMS != 800 || len(second.Words) != 1 {
		t.Fatal("200 ms increment not retained", n2, ready, second.DurationMS, len(second.Words))
	}
	if !bytes.Equal(first.Audio[44:], second.Audio[44:44+n]) {
		t.Fatal("prefix PCM changed")
	}
	for i, v := range first.Visemes {
		if second.Visemes[i] != v {
			t.Fatal("native event changed")
		}
	}
	if _, _, ready = nativePCMPrefix(input, Result{}, pcm[:28800], events[:1], words, 0); ready {
		t.Fatal("published without native coverage")
	}
}
