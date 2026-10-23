import type { PosePackage } from '../src/features/character/types/oralTypes.ts'
import type { MotionPackage, Background } from '../src/features/character/types/motionTypes.ts'
import type { GazePackage } from '../src/features/character/types/gazeTypes.ts'
import type { ExpressionPackage } from '../src/features/character/types/expressionTypes.ts'

// An allowlist, rather than deleting known evidence keys: new preparation fields
// must never silently become browser data. Arrays also validate every leaf.
type Shape = 'string' | 'number' | 'boolean' | readonly [Shape] | { [key: string]: Shape }
const fields = (type: Shape, names: string): Record<string, Shape> =>
  Object.fromEntries(names.split(' ').map((name) => [name, type]))
const vector: Shape = ['number']
const matrix: Shape = [vector]
const rect = fields('number', 'x y width height')
const size = fields('number', 'width height')
const asset = fields('string', 'file sha256')
const geometryValidation = { valid: 'boolean', minimum_area_ratio: 'number' } as const

export function project(value: unknown, shape: Shape, path = '$'): unknown {
  if (typeof shape === 'string') {
    if (typeof value !== shape || (shape === 'number' && !Number.isFinite(value)))
      throw Error(`Invalid runtime manifest field: ${path}`)
    return value
  }
  if (Array.isArray(shape)) {
    if (!Array.isArray(value)) throw Error(`Expected runtime array: ${path}`)
    return value.map((item, index) => project(item, shape[0]!, `${path}[${index}]`))
  }
  const input = record(value, path)
  return Object.fromEntries(
    Object.entries(shape).flatMap(([key, child]) => {
      const optional = key.endsWith('?')
      const name = optional ? key.slice(0, -1) : key
      if (!Object.hasOwn(input, name)) {
        if (optional) return []
        throw Error(`Missing runtime manifest field: ${path}.${name}`)
      }
      return [[name, project(input[name], child, `${path}.${name}`)]]
    }),
  )
}

function record(value: unknown, path: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw Error(`Expected runtime object: ${path}`)
  return value as Record<string, unknown>
}
function entries(value: unknown, path: string, convert: (item: unknown) => unknown) {
  return Object.fromEntries(Object.entries(record(value, path)).map(([k, v]) => [k, convert(v)]))
}

const mesh: Shape = {
  ...size,
  ...fields(matrix, 'rest open triangles'),
  cavity_triangles: vector,
}
const pose: Shape = {
  ...fields('string', 'id status endpoint surface mask cavity? aperture_sdf?'),
  ...fields('boolean', 'upper_visible lower_visible'),
  lower_translation: vector,
  'upper_translation?': vector,
  'dental_inset_px?': 'number',
}

export function runtimeOral(value: unknown): PosePackage {
  const input = record(value, 'oral')
  const output = record(
    project(input, {
      'schema?': 'string',
      build_id: 'string',
      canvas: size,
      crop: rect,
      detail: rect,
      reference: fields('string', 'upper lower'),
      'oral?': fields(
        'number',
        'contact_shadow_strength contact_shadow_width_px depth_strength closed_gap_px',
      ),
    }),
    'oral',
  )
  output.poses = entries(input.poses, 'poses', (item) => ({
    ...record(project(item, pose), 'pose'),
    notes: '',
  }))
  output.pairs = entries(input.pairs, 'pairs', (item) => {
    const pair = record(item, 'pair')
    return {
      ...record(project(pair, { safe: 'boolean' }), 'pair'),
      reason: pair.safe ? null : 'The prepared transition is unavailable.',
      mesh: pair.mesh === null ? null : project(pair.mesh, mesh),
    }
  })
  return output as unknown as PosePackage
}

const projection: Shape = {
  ...fields('string', 'version source_sha256 parent_build_id'),
  grid: 'number',
  geometry: { ...asset, ...fields('number', 'vertices triangles fringe_triangles') },
  camera: {
    ...fields(vector, 'size center pivot_px'),
    ...fields('number', 'focal distance near far'),
  },
  pivot: vector,
  bounds: fields('number', 'yaw pitch roll'),
  proposed_idle_degrees: vector,
  validation: geometryValidation,
  'articulation?': {
    version: 'string',
    pivots: matrix,
    rotation_fractions: vector,
    chest_center_px: vector,
    localization: fields('number', 'chin_y base_y'),
    ...fields('number', 'breath_forward_gain breath_lateral_gain'),
    attributes: fields('number', 'stride offset_bytes'),
    validation: geometryValidation,
  },
}

export function runtimeMotion(value: unknown): MotionPackage {
  return project(value, {
    ...fields('string', 'build_id original_sha256 oral_build_id foreground depth'),
    canvas: size,
    'animation_seed?': 'number',
    eyes: { crop: rect, ...fields('string', 'open closed mask'), curves: matrix },
    anatomy: { head_pivot: vector, ...fields('number', 'chin_y face_width face_height') },
    blink: fields(
      'number',
      'closing_ms closed_ms opening_ms minimum_interval_ms maximum_interval_ms',
    ),
    motion: fields(
      'number',
      'yaw_degrees pitch_degrees roll_degrees breath_period_ms breath_face_height_fraction',
    ),
    'rig?': {
      ...fields('string', 'version source_sha256 previous_build_id'),
      grid: 'number',
      vertices: matrix,
      pivot: vector,
      bounds: fields(vector, 'idle speaking'),
      'posture?': {
        version: 'string',
        ...fields('number', 'minimum_yaw_fraction turn_duration_ms'),
      },
      geometry_validation: geometryValidation,
    },
    'behavior?': {
      version: 'string',
      head: {
        idle_degrees: vector,
        ...fields('number', 'maximum_nod_degrees maximum_turn_degrees minimum_gesture_gap_seconds'),
      },
      blink: {
        ...fields(
          'number',
          'rate_per_minute minimum_gap_ms pause_probability ending_probability onset_probability nod_probability',
        ),
        'double?': { ...fields('number', 'probability minimum_interval_ms'), open_gap_ms: vector },
      },
    },
    'breathing?': { version: 'string', ...fields(vector, 'duration_ms inhale_fraction amplitude') },
    'projection?': projection,
  }) as MotionPackage
}

export function runtimeGaze(value: unknown): GazePackage {
  return project(value, {
    ...fields('string', 'version source_sha256 motion_build_id'),
    crop: rect,
    eyes: [{ ...fields(vector, 'center radii'), width_px: 'number', bounds_px: matrix }],
    assets: { 'sclera.png': asset, 'ownership.png': asset },
    validation: fields('boolean', 'valid hidden_iris_generated neutral_source_exact'),
  }) as GazePackage
}

export function runtimeExpressions(value: unknown): ExpressionPackage {
  return project(value, {
    ...fields('string', 'version source_sha256 motion_build_id oral_build_id'),
    crop: rect,
    field_size: vector,
    field: asset,
    ...fields('number', 'units_per_byte inverse_iterations'),
    validation: {
      valid: 'boolean',
      ...fields('number', 'minimum_jacobian maximum_inverse_residual_px'),
    },
    'smile?': {
      ...fields('string', 'version reference_sha256'),
      closed_mouth_only: 'boolean',
      material: asset,
    },
  }) as ExpressionPackage
}

export function runtimeBackground(value: unknown): Background {
  return project(value, fields('string', 'id name image thumbnail source_sha256')) as Background
}
