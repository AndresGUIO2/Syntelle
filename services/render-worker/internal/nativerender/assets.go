package nativerender

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syntelle/render-contract"
)

type Rect struct{ X, Y, Width, Height int }
type Size struct{ Width, Height int }
type vector3 [3]float64
type Camera struct {
	Center                     [2]float64
	Focal, Distance, Near, Far float64
	PivotPx                    [2]float64 `json:"pivot_px"`
}
type Projection struct {
	MultiView *MultiView `json:"multiview"`
	Version   string
	SourceSHA string `json:"source_sha256"`
	Pivot     vector3
	Camera    Camera
	Bounds    struct{ Yaw, Pitch, Roll float64 }
	Geometry  struct {
		File, SHA256        string
		Vertices, Triangles int
		FringeTriangles     int `json:"fringe_triangles"`
	}
	Validation   struct{ Valid bool }
	Articulation json.RawMessage
}
type Motion struct {
	SpeechJaw   *rendercontract.SpeechJawRig `json:"speech_jaw"`
	Mandible    *rendercontract.MandibleRig  `json:"mandible"`
	BuildID     string                       `json:"build_id"`
	OriginalSHA string                       `json:"original_sha256"`
	OralBuildID string                       `json:"oral_build_id"`
	Canvas      Size
	Foreground  string
	Projection  Projection
	Anatomy     struct {
		ChinY      float64 `json:"chin_y"`
		FaceHeight float64 `json:"face_height"`
	}
	Eyes struct {
		Crop               Rect
		Open, Closed, Mask string
		Curves             [][]float64
	}
}
type EmotionRig struct {
	EyelidTension []float64 `json:"eyelid_tension"`
	EditCoverage  string    `json:"edit_coverage"`
	Oral          *struct {
		Version    string
		States     map[string]map[string]JointPair
		Triangles  map[string][][3]int
		Validation struct{ Valid bool }
	} `json:"oral"`
	InverseIterations int `json:"inverse_iterations"`
	Version           string
	SourceSHA         string `json:"source_sha256"`
	MotionBuild       string `json:"motion_build_id"`
	OralBuild         string `json:"oral_build_id"`
	Crop              Rect
	Units             float64 `json:"units_per_byte"`
	Field             struct{ File, SHA256 string }
	FieldSize         [2]int `json:"field_size"`
	States            []string
	Validation        struct{ Valid bool }
}

type Expression struct {
	EmotionRig  *EmotionRig `json:"emotion_rig"`
	Version     string
	SourceSHA   string `json:"source_sha256"`
	MotionBuild string `json:"motion_build_id"`
	Crop        Rect
	Units       float64 `json:"units_per_byte"`
	Field       struct{ File, SHA256 string }
	Smile       *struct {
		Material   struct{ File, SHA256 string }
		ClosedOnly bool `json:"closed_mouth_only"`
	}
	Validation        struct{ Valid bool }
	Joint             *JointExpression   `json:"joint_articulation"`
	ReferenceLaughter *ReferenceLaughter `json:"reference_laughter"`
}

type JointPair struct {
	RestDelta [][2]float64 `json:"rest_delta"`
	OpenDelta [][2]float64 `json:"open_delta"`
	Triangles [][3]int     `json:"triangles"`
}
type JointExpression struct {
	SkinAppearance struct {
		FieldSHA string `json:"field_sha256"`
	} `json:"skin_appearance"`
	LipMaterial          *textureRecord `json:"lip_material"`
	Mode                 string         `json:"mode"`
	MaximumArticulation  float64        `json:"maximum_articulation"`
	EyeClosure           float64        `json:"eye_closure"`
	RestLowerTranslation [2]float64     `json:"rest_lower_translation"`
	RestOpening          float64        `json:"rest_opening"`
	Version              string
	Pairs                map[string]JointPair
	DentalTranslation    [2]float64 `json:"dental_translation"`
	Validation           struct{ Valid bool }
	ReviewStatus         string           `json:"review_status"`
	SkinFrames           []textureRecord  `json:"skin_frames"`
	Laugh                *JointExpression `json:"laugh"`
}
type textureRecord struct {
	File, SHA256  string
	Width, Height int
}
type manifest struct {
	MotionScale float64 `json:"motion_scale"`
	Envelope    *struct {
		SourceBuild string `json:"source_build_id"`
		GeometrySHA string `json:"geometry_sha256"`
		Scale       float64
		Status      string
		Validation  struct{ Valid bool }
	} `json:"motion_envelope"`
	Version       string
	PackageHash   string          `json:"package_hash"`
	IdentityFiles json.RawMessage `json:"identity_files"`
	Packages      map[string]json.RawMessage
	Textures      map[string]textureRecord
	Geometry      struct{ File, SHA256 string }
}
type sourceVertex struct{ X, Y, Z, Head, Torso float64 }
type fringeTriangle struct {
	IDs      [3]uint16
	Weighted vector3
	Z, Depth float64
}

func mathFloat32(b []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }

// Assets is loaded once. Sessions must never mutate its package or texture data.
type Assets struct {
	viewTextures []*Texture
	// Only small content identities are cached here, never concatenated atlases.
	// Prepared texture bytes and their ordering remain immutable after Load.
	gpuAtlasKeys                                               sync.Map
	MotionScale                                                float64
	PackageHash                                                string
	Motion                                                     Motion
	MotionJSON, OralJSON, GazeJSON, ExpressionJSON             json.RawMessage
	Foreground, ReferenceField, ExpressionField, SmileMaterial *Texture
	EmotionField                                               *Texture
	Expression                                                 Expression
	SkinFrames                                                 []*Texture
	LaughFrames                                                []*Texture
	LaughLips                                                  *Texture
	ReferenceFrames                                            []*Texture
	ReferenceCavity                                            *Texture
	ReferenceUpper, ReferenceLower                             *Texture
	Oral                                                       *OralAssets
	Opened, Closed, EyeMask, Sclera, Ownership                 *Texture
	EyeSplit                                                   float64
	Backgrounds                                                map[string]*Texture
	vertices                                                   []sourceVertex
	indices                                                    []uint16
	fringe                                                     []fringeTriangle
}

func readChecked(root, path, hash string) ([]byte, error) {
	if filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return nil, errors.New("invalid native resource path")
	}
	name := filepath.Join(root, filepath.FromSlash(path))
	rel, err := filepath.Rel(root, name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("native resource escapes package")
	}
	real, err := filepath.EvalSymlinks(name)
	if err != nil {
		return nil, errors.New("native resource unavailable")
	}
	rel, err = filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("native resource symlink escapes package")
	}
	b, err := os.ReadFile(real)
	if err != nil {
		return nil, errors.New("native resource unreadable")
	}
	sum := sha256.Sum256(b)
	if len(hash) != 64 || hex.EncodeToString(sum[:]) != hash {
		return nil, errors.New("native resource checksum mismatch")
	}
	return b, nil
}

func Load(root string) (*Assets, error) { return loadAssets(root, true) }

// LoadCharacter omits backgrounds from the live worker; they are public browser resources.
func LoadCharacter(root string) (*Assets, error) { return loadAssets(root, false) }

func loadAssets(root string, backgrounds bool) (*Assets, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m manifest
	if err = json.Unmarshal(b, &m); err != nil || m.Version != "native-character/1" {
		return nil, errors.New("invalid native character manifest")
	}
	// Hash the exact compact file-list representation used by configure-local.
	// A worker-specific export hash would reject the unchanged API envelope.
	var identity bytes.Buffer
	if err = json.Compact(&identity, m.IdentityFiles); err != nil {
		return nil, errors.New("invalid native character identity")
	}
	identityHash := sha256.Sum256(identity.Bytes())
	if len(m.PackageHash) != 64 || hex.EncodeToString(identityHash[:]) != m.PackageHash {
		return nil, errors.New("native character identity checksum mismatch")
	}
	a := &Assets{MotionJSON: m.Packages["motion"], OralJSON: m.Packages["oral"], GazeJSON: m.Packages["gaze"], ExpressionJSON: m.Packages["expressions"], Backgrounds: map[string]*Texture{}}
	a.PackageHash = m.PackageHash
	if err = json.Unmarshal(a.MotionJSON, &a.Motion); err != nil {
		return nil, err
	}
	a.MotionScale = 1
	if m.MotionScale != 0 && m.MotionScale != 1 {
		e := m.Envelope
		if (m.MotionScale != 1.25 && m.MotionScale != 1.5) || e == nil || e.Scale != m.MotionScale || !e.Validation.Valid || e.Status != "internal-visual-pass" || e.SourceBuild != a.Motion.BuildID || e.GeometrySHA != a.Motion.Projection.Geometry.SHA256 {
			return nil, errors.New("unvalidated motion envelope")
		}
		a.MotionScale = m.MotionScale
	}
	if !a.Motion.Projection.Validation.Valid || a.Motion.Projection.SourceSHA != a.Motion.OriginalSHA || a.Motion.Projection.Version != "rgbd-perspective/1" || len(a.Motion.Projection.Articulation) > 0 {
		return nil, errors.New("unsupported or invalid native projection")
	}
	validRect := func(r Rect) bool {
		return r.X >= 0 && r.Y >= 0 && r.Width > 0 && r.Height > 0 && r.X+r.Width <= a.Motion.Canvas.Width && r.Y+r.Height <= a.Motion.Canvas.Height
	}
	var oralBinding struct {
		Source string `json:"source_sha256"`
		Build  string `json:"build_id"`
		Canvas Size
		Crop   Rect
	}
	if err = json.Unmarshal(a.OralJSON, &oralBinding); err != nil {
		return nil, err
	}
	if oralBinding.Source != a.Motion.OriginalSHA || oralBinding.Build != a.Motion.OralBuildID || oralBinding.Canvas != a.Motion.Canvas || !validRect(oralBinding.Crop) || !validRect(a.Motion.Eyes.Crop) {
		return nil, errors.New("native mouth/eye package does not match projection")
	}
	camera := a.Motion.Projection.Camera
	if camera.Focal <= 0 || camera.Near <= 0 || camera.Distance <= camera.Near || camera.Far <= camera.Distance {
		return nil, errors.New("invalid native projection camera")
	}
	loaded := map[string]*Texture{}
	load := func(name string) (*Texture, error) {
		if t := loaded[name]; t != nil {
			return t, nil
		}
		r, ok := m.Textures[name]
		if !ok || r.Width < 1 || r.Height < 1 || r.Width > 8192 || r.Height > 8192 {
			return nil, fmt.Errorf("missing native texture: %s", name)
		}
		bytes, e := readChecked(root, r.File, r.SHA256)
		if e != nil {
			return nil, e
		}
		if len(bytes) != r.Width*r.Height*4 {
			return nil, errors.New("invalid native texture size")
		}
		t := &Texture{Width: r.Width, Height: r.Height, Pixels: bytes}
		loaded[name] = t
		return t, nil
	}
	for name, target := range map[string]**Texture{"motion/" + a.Motion.Foreground: &a.Foreground, "motion/" + a.Motion.Eyes.Open: &a.Opened, "motion/" + a.Motion.Eyes.Closed: &a.Closed, "motion/" + a.Motion.Eyes.Mask: &a.EyeMask, "gaze/sclera.png": &a.Sclera, "gaze/ownership.png": &a.Ownership} {
		*target, err = load(name)
		if err != nil {
			return nil, err
		}
	}
	if a.Foreground.Width != a.Motion.Canvas.Width || a.Foreground.Height != a.Motion.Canvas.Height {
		return nil, errors.New("native foreground dimensions do not match projection")
	}
	a.Foreground = a.Foreground.Premultiply()
	a.Oral, err = NewOralAssets(a.OralJSON, func(name string) (*Texture, error) { return load("oral/" + name) })
	if err != nil {
		return nil, err
	}
	if len(a.ExpressionJSON) > 0 {
		if err = json.Unmarshal(a.ExpressionJSON, &a.Expression); err != nil {
			return nil, err
		}
		if !a.Expression.Validation.Valid || a.Expression.SourceSHA != a.Motion.OriginalSHA || a.Expression.MotionBuild != a.Motion.BuildID || !validRect(a.Expression.Crop) || a.Expression.Units <= 0 || math.IsNaN(a.Expression.Units) || math.IsInf(a.Expression.Units, 0) {
			return nil, errors.New("native expression source mismatch")
		}
		a.ExpressionField, err = load("expressions/" + a.Expression.Field.File)
		if err != nil {
			return nil, err
		}
		if rig := a.Expression.EmotionRig; rig != nil {
			if rig.EyelidTension != nil {
				if len(rig.EyelidTension) != 10 || rig.EyelidTension[0] != 0 {
					return nil, errors.New("invalid emotional eyelid calibration")
				}
				for _, value := range rig.EyelidTension {
					if !patchFinite(value) || value < 0 || value > .4 {
						return nil, errors.New("emotional eyelid calibration exceeds prepared range")
					}
				}
			}
			if rig.EditCoverage != "" && rig.EditCoverage != "deformed-oral-coverage/1" {
				return nil, errors.New("unsupported emotional edit coverage")
			}
			if rig.InverseIterations < 5 || rig.InverseIterations > 16 || rig.Version != "emotion-rig/1" || !rig.Validation.Valid || rig.SourceSHA != a.Motion.OriginalSHA || rig.MotionBuild != a.Motion.BuildID || rig.OralBuild != a.Motion.OralBuildID || !validRect(rig.Crop) || !patchFinite(rig.Units) || rig.Units <= 0 || len(rig.States) != 10 {
				return nil, errors.New("invalid source-bound emotion rig")
			}
			if rig.Oral != nil {
				if rig.Oral.Version != "joint-emotion-articulation/1" || !rig.Oral.Validation.Valid {
					return nil, errors.New("invalid emotional articulation")
				}
				for name, triangles := range rig.Oral.Triangles {
					pair, ok := a.Oral.pairs[name]
					if !ok || !pair.Safe || pair.Mesh == nil || len(triangles) == 0 {
						return nil, errors.New("emotion topology binding mismatch")
					}
					for _, triangle := range triangles {
						for _, index := range triangle {
							if index < 0 || index >= len(pair.Mesh.Rest) {
								return nil, errors.New("emotion triangle index exceeds geometry")
							}
						}
					}
				}
				for state, pairs := range rig.Oral.States {
					if rendercontract.EmotionIndex(state) <= 0 {
						return nil, errors.New("unknown emotional lip action")
					}
					for name, delta := range pairs {
						pair, ok := a.Oral.pairs[name]
						if !ok || !pair.Safe || pair.Mesh == nil || len(rig.Oral.Triangles[name]) == 0 || len(delta.RestDelta) != len(pair.Mesh.Rest) || len(delta.OpenDelta) != len(pair.Mesh.Open) {
							return nil, errors.New("emotion lip geometry binding mismatch")
						}
						for _, points := range [][][2]float64{delta.RestDelta, delta.OpenDelta} {
							for _, point := range points {
								if !patchFinite(point[0]) || !patchFinite(point[1]) {
									return nil, errors.New("non-finite emotional articulation")
								}
							}
						}
					}
				}
			}
			for i, state := range rig.States {
				if state != rendercontract.EmotionStates[i] {
					return nil, errors.New("invalid emotion field ordering")
				}
			}
			a.EmotionField, err = load("expressions/" + rig.Field.File)
			if err != nil {
				return nil, err
			}
			if a.EmotionField.Width != rig.FieldSize[0] || a.EmotionField.Height != rig.FieldSize[1]*10 {
				return nil, errors.New("invalid emotion atlas dimensions")
			}
		}
		if a.Expression.Smile != nil {
			a.SmileMaterial, err = load("expressions/" + a.Expression.Smile.Material.File)
			if err != nil {
				return nil, err
			}
			joint := a.Expression.Joint
			jointValid := joint != nil && joint.Version == "joint-expression-articulation/1" && joint.Validation.Valid
			if jointValid {
				for name, prepared := range joint.Pairs {
					pair, ok := a.Oral.pairs[name]
					if !ok || pair.Mesh == nil || len(prepared.RestDelta) != len(pair.Mesh.Rest) || len(prepared.OpenDelta) != len(pair.Mesh.Open) || len(prepared.Triangles) == 0 {
						return nil, errors.New("joint oral geometry binding differs")
					}
					for _, points := range [][][2]float64{prepared.RestDelta, prepared.OpenDelta} {
						for _, p := range points {
							if !patchFinite(p[0]) || !patchFinite(p[1]) {
								return nil, errors.New("non-finite joint geometry")
							}
						}
					}
					for _, triangle := range prepared.Triangles {
						for _, id := range triangle {
							if id < 0 || id >= len(pair.Mesh.Rest) {
								return nil, errors.New("joint triangle index exceeds geometry")
							}
						}
					}
				}
				for _, frame := range joint.SkinFrames {
					texture, loadErr := load("expressions/" + frame.File)
					if loadErr != nil {
						return nil, loadErr
					}
					if texture.Width != a.Expression.Crop.Width || texture.Height != a.Expression.Crop.Height {
						return nil, errors.New("joint skin dimensions differ")
					}
					a.SkinFrames = append(a.SkinFrames, texture.Premultiply())
				}
				if len(a.SkinFrames) != 25 {
					return nil, errors.New("joint expression skin frames missing")
				}
				if joint.Laugh != nil {
					if lip := joint.Laugh.LipMaterial; lip != nil {
						a.LaughLips, err = load("expressions/" + lip.File)
						if err != nil {
							return nil, err
						}
						if a.LaughLips.Width != a.Oral.Width || a.LaughLips.Height != a.Oral.Height {
							return nil, errors.New("laughter lip material dimensions differ")
						}
					}
					for _, frame := range joint.Laugh.SkinFrames {
						texture, loadErr := load("expressions/" + frame.File)
						if loadErr != nil {
							return nil, loadErr
						}
						a.LaughFrames = append(a.LaughFrames, texture.Premultiply())
					}
					if len(a.LaughFrames) != 25 {
						return nil, errors.New("joint laughter skin frames missing")
					}
				}
			}
			if (!a.Expression.Smile.ClosedOnly && !jointValid) || a.SmileMaterial.Width != a.Expression.Crop.Width || a.SmileMaterial.Height != a.Expression.Crop.Height {
				return nil, errors.New("native smile dimensions or articulation policy differ")
			}
			a.SmileMaterial = a.SmileMaterial.Premultiply()
		}
	}
	var gaze struct {
		Version     string
		Source      string `json:"source_sha256"`
		MotionBuild string `json:"motion_build_id"`
		Crop        Rect
		Validation  struct{ Valid bool }
		Eyes        []struct{ Center [2]float64 }
	}
	if err = json.Unmarshal(a.GazeJSON, &gaze); err != nil || len(gaze.Eyes) != 2 || gaze.Version != "observed-iris/1" || !gaze.Validation.Valid || gaze.Source != a.Motion.OriginalSHA || gaze.MotionBuild != a.Motion.BuildID || gaze.Crop != a.Motion.Eyes.Crop {
		return nil, errors.New("invalid native gaze package")
	}
	a.EyeSplit = (gaze.Eyes[0].Center[0] + gaze.Eyes[1].Center[0]) / 2
	if err = a.loadReference(load); err != nil {
		return nil, err
	}
	if v := a.Motion.Projection.MultiView; v != nil {
		if v.Version != "registered-multiview/1" || v.SourceSHA != a.Motion.OriginalSHA || v.GeometrySHA != a.Motion.Projection.Geometry.SHA256 || !v.Protected || len(v.Views) == 0 || len(v.Views) > 4 {
			return nil, errors.New("invalid multiview source binding")
		}
		for _, view := range v.Views {
			if view.Axis < 0 || view.Axis > 2 || !patchFinite(view.Pose[view.Axis]) || math.Abs(view.Pose[view.Axis]) < 5 || math.Abs(view.Pose[view.Axis]) > 25 || len(view.ReferenceSHA) != 64 || len(view.UV.SHA256) != 64 {
				return nil, errors.New("invalid registered view")
			}
			t, e := load("multiview/" + view.Texture.File)
			if e != nil {
				return nil, e
			}
			if t.Width != a.Motion.Canvas.Width || t.Height != a.Motion.Canvas.Height {
				return nil, errors.New("registered view dimensions differ")
			}
			protected := []Rect{oralBinding.Crop, a.Motion.Eyes.Crop, a.Expression.Crop}
			if a.Expression.ReferenceLaughter != nil {
				protected = append(protected, a.Expression.ReferenceLaughter.Crop)
			}
			for _, crop := range protected {
				for y := crop.Y; y < crop.Y+crop.Height; y++ {
					for x := crop.X; x < crop.X+crop.Width; x++ {
						if t.Pixels[(y*t.Width+x)*4+3] != 0 {
							return nil, errors.New("registered view overlaps protected eye or oral material")
						}
					}
				}
			}
			a.viewTextures = append(a.viewTextures, t.Premultiply())
		}
	}
	for name := range m.Textures {
		if backgrounds && strings.HasPrefix(name, "background/") {
			id := strings.TrimPrefix(name, "background/")
			a.Backgrounds[id], err = load(name)
			if err != nil {
				return nil, err
			}
		}
	}
	g, err := readChecked(root, m.Geometry.File, m.Geometry.SHA256)
	if err != nil {
		return nil, err
	}
	p := a.Motion.Projection.Geometry
	if p.SHA256 != m.Geometry.SHA256 || p.Vertices < 4 || p.Vertices > 65535 || len(g) != p.Vertices*20+(p.Triangles+p.FringeTriangles)*6 {
		return nil, errors.New("invalid native geometry length")
	}
	if rig := a.Motion.Mandible; rig != nil {
		if (rig.Version != "mandibular-rig/1" && rig.Version != "mandibular-rig/2") || rig.SourceSHA != a.Motion.OriginalSHA || rig.MotionBuild != a.Motion.BuildID || rig.OralBuild != a.Motion.OralBuildID || rig.GeometrySHA != p.SHA256 || len(rig.Weights) != p.Vertices || !rig.Validation.Valid || rig.Validation.MinimumAreaRatio <= .25 || rig.Validation.ChinTravel720 < 3 || !patchFinite(rig.MaximumAngle) || rig.MaximumAngle <= 0 || rig.MaximumAngle > .3 {
			return nil, errors.New("invalid source-bound mandibular companion")
		}
		for _, n := range append(append(rig.Hinge[:], rig.Chin[:]...), rig.OralTravel, rig.CornerTravel, rig.ReferenceGap, rig.LowerStop, rig.Validation.MinimumAreaRatio, rig.Validation.ChinTravel720) {
			if !patchFinite(n) {
				return nil, errors.New("non-finite mandibular calibration")
			}
		}
		if rig.OralTravel <= 0 || rig.CornerTravel < 0 || rig.ReferenceGap <= 0 {
			return nil, errors.New("invalid mandibular range")
		}
		for pose := range a.Oral.poses {
			jaw, ok := rig.PoseJaw[pose]
			gap, found := rig.PoseGap[pose]
			if !ok || !found || !patchFinite(jaw) || jaw < 0 || jaw > 1 || !patchFinite(gap) || gap < 0 {
				return nil, errors.New("incomplete mandibular pose calibration")
			}
		}
		for _, w := range rig.Weights {
			if !patchFinite(w) || w < 0 || w > 1 {
				return nil, errors.New("invalid mandibular weight")
			}
		}
	}
	a.vertices = make([]sourceVertex, p.Vertices)
	for i := range a.vertices {
		at := i * 20
		v := &a.vertices[i]
		values := []*float64{&v.X, &v.Y, &v.Z, &v.Head, &v.Torso}
		for k, d := range values {
			*d = float64(mathFloat32(g[at+k*4:]))
			if math.IsNaN(*d) || math.IsInf(*d, 0) {
				return nil, errors.New("non-finite native geometry")
			}
		}
	}
	if err := a.validateSpeechJaw(); err != nil {
		return nil, err
	}
	decodeIndices := func(at, n int) ([]uint16, error) {
		out := make([]uint16, n)
		for i := range out {
			out[i] = binary.LittleEndian.Uint16(g[at+i*2:])
			if int(out[i]) >= p.Vertices {
				return nil, errors.New("native geometry index out of range")
			}
		}
		return out, nil
	}
	a.indices, err = decodeIndices(p.Vertices*20, p.Triangles*3)
	if err != nil {
		return nil, err
	}
	if err := a.validateSpeechJawSamplingProtection(); err != nil {
		return nil, err
	}
	f, err := decodeIndices(p.Vertices*20+p.Triangles*6, p.FringeTriangles*3)
	if err != nil {
		return nil, err
	}
	a.fringe = make([]fringeTriangle, p.FringeTriangles)
	for i := range a.fringe {
		t := &a.fringe[i]
		copy(t.IDs[:], f[i*3:i*3+3])
		for _, id := range t.IDs {
			v := a.vertices[id]
			c := a.Motion.Projection.Camera
			q := c.Distance - v.Z
			world := vector3{(v.X*float64(a.Motion.Canvas.Width) - c.Center[0]) * q / c.Focal, (v.Y*float64(a.Motion.Canvas.Height) - c.Center[1]) * q / c.Focal, v.Z}
			t.Z += world[2] / 3
			for k := range 3 {
				t.Weighted[k] += (world[k] - a.Motion.Projection.Pivot[k]) * v.Head / 3
			}
		}
	}
	return a, nil
}
