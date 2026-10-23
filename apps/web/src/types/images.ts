export interface ImageMetadata {
  format: 'JPEG' | 'PNG' | 'WEBP'
  mime_type: 'image/jpeg' | 'image/png' | 'image/webp'
  width: number
  height: number
  size_bytes: number
}

export interface ValidationError {
  code: string
  message: string
}
export type ValidationStatus = 'idle' | 'loading' | 'success' | 'error'
export const MAX_IMAGE_BYTES = 5_000_000

export function isImageMetadata(value: unknown): value is ImageMetadata {
  if (typeof value !== 'object' || value === null) return false
  const data = value as Record<string, unknown>
  const mime = { JPEG: 'image/jpeg', PNG: 'image/png', WEBP: 'image/webp' }
  return (
    typeof data.format === 'string' &&
    data.format in mime &&
    data.mime_type === mime[data.format as keyof typeof mime] &&
    Number.isInteger(data.width) &&
    Number(data.width) > 0 &&
    Number.isInteger(data.height) &&
    Number(data.height) > 0 &&
    Number.isInteger(data.size_bytes) &&
    Number(data.size_bytes) > 0 &&
    Number(data.size_bytes) <= MAX_IMAGE_BYTES
  )
}
