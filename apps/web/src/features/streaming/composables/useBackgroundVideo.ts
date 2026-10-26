import { onScopeDispose, shallowRef, watch } from 'vue'
import { createBackgroundCompositor } from '../backgroundCompositor'
import { backgrounds } from '../settingsCatalog'
import type { StreamSettings } from '../types'

// Owns only receiver-side background composition and its captured presentation
// track. The original audio tracks remain untouched and are never stopped here.
export function useBackgroundVideo(
  props: () => {
    media: MediaStream | null
    background: StreamSettings['background']
    blur: number
  },
) {
  const media = shallowRef<MediaStream | null>(null)
  const error = shallowRef('')
  let redraw: (() => void) | undefined
  let setBackground: (() => void) | undefined
  watch(
    () => props().media,
    (stream, _previous, onCleanup) => {
      error.value = ''
      if (!stream) {
        media.value = null
        return
      }
      const output = new MediaStream(stream.getAudioTracks())
      media.value = output
      let source: HTMLVideoElement | undefined
      let capture: MediaStream | undefined
      let compositor: ReturnType<typeof createBackgroundCompositor> | undefined
      let image: HTMLImageElement | undefined
      let imageURL = ''
      let frame = 0,
        fallback = 0,
        disposed = false,
        failed = false
      const canvas = document.createElement('canvas')
      function fail(failure: unknown) {
        failed = true
        error.value = failure instanceof Error ? failure.message : 'Live video composition failed.'
      }
      function background() {
        const settings = props()
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
        if (disposed || failed || !source || source.readyState < 2) return
        try {
          if (!(
            (source.videoWidth === 1440 && source.videoHeight === 960) ||
            (source.videoWidth === 1080 && source.videoHeight === 720)
          ))
            throw Error('The renderer must be updated for browser backgrounds.')
          if (!compositor) {
            compositor = createBackgroundCompositor(canvas, source.videoWidth, source.videoHeight)
            background()
            if (!canvas.captureStream)
              throw Error('This browser cannot present composed live video.')
          }
          if (image?.complete && image.naturalWidth) {
            compositor.draw(source)
            // The capture track's initial frame must already contain the scene.
            if (!capture) {
              capture = canvas.captureStream(0)
              if (!(capture.getVideoTracks()[0] as CanvasCaptureMediaStreamTrack).requestFrame) {
                capture.getVideoTracks().forEach((track) => track.stop())
                capture = canvas.captureStream(60)
              }
              capture.getVideoTracks().forEach((track) => output.addTrack(track))
            }
            ;(
              capture?.getVideoTracks()[0] as CanvasCaptureMediaStreamTrack | undefined
            )?.requestFrame?.()
          }
        } catch (failure) {
          fail(failure)
        }
      }
      function schedule() {
        if (disposed || failed || !source) return
        if (source.requestVideoFrameCallback)
          frame = source.requestVideoFrameCallback(() => {
            draw()
            schedule()
          })
        else
          fallback = window.setTimeout(() => {
            draw()
            schedule()
          }, 33)
      }
      function tracks() {
        for (const track of stream!.getAudioTracks())
          if (!output.getTrackById(track.id)) output.addTrack(track)
        if (source || !stream!.getVideoTracks().length) return
        source = document.createElement('video')
        source.muted = true
        source.autoplay = true
        source.playsInline = true
        source.srcObject = stream!
        source.addEventListener('loadeddata', draw)
        source.play().catch(() => {
          if (!disposed) fail(Error('Live video could not start. Reconnect to try again.'))
        })
        schedule()
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
      background()
      tracks()
      stream.addEventListener('addtrack', tracks)
      onCleanup(() => {
        disposed = true
        redraw = undefined
        setBackground = undefined
        stream.removeEventListener('addtrack', tracks)
        if (source) {
          source.cancelVideoFrameCallback?.(frame)
          source.pause()
          source.srcObject = null
        }
        clearTimeout(fallback)
        capture?.getVideoTracks().forEach((track) => track.stop())
        compositor?.dispose()
        if (image) {
          image.onload = null
          image.onerror = null
        }
      })
    },
    { immediate: true },
  )
  watch(
    () => [props().background, props().blur],
    () => setBackground?.(),
  )
  const resume = () => redraw?.()
  document.addEventListener('visibilitychange', resume)
  onScopeDispose(() => document.removeEventListener('visibilitychange', resume))
  return { media, error }
}
