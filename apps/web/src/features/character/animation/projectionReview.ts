import { easeMotion, type HeadPose } from './headMotion'

export const projectionReviewDuration = 16000
export function projectionReviewPose(ms: number): HeadPose {
  const keys = [
    [0, 0, 0],
    [2500, 3, 0],
    [4000, 3, 0],
    [6500, -3, 0],
    [8000, -3, 0],
    [10000, 0, 1],
    [11500, 0, 1],
    [13500, 0, -1],
    [14500, 0, -1],
    [16000, 0, 0],
  ]
  const t = Math.max(0, Math.min(projectionReviewDuration, ms))
  const i = keys.findIndex((key, index) => index > 0 && t <= key[0]!)
  if (i < 1) return { yaw: 0, pitch: 0, roll: 0 }
  const a = keys[i - 1]!,
    b = keys[i]!,
    amount = easeMotion((t - a[0]!) / (b[0]! - a[0]!))
  return { yaw: a[1]! + (b[1]! - a[1]!) * amount, pitch: a[2]! + (b[2]! - a[2]!) * amount, roll: 0 }
}
