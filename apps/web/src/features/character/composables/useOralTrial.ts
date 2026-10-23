import { computed, onScopeDispose, shallowRef, watch, watchEffect } from 'vue'
import { createPoseMorph } from '../rendering/poseMorph'
import { createCompletedMorph } from '../rendering/completedMorph'
import { useVisemePlayback } from './useVisemePlayback'
import { useOralAssets } from './useOralAssets'
import type { OralContext } from '../types/packageContext'

export function useOralTrial(context: OralContext) {
  const { package: posePackage, completed: completedTrial } = context
  const { amount, playing, reducedMotion, duration, hold, pose, toggle } = useVisemePlayback()
  const { images, error } = useOralAssets(context)
  const sourceId = shallowRef('Rest')
  const targetId = shallowRef(
    posePackage.poses.A
      ? 'A'
      : (Object.keys(posePackage.poses).find((id) => id !== 'Rest') ?? 'Rest'),
  )
  const source = computed(() => images.value?.poses[sourceId.value])
  const target = computed(() => images.value?.poses[targetId.value])
  const pair = computed(() => posePackage.pairs[`${sourceId.value}-${targetId.value}`])
  const pairSafe = computed(() => pair.value?.safe === true)
  const showBounds = shallowRef(false)
  const mode = shallowRef<'morph' | 'crossfade'>('morph')
  const renderer = shallowRef<ReturnType<typeof createPoseMorph>>(null)
  const morphFrame = shallowRef<HTMLCanvasElement | null>(null)
  const morphUnavailable = shallowRef(false)
  const poseLabel = computed(() =>
    amount.value < 0.001 ? sourceId.value : amount.value > 0.999 ? targetId.value : 'Transition',
  )

  watch(
    [sourceId, targetId, images],
    () => {
      pose(0)
      renderer.value?.dispose()
      renderer.value = null
      morphFrame.value = null
      morphUnavailable.value = false
      if (images.value && source.value && target.value && pairSafe.value && pair.value.mesh) {
        renderer.value = (completedTrial ? createCompletedMorph : createPoseMorph)(
          source.value,
          target.value,
          pair.value.mesh,
          images.value.upper,
          images.value.lower,
          () => {
            pose(amount.value)
            mode.value = 'crossfade'
            morphUnavailable.value = true
          },
          posePackage.oral,
        )
        if (!renderer.value) {
          mode.value = 'crossfade'
          morphUnavailable.value = true
        }
      }
    },
    { immediate: true },
  )
  watchEffect(() => {
    if (
      mode.value === 'morph' &&
      renderer.value &&
      (completedTrial || (amount.value > 0 && amount.value < 1))
    ) {
      renderer.value.draw(amount.value)
      morphFrame.value = renderer.value.canvas
    } else morphFrame.value = null
  })
  onScopeDispose(() => renderer.value?.dispose())
  function scrub(event: Event) {
    pose(Number((event.target as HTMLInputElement).value))
  }
  return {
    amount,
    playing,
    reducedMotion,
    duration,
    hold,
    pose,
    toggle,
    images,
    error,
    sourceId,
    targetId,
    source,
    target,
    pair,
    pairSafe,
    showBounds,
    mode,
    morphFrame,
    morphUnavailable,
    poseLabel,
    scrub,
  }
}
