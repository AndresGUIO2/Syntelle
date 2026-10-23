import { computed, shallowRef, watch, type Ref } from 'vue'
import type { HeadPose } from '../animation/headMotion'
import { projectionReviewDuration, projectionReviewPose } from '../animation/projectionReview'

export function useProjectionReview(elapsed: Readonly<Ref<number>>, pose: Ref<HeadPose | null>) {
  const yaw = shallowRef(0),
    pitch = shallowRef(0),
    manual = shallowRef(false),
    started = shallowRef<number | null>(null)
  const playing = computed(() => started.value !== null)
  watch(
    [elapsed, yaw, pitch, manual, started],
    () => {
      if (started.value !== null) {
        const time = elapsed.value - started.value
        pose.value = projectionReviewPose(time)
        if (time >= projectionReviewDuration) {
          started.value = null
          manual.value = true
          yaw.value = 0
          pitch.value = 0
        }
      } else pose.value = manual.value ? { yaw: yaw.value, pitch: pitch.value, roll: 0 } : null
    },
    { immediate: true, flush: 'sync' },
  )
  function inspect(value: number, axis: 'yaw' | 'pitch') {
    started.value = null
    manual.value = true
    if (axis === 'yaw') yaw.value = value
    else pitch.value = value
  }
  function replay() {
    manual.value = false
    started.value = elapsed.value
  }
  function automatic() {
    started.value = null
    manual.value = false
  }
  return { yaw, pitch, manual, playing, inspect, replay, automatic }
}
