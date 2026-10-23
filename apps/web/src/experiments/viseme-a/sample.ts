// Coordinates and lossless assets from .tools/viseme-a-05/run.json.
import baseUrl from './base.webp'
import mouthUrl from './mouth-a.webp'
import textureUrl from './mouth-texture.webp'

export const sample = {
  baseUrl,
  mouthUrl,
  textureUrl,
  width: 1792,
  height: 2400,
  mouth: { x: 740, y: 998, width: 320, height: 210 },
  detail: { x: 620, y: 860, width: 560, height: 480 },
} as const
