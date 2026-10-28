package rendercontract

// MandibleRig is an optional private, source-bound articulation companion.
// It stores estimated kinematics, not a reconstructed anatomical joint.
type MandibleRig struct {
	Version      string             `json:"version"`
	SourceSHA    string             `json:"source_sha256"`
	MotionBuild  string             `json:"motion_build_id"`
	OralBuild    string             `json:"oral_build_id"`
	GeometrySHA  string             `json:"geometry_sha256"`
	Hinge        [3]float64         `json:"hinge"`
	Chin         [2]float64         `json:"chin"`
	MaximumAngle float64            `json:"maximum_angle_radians"`
	OralTravel   float64            `json:"oral_travel_px"`
	CornerTravel float64 `json:"corner_travel_px"`
	ReferenceGap float64            `json:"reference_gap_px"`
	Weights      []float64          `json:"weights"`
	PoseJaw      map[string]float64 `json:"pose_jaw"`
	PoseGap      map[string]float64 `json:"pose_gap"`
	LowerStop    float64            `json:"lower_stop_px"`
	Validation   struct {
		Valid            bool    `json:"valid"`
		MinimumAreaRatio float64 `json:"minimum_area_ratio"`
		ChinTravel720    float64 `json:"chin_travel_720_px"`
	} `json:"validation"`
}
