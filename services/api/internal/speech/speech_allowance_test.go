package speech

import (
	"strings"
	"testing"
)

func TestSpeechAllowanceReportsActualCause(t *testing.T) {
	for _, item := range []struct {
		name                            string
		count, total, active, requested int
		message                         string
	}{
		{"recording limit", 12, 1921, 0, 14, "All 12 authorized speech requests"},
		{"character limit", 4, 5999, 0, 14, "6000-character speech allowance"},
		{"active recording", 4, 257, 1, 14, "still being prepared"},
		{"available", 4, 257, 0, 14, ""},
	} {
		t.Run(item.name, func(t *testing.T) {
			err := speechAllowanceError(item.count, item.total, item.active, item.requested, 12, 6000)
			if item.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), item.message) {
				t.Fatalf("Unexpected allowance error: %v", err)
			}
		})
	}
}
