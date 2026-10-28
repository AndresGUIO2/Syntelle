package rendercontract

// LaughterSpan is private prepared-media metadata, never public speech input.
type LaughterSpan struct {
	Version       string `json:"version"`
	Offset        int    `json:"offset_samples"`
	Count         int    `json:"sample_count"`
	PCMHash       string `json:"pcm_sha256"`
	RecordingHash string `json:"recording_sha256"`
}
