import { shallowRef, watch, type Ref } from 'vue'
import type { OralContext, MotionContext } from '../types/packageContext'
import { useOralAssets } from './useOralAssets'
import { useMotionAssets } from './useMotionAssets'
import { useMotionPlayback } from './useMotionPlayback'
import { useVisemePlayback } from './useVisemePlayback'
import { useMotionRenderer, type RenderScheduling } from './useMotionRenderer'
import { useSpeechPlayback, type SpeechOutput } from './useSpeechPlayback'
import type { HeadPose } from '../animation/headMotion'
import { useExpressions } from './useExpressions'

export function useCharacterMotion(
  context: OralContext,
  movementContext: MotionContext,
  eyeView: Readonly<Ref<HTMLCanvasElement | null>>,
  review?: {
    compare: Readonly<Ref<boolean>>
    pose: Readonly<Ref<HeadPose | null>>
    legacy?: boolean
  },
  output?: SpeechOutput,
  scheduling?: RenderScheduling,
) {
  const oral = useOralAssets(context)
  const motion = useMotionAssets(movementContext, context.package.build_id)
  const speech = useSpeechPlayback(
    context.character ?? 'syntelle',
    motion.definition?.behavior,
    movementContext.expressions?.smile ? 240 : 0,
    movementContext.performance,
    () =>
      movementContext.laughter &&
      (motion.images.value?.laughter || motion.images.value?.referenceLaughter)
        ? { package: movementContext.laughter, assetUrl: movementContext.assetUrl }
        : undefined,
    output,
    Boolean(movementContext.expressions?.reference_laughter),
  )
  const expressions = useExpressions(
    Boolean(movementContext.expressions),
    motion.definition?.animation_seed ?? 42,
    Boolean(movementContext.expressions?.smile),
    movementContext.performance,
    movementContext.expressive,
  )
  const clock = useMotionPlayback(
    motion.definition?.blink ?? {
      closing_ms: 80,
      closed_ms: 40,
      opening_ms: 140,
      minimum_interval_ms: 4000,
      maximum_interval_ms: 7000,
    },
    parseInt(context.package.build_id.slice(0, 8), 16),
    motion.definition?.behavior
      ? {
          behavior: motion.definition.behavior,
          speech: () => ({
            active: speech.active.value,
            position: speech.position.value,
            session: speech.session.value,
            plan: speech.behaviorPlan.value,
          }),
        }
      : undefined,
  )
  const mouth = useVisemePlayback()
  watch(clock.paused, (value) => {
    if (value) {
      mouth.pose(mouth.amount.value)
      speech.pause()
    } else if (speech.active.value && speech.paused.value) void speech.resume()
  })
  watch(speech.paused, (value) => {
    if (speech.active.value && !clock.reducedMotion.value) clock.paused.value = value
  })
  watch(speech.active, (value) => {
    if (value) {
      mouth.pose(mouth.amount.value)
      if (!clock.reducedMotion.value) clock.paused.value = false
    }
  })
  const sourceId = shallowRef('Rest'),
    targetId = shallowRef('A')
  const rendering = useMotionRenderer({
    context,
    oral,
    motion,
    clock,
    mouth,
    sourceId,
    targetId,
    eyeView,
    speech,
    expressions,
    idle: movementContext.idle,
    expressive: movementContext.expressive,
    performance: movementContext.performance,
    conversationalAttention: movementContext.conversationalAttention,
    scheduling,
    review,
  })
  return {
    clock,
    mouth,
    speech,
    expressions,
    sourceId,
    targetId,
    definition: motion.definition,
    ...rendering,
  }
}
