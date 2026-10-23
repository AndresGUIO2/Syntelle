package streaming

import "testing"

func TestTimeLimitEnvironmentIsExplicitAndProductionSafe(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		disabled    bool
		invalid     bool
	}{
		{"production default", "", false, false},
		{"production enabled", "true", false, false},
		{"local exception", "false", true, false},
		{"invalid setting", "development", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SYNTELLE_STREAM_TIME_LIMITS_ENABLED", tc.value)
			disabled, err := TimeLimitsDisabledFromEnv()
			if (err != nil) != tc.invalid || disabled != tc.disabled {
				t.Fatal("unexpected time policy", disabled, err)
			}
		})
	}
}
