package accounts

import (
	"strings"
	"testing"
)

func TestVerificationTemplateEscapesCodeForEveryPurpose(t *testing.T) {
	t.Parallel()
	for _, purpose := range []string{"sign_in", "staff_entry", "staff_grant", "ownership_transfer"} {
		t.Run(purpose, func(t *testing.T) {
			t.Parallel()
			message, err := renderVerificationEmail(`<img src=x onerror=alert(1)>`, purpose)
			if err != nil || strings.Contains(message.HTML, "<img") || !strings.Contains(message.HTML, "&lt;img") {
				t.Fatal("dynamic code was not escaped")
			}
		})
	}
}
