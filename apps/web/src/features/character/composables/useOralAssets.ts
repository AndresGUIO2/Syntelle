import type { OralContext } from '../types/packageContext'
import type { PoseImages } from '../types/oralTypes'
import { usePreparedImages } from './usePreparedImages'

export function useOralAssets(context: OralContext) {
  const p = context.package
  return usePreparedImages(async (loadImage) => {
    const load = (name: string) => loadImage(context.assetUrl(name))
    const [base, upper, lower] = await Promise.all([
      load('base.webp'),
      load(p.reference.upper),
      load(p.reference.lower),
    ])
    const entries = await Promise.all(
      Object.entries(p.poses).map(async ([id, data]) => {
        const [endpoint, surface, mask, cavity] = await Promise.all([
          load(data.endpoint),
          load(data.surface),
          load(data.mask),
          data.cavity ? load(data.cavity) : undefined,
        ])
        const frame = document.createElement('canvas')
        frame.width = p.crop.width
        frame.height = p.crop.height
        const canvas = frame.getContext('2d')!
        canvas.drawImage(base, -p.crop.x, -p.crop.y)
        canvas.drawImage(endpoint, 0, 0)
        return [id, { data, endpoint, surface, cavity, mask, frame } satisfies PoseImages] as const
      }),
    )
    return { base, upper, lower, poses: Object.fromEntries(entries) }
  }, 'The trial images could not load. Reload this page to try again.')
}
