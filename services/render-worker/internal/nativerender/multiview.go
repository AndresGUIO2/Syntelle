package nativerender

import "math"

type MultiView struct {
	Version     string
	SourceSHA   string `json:"source_sha256"`
	GeometrySHA string `json:"geometry_sha256"`
	Protected   bool   `json:"protected_eye_oral"`
	Views       []RegisteredView
}
type RegisteredView struct {
	Pose         [3]float64
	Axis         int
	ReferenceSHA string `json:"reference_sha256"`
	Texture      textureRecord
	UV           textureRecord
}

func viewWeight(view RegisteredView, head [3]float64) float64 {
	angle, target := head[view.Axis], view.Pose[view.Axis]
	if angle*target <= 0 {
		return 0
	}
	return patchSmoothstep(3, math.Abs(target), math.Abs(angle))
}

func (r *Renderer) multiviewColor(color Color, u, v float64, s *FrameState) Color {
	m := r.assets.Motion.Projection.MultiView
	if m == nil {
		return color
	}
	for i, view := range m.Views {
		weight := viewWeight(view, s.Head)
		if weight == 0 {
			continue
		}
		sample := r.assets.viewTextures[i].Sample(u*float64(r.subjectTexture.Width)-.5, v*float64(r.subjectTexture.Height)-.5)
		for k := range 4 {
			color[k] = sample[k]*weight + color[k]*(1-sample[3]*weight)
		}
	}
	return color
}
