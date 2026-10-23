/** Opaque reference enamel, hidden by an advancing edge instead of a dissolve. */
export interface DentalBounds {
  top: number
  bottom: number
}

export function dentalBounds(image: HTMLImageElement): DentalBounds {
  const canvas = document.createElement('canvas')
  canvas.width = image.naturalWidth
  canvas.height = image.naturalHeight
  const context = canvas.getContext('2d')!
  context.drawImage(image, 0, 0)
  const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data
  let top = canvas.height,
    bottom = 0
  for (let y = 0; y < canvas.height; y++)
    for (let x = 0; x < canvas.width; x++) {
      if (pixels[(y * canvas.width + x) * 4 + 3] > 0) {
        top = Math.min(top, y)
        bottom = Math.max(bottom, y + 1)
      }
    }
  return { top, bottom }
}

export function drawDentalRow(
  context: CanvasRenderingContext2D,
  image: HTMLImageElement,
  bounds: DentalBounds,
  row: 'upper' | 'lower',
  exposure: number,
  dx = 0,
  dy = 0,
) {
  if (exposure <= 0 || bounds.bottom <= bounds.top) return
  context.save()
  context.translate(dx, dy)
  if (exposure < 1) {
    // Extend past alpha support so the final frames contain no residual strip.
    const margin = 2,
      travel = bounds.bottom - bounds.top + margin * 2
    const edge =
      row === 'upper'
        ? bounds.bottom + margin - exposure * travel
        : bounds.top - margin + exposure * travel
    context.beginPath()
    context.rect(
      0,
      row === 'upper' ? edge : 0,
      image.naturalWidth,
      row === 'upper' ? Math.max(0, image.naturalHeight - edge) : Math.max(0, edge),
    )
    context.clip()
  }
  context.drawImage(image, 0, 0)
  context.restore()
}
