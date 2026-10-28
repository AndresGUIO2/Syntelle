package nativeadapter

import (
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
)

// A private cosmetic channel follows the existing viseme timeline. Removing a
// brief contact from this skin-only curve does not change any lip/contact pose.
// The centered40ms preparation window adds no audio or mouth playback delay.
func speechJawAt(speech nativeanimation.SpeechState, rig *rendercontract.SpeechJawRig, preview, reduced bool, laugh float64) float64 {
	if rig == nil || preview || reduced || laugh > 0 || !speech.Active || speech.Speech == nil || speech.Position >= speech.Speech.Duration {
		return 0
	}
	if plan := speech.Speech.Plan; plan != nil && plan.Performance != nil {
		for _, cue := range plan.Performance.Resolved {
			if cue.Kind == "laugh" && speech.Position >= cue.Time-.02 && speech.Position <= cue.Time+cue.Duration+.32 {
				return 0
			}
		}
	}
	timeline := speech.Speech.Timeline
	jawTimeline := make([]nativeanimation.TimelineEvent, 0, len(timeline))
	for i, event := range timeline {
		end := speech.Speech.Duration
		if i+1 < len(timeline) {
			end = timeline[i+1].Time
		}
		if (event.Pose == "MBP" || event.Pose == "FV") && end-event.Time < .12 {
			continue
		}
		jawTimeline = append(jawTimeline, event)
	}
	value := func(seconds float64) float64 {
		mouth := nativeanimation.MouthAt(jawTimeline, seconds)
		return rig.PoseOpening[mouth.From]*(1-mouth.Amount) + rig.PoseOpening[mouth.To]*mouth.Amount
	}
	return .25*value(speech.Position-.02) + .5*value(speech.Position) + .25*value(speech.Position+.02)
}
