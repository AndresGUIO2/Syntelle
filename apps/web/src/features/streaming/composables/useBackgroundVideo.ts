import { onScopeDispose, shallowRef, watch, type Ref } from 'vue'
import { createBackgroundCompositor } from '../backgroundCompositor'
import { backgrounds } from '../settingsCatalog'
import type { StreamSettings } from '../types'

// Inline rendering samples the native A/V playback element directly. Only PiP
// requests create a capture track; the original audio never changes clock/rate.
export function useBackgroundVideo(options: {
  settings: () => {
    media: MediaStream | null
    background: StreamSettings['background']
    blur: number
  }
  source: Readonly<Ref<HTMLVideoElement | null>>
  canvas: Readonly<Ref<HTMLCanvasElement | null>>
}) {
  const ready = shallowRef(false)
  const error = shallowRef('')
  let redraw: (() => void) | undefined
  let setBackground: (() => void) | undefined
  let capture: MediaStream | undefined
  function stopCapture() {
    capture?.getVideoTracks().forEach((track) => track.stop())
    capture = undefined
  }
  function captureForPictureInPicture() {
    const canvas = options.canvas.value
    if (!canvas || !ready.value || !canvas.captureStream)
      throw Error('This browser cannot present composed live video.')
    if (!capture) {
      capture = canvas.captureStream(0)
      if (!(capture.getVideoTracks()[0] as CanvasCaptureMediaStreamTrack).requestFrame) {
        stopCapture()
        capture = canvas.captureStream(60)
      }
    }
    redraw?.()
    return capture
  }
  watch(
    [() => options.settings().media, options.source, options.canvas],
    ([stream, source, canvas], _previous, onCleanup) => {
      ready.value = false
      error.value = ''
      stopCapture()
      if (!stream || !source || !canvas) return
      let compositor: ReturnType<typeof createBackgroundCompositor> | undefined
      let image: HTMLImageElement | undefined
      let imageURL = ''
      let frame = 0,
        fallback = 0,
        disposed = false,
        failed = false
      function fail(failure: unknown) {
        failed = true
        error.value = failure instanceof Error ? failure.message : 'Live video composition failed.'
      }
      function background() {
        const settings = options.settings()
        const url = backgrounds.find((item) => item.id === settings.background)!.image
        if (url !== imageURL) {
          imageURL = url
          const loading = new Image()
          image = loading
          loading.onload = () => {
            if (!disposed && image === loading) {
              try {
                background()
                draw()
              } catch (failure) {
                fail(failure)
              }
            }
          }
          loading.onerror = () => {
            if (!disposed && image === loading) fail(Error('The background could not load.'))
          }
          loading.src = url
        } else if (compositor && image?.complete && image.naturalWidth) {
          compositor.setBackground(image, settings.blur)
        }
      }
      function draw() {
        if (disposed || failed || source!.readyState < 2) return
        try {
          if (!(
            (source!.videoWidth === 1440 && source!.videoHeight === 960) ||
            (source!.videoWidth === 1080 && source!.videoHeight === 720)
          ))
            throw Error('The renderer must be updated for browser backgrounds.')
          if (!compositor) {
            compositor = createBackgroundCompositor(
              canvas!,
              source!.videoWidth,
              source!.videoHeight,
            )
            background()
          }
          if (image?.complete && image.naturalWidth) {
            compositor.draw(source!)
            ready.value = true
            ;(
              capture?.getVideoTracks()[0] as CanvasCaptureMediaStreamTrack | undefined
            )?.requestFrame?.()
          }
        } catch (failure) {
          fail(failure)
        }
      }
      function schedule() {
        if (disposed || failed) return
        if (typeof source!.requestVideoFrameCallback === 'function')
          frame = source!.requestVideoFrameCallback(() => {
            draw()
            schedule()
          })
        else
          fallback = window.setTimeout(() => {
            draw()
            schedule()
          }, 33)
      }
      redraw = draw
      setBackground = () => {
        try {
          background()
          draw()
        } catch (failure) {
          fail(failure)
        }
      }
      source.addEventListener('loadeddata', draw)
      background()
      draw()
      schedule()
      onCleanup(() => {
        disposed = true
        redraw = undefined
        setBackground = undefined
        source.cancelVideoFrameCallback?.(frame)
        source.removeEventListener('loadeddata', draw)
        clearTimeout(fallback)
        stopCapture()
        compositor?.dispose()
        if (image) {
          image.onload = null
          image.onerror = null
        }
      })
    },
    { immediate: true, flush: 'post' },
  )
  watch(
    () => [options.settings().background, options.settings().blur],
    () => setBackground?.(),
  )
  const resume = () => redraw?.()
  document.addEventListener('visibilitychange', resume)
  onScopeDispose(() => {
    document.removeEventListener('visibilitychange', resume)
    stopCapture()
  })
  return { ready, error, captureForPictureInPicture, stopCapture }
}
