export interface Rect {
  x: number
  y: number
  width: number
  height: number
}
export interface PoseMesh {
  width: number
  height: number
  rest: number[][]
  open: number[][]
  triangles: number[][]
  cavity_triangles: number[]
}
export interface PoseDefinition {
  id: string
  status: string
  notes: string
  upper_visible: boolean
  lower_visible: boolean
  lower_translation: number[]
  endpoint: string
  surface: string
  mask: string
  dental_inset_px?: number
  cavity?: string
  aperture_sdf?: string
  upper_translation?: number[]
  comparison?: { original: string; semantic: string; previous?: string }
  preparation?: { ready: boolean; issues: string[] }
}
export interface PoseImages {
  data: PoseDefinition
  endpoint: HTMLImageElement
  surface: HTMLImageElement
  cavity?: HTMLImageElement
  mask: HTMLImageElement
  frame: HTMLCanvasElement
}
export interface DentalSelection {
  method: string
  selected_pose: string
  status: string
  ranking: string[]
  score_margin: number | null
  new_provider_requests: number
  review_status: string
  original_preview: string
  mask_preview: string
  mask_review?: { method: string; version: string; status: string }
  candidates: Record<
    string,
    {
      eligible: boolean
      score: number
      reasons: string[]
      upper_span_ratio?: number
      lower_span_ratio?: number
    }
  >
}
export interface PosePackage {
  schema?: string
  oral?: OralLighting
  build_id: string
  canvas: { width: number; height: number }
  crop: Rect
  detail: Rect
  poses: Record<string, PoseDefinition>
  pairs: Record<string, { safe: boolean; reason: string | null; mesh: PoseMesh | null }>
  reference: { upper: string; lower: string; selection?: DentalSelection }
}
export interface OralLighting {
  contact_shadow_strength: number
  contact_shadow_width_px: number
  depth_strength: number
  closed_gap_px: number
  visible_aperture?: {
    version: 'visible-aperture-lighting/1'
    reference_gap_px: number
    minimum_irradiance: number
    full_light_ratio: number
    contact_width_px: number
    measurement: 'mask-b-column-coverage/1'
  }
}
