package conversation

import "testing"

func TestRecognitionUtterancePreservesConfirmedSegments(t *testing.T) {
	for _, tt := range []struct {
		name, first, next, want string
	}{
		{"English continuation", "What did you do?", "In your daily routine.", "What did you do? In your daily routine."},
		{"Spanish continuation", "¿Qué hiciste?", " En tu rutina diaria. ", "¿Qué hiciste? En tu rutina diaria."},
		{"intentional repetition", "Yes.", "Yes.", "Yes. Yes."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u := recognitionUtterance{}
			u.append("first-result-01", tt.first)
			if preview := u.preview(tt.next); preview != tt.want {
				t.Fatalf("preview = %q, want %q", preview, tt.want)
			}
			u.append("next-result-02", tt.next)
			u.append("blank-result-03", " ")
			id, text := u.take()
			if id != "first-result-01" || text != tt.want {
				t.Fatalf("turn = %q %q", id, text)
			}
			if id, text = u.take(); id != "" || text != "" {
				t.Fatal("A consumed utterance was submitted again")
			}
		})
	}
}
