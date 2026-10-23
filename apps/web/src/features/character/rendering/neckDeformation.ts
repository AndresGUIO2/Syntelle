import type { ProjectionRig } from '../types/projectionTypes'

export type NeckMode = 'previous' | 'neck' | 'full'

export function neckTransforms(rig: NonNullable<ProjectionRig['articulation']>, degrees: number[]) {
  const values = new Float32Array(48)
  for (let j = 0; j < 3; j++) {
    const [yaw, pitch, roll] = degrees.map((v) => (v * rig.rotation_fractions[j]! * Math.PI) / 180)
    const cy = Math.cos(yaw!),
      sy = Math.sin(yaw!),
      cp = Math.cos(pitch!),
      sp = Math.sin(pitch!),
      cr = Math.cos(roll!),
      sr = Math.sin(roll!)
    const m = [
      cr * cy - sr * sp * sy,
      sr * cy + cr * sp * sy,
      -cp * sy,
      0,
      -sr * cp,
      cr * cp,
      sp,
      0,
      cr * sy + sr * sp * cy,
      sr * sy - cr * sp * cy,
      cp * cy,
      0,
      0,
      0,
      0,
      1,
    ]
    const p = rig.pivots[j]!
    for (let a = 0; a < 3; a++)
      m[12 + a] = p[a]! - m[a]! * p[0]! - m[4 + a]! * p[1]! - m[8 + a]! * p[2]!
    values.set(m, j * 16)
  }
  return values
}
