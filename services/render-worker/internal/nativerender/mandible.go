package nativerender

import "math"

// Existing oral poses already contain their measured gross displacement.
// Only the deviation from that calibration is applied again to the oral layer;
// the visible chin/lower-cheek geometry receives the complete jaw transform.
func (r *Renderer) configureArticulation(s FrameState) {
	r.oral.withoutTeeth = r.referenceActive(s) && r.assets.Expression.ReferenceLaughter.Presentation != nil && !s.LaughSpeech
	rig := r.assets.Motion.Mandible
	r.oral.mandible = rig
	r.oral.jawDelta = 0
	r.oral.lipOpening = 0
	r.oral.lipTension = 0
	if rig == nil {
		return
	}
	contact := (s.From == "Rest" && s.Amount <= 0) || (s.To == "Rest" && s.Amount >= 1) || s.From == "MBP" || s.To == "MBP" || s.From == "FV" || s.To == "FV"
	r.oral.contact = contact
	{
		nominal := mix(rig.PoseJaw[s.From], rig.PoseJaw[s.To], s.Amount)
		r.oral.jawDelta = (clamp(s.JawOpen, 0, 1) - nominal) * rig.OralTravel
		gap := mix(rig.PoseGap[s.From], rig.PoseGap[s.To], s.Amount)
		if !contact {
			r.oral.lipOpening = clamp(s.LipOpening, -.05, .05) * gap
		}
		if !contact {
			r.oral.lipTension = clamp(s.LipTension, -.05, .05)
		}
	}
}
func lowerLipWeight(id int) float64 {
	if id >= 20 {
		id -= 20
	}
	if id <= 10 || id >= 20 {
		return 0
	}
	return math.Sin(math.Pi * float64(20-id) / 10)
}
