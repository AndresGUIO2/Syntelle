package conversation

import (
	"strings"
	"time"
)

// Azure's 700 ms segmentation boundary is not the end of a conversational turn.
// Allow another short pause after a final segment; interim speech suspends it.
const utterancePause = 1100 * time.Millisecond

// Owned exclusively by the socket event loop. Raw SDK results remain archived
// separately; only confirmed text is eligible for a conversation turn.
type recognitionUtterance struct {
	id   string
	text string
}

func (u *recognitionUtterance) append(id, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if u.id == "" {
		u.id = id
	}
	u.text = u.preview(text)
}

func (u recognitionUtterance) preview(partial string) string {
	return strings.TrimSpace(u.text + " " + strings.TrimSpace(partial))
}

func (u *recognitionUtterance) take() (string, string) {
	id, text := u.id, u.text
	*u = recognitionUtterance{}
	return id, text
}
