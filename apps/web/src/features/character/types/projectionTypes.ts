export interface ProjectionRig {
  version: 'rgbd-perspective/1'
  source_sha256: string
  parent_build_id: string
  grid: number
  geometry: {
    file: string
    sha256: string
    vertices: number
    triangles: number
    fringe_triangles: number
  }
  camera: {
    size: number[]
    center: number[]
    focal: number
    distance: number
    near: number
    far: number
    pivot_px: number[]
  }
  pivot: number[]
  bounds: { yaw: number; pitch: number; roll: number }
  proposed_idle_degrees: number[]
  validation: { valid: boolean; minimum_area_ratio: number }
  articulation?: {
    version: 'neck-volume/1' | 'neck-volume/2' | 'neck-volume/3'
    pivots: number[][]
    rotation_fractions: number[]
    chest_center_px: number[]
    localization: { chin_y: number; base_y: number }
    breath_forward_gain: number
    breath_lateral_gain: number
    attributes: { stride: number; offset_bytes: number }
    validation: { valid: boolean; minimum_area_ratio: number }
  }
}

export function decodeProjection(buffer: ArrayBuffer, rig: ProjectionRig) {
  const g = rig.geometry,
    vertexBytes = g.vertices * 5 * 4,
    indexBytes = g.triangles * 3 * 2
  if (
    g.vertices > 65535 ||
    g.vertices < 4 ||
    buffer.byteLength !==
      (rig.articulation
        ? rig.articulation.attributes.offset_bytes + g.vertices * 6 * 4
        : vertexBytes + indexBytes + g.fringe_triangles * 3 * 2)
  )
    throw Error('Invalid prepared projection geometry size')
  const vertices = new Float32Array(buffer, 0, g.vertices * 5)
  const indices = new Uint16Array(buffer, vertexBytes, g.triangles * 3)
  const fringe = new Uint16Array(buffer, vertexBytes + indexBytes, g.fringe_triangles * 3)
  let articulation: Float32Array | undefined
  if (rig.articulation) {
    const a = rig.articulation.attributes
    const expectedOffset = Math.ceil((vertexBytes + indexBytes + g.fringe_triangles * 6) / 4) * 4
    if (a.stride !== 6 || a.offset_bytes !== expectedOffset || !rig.articulation.validation.valid)
      throw Error('Invalid prepared neck geometry')
    articulation = new Float32Array(buffer, a.offset_bytes, g.vertices * 6)
    if (articulation.some((v) => !Number.isFinite(v))) throw Error('Invalid neck weights')
    for (let i = 0; i < articulation.length; i += 6)
      if (
        articulation[i]! < 0 ||
        articulation[i + 1]! < 0 ||
        articulation[i + 2]! < 0 ||
        articulation[i]! + articulation[i + 1]! + articulation[i + 2]! > 1.00001
      )
        throw Error('Neck influence weights are invalid')
  }
  if (
    vertices.some((v) => !Number.isFinite(v)) ||
    indices.some((v) => v >= g.vertices) ||
    fringe.some((v) => v >= g.vertices)
  )
    throw Error('Invalid prepared projection coordinates')
  return { vertices, indices, fringe, articulation }
}
