import { onScopeDispose, readonly, shallowRef } from 'vue'
import {
  isImageMetadata,
  MAX_IMAGE_BYTES,
  type ImageMetadata,
  type ValidationError,
  type ValidationStatus,
} from '../types/images'

export function useImageValidation() {
  const file = shallowRef<File | null>(null)
  const status = shallowRef<ValidationStatus>('idle')
  const result = shallowRef<ImageMetadata | null>(null)
  const error = shallowRef<ValidationError | null>(null)
  const previewUrl = shallowRef<string | null>(null)
  let controller: AbortController | null = null
  let sequence = 0

  function clear() {
    sequence++
    controller?.abort()
    controller = null
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    file.value = null
    result.value = null
    error.value = null
    status.value = 'idle'
  }

  function rejectSelection(message: string) {
    clear()
    error.value = { code: 'invalid_selection', message }
    status.value = 'error'
  }

  async function validate(selected: File) {
    clear()
    file.value = selected
    if (selected.size === 0 || selected.size > MAX_IMAGE_BYTES) {
      error.value =
        selected.size === 0
          ? { code: 'empty_file', message: 'The selected file is empty. Choose another image.' }
          : {
              code: 'file_too_large',
              message: 'Choose an image no larger than 5 MB (5,000,000 bytes).',
            }
      status.value = 'error'
      return
    }
    status.value = 'loading'
    const current = sequence
    const request = new AbortController()
    controller = request
    let timedOut = false
    const timeout = setTimeout(() => {
      timedOut = true
      request.abort()
    }, 20_000)
    try {
      const body = new FormData()
      body.append('image', selected)
      const response = await fetch('/api/v1/images/validate', {
        method: 'POST',
        body,
        signal: request.signal,
      })
      const payload: unknown = await response.json()
      if (current !== sequence) return
      if (!response.ok) {
        const failure = payload as { error?: ValidationError }
        error.value =
          failure?.error && typeof failure.error.message === 'string'
            ? failure.error
            : {
                code: 'validation_failed',
                message: 'The image could not be checked. Please try again.',
              }
        status.value = 'error'
        return
      }
      if (!isImageMetadata(payload) || payload.size_bytes !== selected.size)
        throw new Error('Invalid response')
      result.value = payload
      // Use the verified MIME type even when the filename or browser MIME claim is misleading.
      previewUrl.value = URL.createObjectURL(new Blob([selected], { type: payload.mime_type }))
      status.value = 'success'
    } catch {
      if (current !== sequence) return
      error.value = {
        code: 'connection_error',
        message: timedOut
          ? 'Image validation took too long. Please try again.'
          : 'Could not reach image validation. Please try again.',
      }
      status.value = 'error'
    } finally {
      clearTimeout(timeout)
      if (current === sequence) controller = null
    }
  }

  function retry() {
    if (file.value) void validate(file.value)
  }
  onScopeDispose(clear)
  return {
    file: readonly(file),
    status: readonly(status),
    result: readonly(result),
    error: readonly(error),
    previewUrl: readonly(previewUrl),
    validate,
    clear,
    rejectSelection,
    retry,
  }
}
