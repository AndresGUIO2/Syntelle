<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import { computed, onMounted, onScopeDispose, shallowRef, useTemplateRef, watch } from 'vue'
import AppIcon from '../../../components/AppIcon.vue'
import { useBackgroundVideo } from '../composables/useBackgroundVideo'
import type { StreamSettings } from '../types'
const { t, errorMessage } = useLocaleStore()
const props = defineProps<{
  media: MediaStream | null
  ready: boolean
  suppressed: boolean
  background: StreamSettings['background']
  blur: number
  controlsTarget: HTMLElement | null
}>()
const composed = useBackgroundVideo(() => props)
const video = useTemplateRef<HTMLVideoElement>('video')
const frameReady = shallowRef(false)
const showVideo = computed(() => props.ready && frameReady.value)
const muted = shallowRef(true)
const audioError = shallowRef('')
let audioAttempt = 0
type SafariVideo = HTMLVideoElement & {
  webkitSupportsPresentationMode?: (mode: string) => boolean
  webkitSetPresentationMode?: (mode: string) => void
  webkitPresentationMode?: string
}
const pipSupported = shallowRef(false)
const pipActive = shallowRef(false)
const pipPending = shallowRef(false)
const pipError = shallowRef('')
// The browser grants this action when its automatic-PiP eligibility and site
// permission checks pass. A visibility handler alone cannot grant activation.
const autoPipAction = 'enterpictureinpicture' as Parameters<
  typeof navigator.mediaSession.setActionHandler
>[0]
let autoPipRegistered = false
function updatePictureInPicture() {
  const element = video.value as SafariVideo | null
  pipSupported.value = Boolean(
    (document.pictureInPictureEnabled && element?.requestPictureInPicture) ||
    (element?.webkitSetPresentationMode &&
      element.webkitSupportsPresentationMode?.('picture-in-picture')),
  )
  pipActive.value = Boolean(
    element &&
    (document.pictureInPictureElement === element ||
      element.webkitPresentationMode === 'picture-in-picture'),
  )
}
async function changePictureInPicture(automatic = false) {
  const element = video.value as SafariVideo | null
  if (!element || !props.ready || pipPending.value) return
  const alreadyOpen =
    document.pictureInPictureElement === element ||
    element.webkitPresentationMode === 'picture-in-picture'
  // Automatic callbacks only open the current live video. They never toggle
  // an existing window closed or replace another application's PiP window.
  if (automatic && (!document.hidden || alreadyOpen || document.pictureInPictureElement)) return
  pipPending.value = true
  pipError.value = ''
  try {
    if (document.pictureInPictureElement === element) await document.exitPictureInPicture()
    else if (element.webkitPresentationMode === 'picture-in-picture')
      element.webkitSetPresentationMode?.('inline')
    else if (document.pictureInPictureEnabled && element.requestPictureInPicture)
      await element.requestPictureInPicture()
    else element.webkitSetPresentationMode?.('picture-in-picture')
    updatePictureInPicture()
  } catch {
    if (!automatic) pipError.value = 'Picture in picture could not open. Try again or use this tab.'
  } finally {
    pipPending.value = false
  }
}
onMounted(() => {
  try {
    navigator.mediaSession?.setActionHandler(autoPipAction, () => {
      void changePictureInPicture(true)
    })
    autoPipRegistered = Boolean(navigator.mediaSession)
  } catch {
    // Unsupported actions keep the manual video control available.
  }
})
watch([video, () => props.ready], updatePictureInPicture, { flush: 'post' })
watch(
  [video, composed.media],
  ([element, stream], _previous, onCleanup) => {
    frameReady.value = false
    if (!element) return
    element.srcObject = stream
    audioAttempt++
    audioError.value = ''
    pipError.value = ''
    if (stream) {
      const presented = () => {
        if (element.srcObject === stream && element.readyState >= 2) frameReady.value = true
      }
      if (element.requestVideoFrameCallback) {
        const callback = element.requestVideoFrameCallback(presented)
        onCleanup(() => element.cancelVideoFrameCallback(callback))
      } else {
        element.addEventListener('loadeddata', presented)
        element.addEventListener('playing', presented)
        onCleanup(() => {
          element.removeEventListener('loadeddata', presented)
          element.removeEventListener('playing', presented)
        })
      }
    }
    if (stream)
      void element.play().catch((failure: unknown) => {
        if (
          element.srcObject !== stream ||
          (failure instanceof DOMException && failure.name === 'AbortError')
        )
          return
        audioError.value = 'Select Unmute sound to hear Syntelle.'
      })
  },
  { flush: 'post' },
)
async function enableAudio() {
  const element = video.value
  if (!element) return
  const attempt = ++audioAttempt
  const stream = element.srcObject
  muted.value = false
  element.muted = props.suppressed
  try {
    await element.play()
    if (attempt === audioAttempt) audioError.value = ''
  } catch {
    if (attempt !== audioAttempt || element.srcObject !== stream) return
    muted.value = true
    element.muted = true
    audioError.value = 'Sound could not start. Select Unmute sound to try again.'
  }
}
function toggleAudio() {
  if (muted.value || audioError.value) {
    void enableAudio()
    return
  }
  audioAttempt++
  muted.value = true
  audioError.value = ''
  if (video.value) video.value.muted = true
}
onScopeDispose(() => {
  audioAttempt++
  if (autoPipRegistered) navigator.mediaSession.setActionHandler(autoPipAction, null)
  if (video.value && document.pictureInPictureElement === video.value)
    void document.exitPictureInPicture().catch(() => undefined)
  const element = video.value as SafariVideo | null
  if (element?.webkitPresentationMode === 'picture-in-picture')
    element.webkitSetPresentationMode?.('inline')
  if (video.value) {
    video.value.pause()
    video.value.srcObject = null
  }
})
defineExpose({ enableAudio })
</script>

<template>
  <section
    class="remote-stage"
    :aria-label="t(showVideo ? 'Syntelle live video' : 'Syntelle portrait')"
    :aria-busy="Boolean(media) && !showVideo"
  >
    <div class="remote-media">
      <picture
        class="remote-poster"
        :class="{ 'remote-poster-live': showVideo }"
        :aria-hidden="showVideo"
      >
        <img
          :src="`/characters/posters/syntelle-${background}-portrait.webp`"
          :alt="t('Syntelle portrait')"
          width="720"
          height="960"
          fetchpriority="high"
        />
      </picture>
      <video
        v-show="Boolean(media)"
        ref="video"
        class="remote-video"
        autoplay
        playsinline
        :muted="muted || suppressed"
        :aria-label="t('Animated Syntelle with synchronized speech')"
        :aria-hidden="!showVideo"
        @loadedmetadata="updatePictureInPicture"
        @enterpictureinpicture="updatePictureInPicture"
        @leavepictureinpicture="updatePictureInPicture"
        @webkitpresentationmodechanged="updatePictureInPicture"
      />
      <p v-if="composed.error.value" class="audio-error" role="alert">
        {{ errorMessage(composed.error.value) }}
      </p>
      <p v-if="audioError" class="audio-error" role="status">{{ t(audioError) }}</p>
      <p v-if="pipError" class="audio-error" role="status">{{ t(pipError) }}</p>
    </div>
    <Teleport v-if="ready && controlsTarget" :to="controlsTarget">
      <div class="player-controls" role="group" :aria-label="t('Video controls')">
        <button
          v-if="ready"
          class="audio-action"
          type="button"
          :aria-label="t(muted ? 'Unmute sound' : 'Mute sound')"
          :title="t(muted ? 'Unmute sound' : 'Mute sound')"
          :aria-pressed="!muted"
          @click="toggleAudio"
        >
          <AppIcon :name="muted ? 'volume-off' : 'volume'" />
        </button>
        <button
          v-if="ready && pipSupported"
          class="audio-action pip-action"
          type="button"
          :disabled="pipPending"
          :aria-pressed="pipActive"
          :aria-label="t(pipActive ? 'Return to tab' : 'Picture in picture')"
          :title="t(pipActive ? 'Return video to tab' : 'Keep Syntelle in a floating window')"
          @click="changePictureInPicture()"
        >
          <AppIcon name="pip" />
        </button>
      </div>
    </Teleport>
  </section>
</template>

<style scoped>
.remote-stage {
  position: relative;
  display: grid;
  place-items: center;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  background: #19181b;
  border: 1px solid var(--divider);
  border-radius: 16px;
  box-shadow: var(--panel-shadow);
}
.remote-media {
  position: absolute;
  inset: 0;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}
.remote-video,
.remote-poster img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.remote-video {
  position: absolute;
  inset: 0;
}
.remote-poster {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  z-index: 1;
  opacity: 1;
  pointer-events: none;
}
.remote-poster-live {
  opacity: 0;
  transition: opacity 240ms ease-out;
}
@media (prefers-reduced-motion: reduce) {
  .remote-poster-live {
    /* Only opacity changes; retain a brief handoff under global motion suppression. */
    transition: opacity 80ms ease-out !important;
  }
}
.remote-poster img {
  object-position: center;
}
@media (max-width: 899px) {
  .remote-video,
  .remote-poster img {
    object-position: left center;
  }
}
.audio-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  min-height: 44px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text);
  background: transparent;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  line-height: 20px;
  cursor: pointer;
  transition: background-color 150ms ease;
}
.player-controls {
  display: flex;
  align-items: center;
  gap: 0;
}
.audio-action:focus-visible {
  background: var(--subtle);
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
.audio-action:disabled {
  opacity: 0.6;
  cursor: default;
}
@media (hover: hover) {
  .audio-action:hover:not(:disabled) {
    background: var(--subtle);
  }
}
.audio-action:active:not(:disabled) {
  background: var(--divider);
}
.audio-error {
  position: absolute;
  z-index: 2;
  left: 16px;
  right: 16px;
  bottom: 16px;
  margin: 0;
  padding: 8px 12px;
  border-radius: 6px;
  background: #242226;
  font-size: 12px;
  color: white;
  text-align: left;
}
</style>
