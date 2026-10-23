import { createCharacterRenderer } from './characterRenderer'
import { createPerspectiveRenderer } from './perspectiveRenderer'
import type { MotionImages, MotionPackage } from '../types/motionTypes'
import type { NeckMode } from './neckDeformation'
import type { ExpressionPackage, ExpressionState } from '../types/expressionTypes'
import type { Rect } from '../types/oralTypes'

export function createBodyRenderer(
  images: MotionImages,
  data: MotionPackage,
  unavailable: () => void,
  neckMode: NeckMode = 'full',
  expressions?: ExpressionPackage,
): ReturnType<typeof createCharacterRenderer> & {
  setExpression?: (value: ExpressionState) => void
  setClip?: (rect: Rect) => void
} {
  if (data.projection) {
    if (!images.geometry) throw Error('Missing prepared projection geometry')
    return createPerspectiveRenderer(
      images.foreground,
      data,
      images.geometry,
      unavailable,
      1024,
      neckMode,
      expressions && images.expressionField
        ? { package: expressions, field: images.expressionField, smile: images.smileMaterial }
        : undefined,
    )
  }
  return createCharacterRenderer(images.foreground, images.depth, data, unavailable)
}
