package nativeanimation

import "math"

// One video-owner lifecycle survives PCM completion. It does not own emotion policy.
type laughterPresentation struct {
	session                     uint64
	speech                      *Speech
	origin, from                float64
	cancel                      bool
	cancelAt, faceFrom, jawFrom float64
}

func (e *Engine) LaughterPresenting() bool { return e.config.LaughterV2 && e.laughter.speech != nil }
func (e *Engine) presentLaughter(ms float64, s SpeechState, settings Settings, out *Frame) {
	d := &e.laughter
	if settings.ReducedMotion {
		*d = laughterPresentation{}
		out.Laugh = 0
		return
	}
	admitted := false
	if s.Active && s.Speech != nil && s.Speech.laughter != nil && s.plan() != nil && s.plan().Performance != nil && (settings.AutomaticExpressions || settings.ManualLaughter) {
		for _, cue := range s.plan().Performance.Resolved {
			if cue.Kind == "laugh" && cue.Amount != nil && *cue.Amount > 0 && s.Position >= cue.Time {
				admitted = true
				break
			}
		}
	}
	if admitted && (d.speech == nil || d.session != s.Session) {
		*d = laughterPresentation{session: s.Session, speech: s.Speech, origin: ms - s.Position*1000, from: e.last.Laugh, jawFrom: func() float64 {
			if e.last.Laugh > 0 {
				return e.last.JawOpen
			}
			return 0
		}()}
	}
	if d.speech == nil {
		out.Laugh = 0
		return
	}
	newSpeech := s.Active && s.Session != d.session
	interrupted := s.Session != d.session || s.Speech == nil || (!s.Active && s.Position+1e-6 < d.speech.Duration)
	if interrupted && !d.cancel {
		d.cancel = true
		d.cancelAt = ms
		d.faceFrom = e.last.Laugh
		d.jawFrom = e.last.JawOpen
	}
	if d.cancel {
		weight := 1 - ease((ms-d.cancelAt)/240)
		out.Laugh = d.faceFrom * weight
		if !newSpeech {
			out.JawOpen = d.jawFrom * weight
		}
		if weight == 0 {
			*d = laughterPresentation{}
		}
	} else {
		position := (ms - d.origin) / 1000
		if s.Active && s.Session == d.session {
			position = s.Position
		}
		channels, _ := d.speech.LaughterAt(position)
		span := d.speech.laughter
		relative := position - float64(span.span.Offset)/float64(span.rate)
		face := blend(d.from, 1, ease(relative/.12)) * (1 - ease((position-d.speech.LaughterLastBurst())/.32))
		out.Laugh = face
		out.JawOpen = math.Max(channels.JawOpen, d.jawFrom*(1-ease(relative/.08)))
		out.LipOpening = 0
		out.LipTension = 0
		if relative > float64(span.span.Count)/float64(span.rate)+.32 {
			out.Laugh = 0
			out.JawOpen = 0
			*d = laughterPresentation{}
		}
	}
	if out.Laugh > 0 && !newSpeech {
		opening := .4*out.Emotion[8] + .14*out.Emotion[7]
		if e.config.EmotionRestOpening != nil {
			opening = 0
			for i, weight := range out.Emotion {
				opening += weight * e.config.EmotionRestOpening[i]
			}
		}
		out.Mouth = Mouth{"Rest", "A", opening}
	}
	out.LaughSpeech = newSpeech
}
