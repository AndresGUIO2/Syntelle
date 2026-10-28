package rendercontract

// SpeechJawRig articulates source lower-face skin only. It never controls oral
// geometry, enamel placement, lip contacts, or the reference laughter path.
type SpeechJawRig struct {
	Version             string             `json:"version"`
	SourceSHA           string             `json:"source_sha256"`
	MotionBuild         string             `json:"motion_build_id"`
	OralBuild           string             `json:"oral_build_id"`
	GeometrySHA         string             `json:"geometry_sha256"`
	Hinge               [3]float64         `json:"hinge"`
	Chin                [2]float64         `json:"chin"`
	MaximumAngle        float64            `json:"maximum_angle_radians"`
	MaximumChinFraction float64            `json:"maximum_chin_fraction"`
	LowerNeckY          float64            `json:"lower_neck_y"`
	Weights             []float64          `json:"weights"`
	PoseOpening         map[string]float64 `json:"pose_opening"`
	// Version1 freezes this historical source rectangle. Version2 protects the
	// calibrated OralSupport instead and lets other skin join the jaw.
	ProtectedOralCrop struct{ X, Y, Width, Height int } `json:"protected_oral_crop"`
	OralSupport       *SpeechJawOralSupport             `json:"oral_support,omitempty"`
	Validation        struct {
		Valid            bool    `json:"valid"`
		MinimumAreaRatio float64 `json:"minimum_area_ratio"`
		ChinFraction     float64 `json:"chin_fraction"`
	} `json:"validation"`
}

// SpeechJawOralSupport bounds real calibrated lip/teeth pixels. Surrounding
// skin inside the historical editing crop is allowed to join the jaw smoothly.
type SpeechJawOralSupport struct {
	Version         string       `json:"version"`
	Polygon         [][2]float64 `json:"polygon"`
	SamplingGuard   float64      `json:"sampling_guard_source_px"`
	TransitionWidth float64      `json:"transition_width_source_px"`
}
