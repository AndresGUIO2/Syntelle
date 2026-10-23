import { computed, onScopeDispose, shallowRef, watch, type Ref } from 'vue'
import { createEyeMorph } from '../rendering/eyeMorph'
import { createCharacterRenderer } from '../rendering/characterRenderer'
import { createBodyRenderer } from '../rendering/bodyRenderer'
import type { HeadPose } from '../animation/headMotion'
import { createCompletedMorph } from '../rendering/completedMorph'
import type { Rect } from '../types/oralTypes'
import type { OralContext } from '../types/packageContext'
import type { useOralAssets } from './useOralAssets'
import type { useMotionAssets } from './useMotionAssets'
import type { useMotionPlayback } from './useMotionPlayback'
import type { useVisemePlayback } from './useVisemePlayback'
import type { useSpeechPlayback } from './useSpeechPlayback'
import { createHeadDirector } from '../animation/headMotion'
import { createContextualHeadDirector } from '../animation/speechBehavior'
import type { IdleMotionProfile } from '../animation/idlePosture'
import { createAttentionDirector, gazeOffsets } from '../animation/attention'
import type { useExpressions } from './useExpressions'

export interface RenderScheduling {
  maxFramerate: number
  onFrame: () => void
  sourceClip?: Rect
}

export function useMotionRenderer(options: {
  context: OralContext
  oral: ReturnType<typeof useOralAssets>
  motion: ReturnType<typeof useMotionAssets>
  clock: ReturnType<typeof useMotionPlayback>
  mouth: ReturnType<typeof useVisemePlayback>
  sourceId: Ref<string>
  targetId: Ref<string>
  eyeView: Readonly<Ref<HTMLCanvasElement | null>>
  speech: ReturnType<typeof useSpeechPlayback>
  expressions: ReturnType<typeof useExpressions>
  idle?: IdleMotionProfile
  expressive?: boolean
  performance?: boolean
  conversationalAttention?: boolean
  scheduling?: RenderScheduling
  review?: {
    compare: Readonly<Ref<boolean>>
    pose: Readonly<Ref<HeadPose | null>>
    legacy?: boolean
  }
}) {
  const { context, oral, motion, clock, mouth, sourceId, targetId, eyeView, speech } = options
  const posePackage = context.package
  const definition = motion.definition
  const attentionDirector =
    motion.attention && definition?.projection
      ? createAttentionDirector(
          definition.animation_seed ?? parseInt(definition.build_id.slice(0, 8), 16),
          definition.projection.bounds,
          options.performance,
          options.conversationalAttention,
          options.expressive,
        )
      : null
  const director = definition?.rig
    ? createHeadDirector(
        definition.animation_seed ?? parseInt(definition.build_id.slice(0, 8), 16),
        definition.rig.bounds,
        definition.rig.posture,
      )
    : null
  const contextualDirector =
    definition?.rig && definition.behavior
      ? createContextualHeadDirector(
          definition.animation_seed ?? parseInt(definition.build_id.slice(0, 8), 16),
          definition.behavior,
          definition.rig.bounds,
        )
      : null
  const proposedDirector =
    definition?.projection && definition.behavior && definition.rig
      ? createContextualHeadDirector(
          definition.animation_seed ?? parseInt(definition.build_id.slice(0, 8), 16),
          {
            ...definition.behavior,
            head: {
              ...definition.behavior.head,
              idle_degrees: definition.projection.proposed_idle_degrees,
            },
          },
          definition.rig.bounds,
          options.idle,
        )
      : null
  const effectiveSource = computed(() =>
    speech.active.value ? speech.frame.value.from : sourceId.value,
  )
  const effectiveTarget = computed(() =>
    speech.active.value
      ? speech.frame.value.to === speech.frame.value.from
        ? speech.frame.value.from === 'Rest'
          ? 'A'
          : 'Rest'
        : speech.frame.value.to
      : targetId.value,
  )
  const { amount } = mouth
  const { paused, closure, movement } = clock
  const body = shallowRef<ReturnType<typeof createBodyRenderer> | null>(null)
  const comparison = shallowRef<{
    baseline: ReturnType<typeof createCharacterRenderer>
    matched: ReturnType<typeof createCharacterRenderer>
  } | null>(null)
  const eye = shallowRef<ReturnType<typeof createEyeMorph> | null>(null)
  const lips = shallowRef<ReturnType<typeof createCompletedMorph> | null>(null)
  const oralRevision = shallowRef(0)
  const failure = shallowRef('')
  const ready = computed(() => Boolean(body.value && eye.value && lips.value))
  const error = computed(() => failure.value || motion.error.value || oral.error.value)
  function unavailable() {
    failure.value =
      'Animation is unavailable in this browser. The preceding mouth trial is still available.'
    paused.value = true
  }
  const localPatch = new Map<string, { output: HTMLCanvasElement; base: HTMLCanvasElement }>()
  function upload(name: string, rect: Rect, frame: CanvasImageSource) {
    if (!body.value || !motion.images.value) return
    let patch = localPatch.get(name)
    if (!patch) {
      const base = document.createElement('canvas'),
        output = document.createElement('canvas')
      base.width = output.width = rect.width
      base.height = output.height = rect.height
      base.getContext('2d')!.drawImage(motion.images.value.foreground, -rect.x, -rect.y)
      patch = { base, output }
      localPatch.set(name, patch)
    }
    const canvas = patch.output
    const ctx = canvas.getContext('2d')!
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(patch.base, 0, 0)
    ctx.globalCompositeOperation = 'source-atop'
    ctx.drawImage(frame, 0, 0)
    ctx.globalCompositeOperation = 'source-over'
    body.value.update(rect, canvas)
    comparison.value?.baseline.update(rect, canvas)
    comparison.value?.matched.update(rect, canvas)
  }
  watch(motion.images, (images) => {
    localPatch.clear()
    body.value?.dispose()
    eye.value?.dispose()
    body.value = null
    eye.value = null
    if (!images || !definition) return
    try {
      body.value = options.review?.legacy
        ? createCharacterRenderer(images.foreground, images.depth, definition, unavailable)
        : createBodyRenderer(images, definition, unavailable, 'full', motion.expressions)
      if (options.scheduling?.sourceClip) body.value.setClip?.(options.scheduling.sourceClip)
      eye.value = createEyeMorph(
        images.opened,
        images.closed,
        images.mask,
        definition.eyes.curves,
        unavailable,
        motion.attention && images.gaze
          ? { package: motion.attention, images: images.gaze }
          : undefined,
        images.referenceLaughter ? motion.expressions?.reference_laughter : undefined,
        images.referenceLaughter?.frames,
      )
    } catch {
      body.value?.dispose()
      eye.value?.dispose()
      body.value = null
      eye.value = null
      unavailable()
    }
  })
  watch([motion.images, () => options.review?.compare.value ?? false], ([images, enabled]) => {
    comparison.value?.baseline.dispose()
    comparison.value?.matched.dispose()
    comparison.value = null
    if (!images || !enabled || !definition?.projection) return
    const baseline = definition.projection.articulation
      ? createBodyRenderer(images, definition, unavailable, 'previous')
      : createCharacterRenderer(images.foreground, images.depth, definition, unavailable)
    try {
      const matched = definition.projection.articulation
        ? createBodyRenderer(images, definition, unavailable, 'neck')
        : createCharacterRenderer(images.foreground, images.depth, definition, unavailable)
      comparison.value = { baseline, matched }
    } catch {
      baseline.dispose()
      unavailable()
    }
  })
  watch(
    [effectiveSource, effectiveTarget, oral.images],
    ([, , images], [, , previousImages]) => {
      if (!speech.active.value) mouth.pose(0)
      if (images !== previousImages) {
        lips.value?.dispose()
        lips.value = null
      }
      const pair = posePackage.pairs[`${effectiveSource.value}-${effectiveTarget.value}`]
      if (!images || !pair) return
      if (!pair.safe || !pair.mesh) {
        lips.value?.dispose()
        lips.value = null
        failure.value = pair.reason ?? 'This transition has invalid geometry.'
        return
      }
      try {
        if (lips.value)
          lips.value.setPair(
            images.poses[effectiveSource.value]!,
            images.poses[effectiveTarget.value]!,
            pair.mesh,
          )
        else
          lips.value = createCompletedMorph(
            images.poses[effectiveSource.value]!,
            images.poses[effectiveTarget.value]!,
            pair.mesh,
            images.upper,
            images.lower,
            unavailable,
            posePackage.oral,
            motion.expressions?.reference_laughter && motion.images.value?.referenceLaughter
              ? {
                  package: motion.expressions.reference_laughter,
                  images: motion.images.value.referenceLaughter,
                  oralCrop: posePackage.crop,
                  chinY: definition!.anatomy.chin_y,
                }
              : undefined,
          )
        if (!lips.value) unavailable()
        oralRevision.value++
      } catch {
        unavailable()
      }
    },
    { immediate: true },
  )
  let lastBody: unknown = null,
    lastComparison: unknown = null,
    lastLips: unknown = null,
    lastEye: unknown = null,
    lastMouth = -1,
    lastClosure = -1,
    lastGaze = '',
    lastExpression = '',
    lastEyeView: unknown = null,
    lastOralRevision = -1,
    lastMoving = true,
    lastLaugh = -1
  let paint = 0,
    nextPaint = 0
  const referenceCanvas = document.createElement('canvas')
  function drawFrame(now: number) {
    paint = 0
    const interval = options.scheduling ? 1000 / options.scheduling.maxFramerate : 0
    if (interval) {
      nextPaint ||= now
      if (now + 0.5 < nextPaint) {
        paint = requestAnimationFrame(drawFrame)
        return
      }
      nextPaint += interval
      if (nextPaint < now - interval) nextPaint = now + interval
    }
    const ms = clock.elapsed.value,
      c = closure.value,
      t = speech.active.value
        ? speech.frame.value.from === speech.frame.value.to
          ? 0
          : speech.frame.value.amount
        : amount.value,
      moving =
        movement.value ||
        (options.review?.pose.value !== null && options.review?.pose.value !== undefined),
      revision = oralRevision.value
    const characterRenderer = body.value,
      eyeRenderer = eye.value,
      mouthRenderer = lips.value,
      detail = eyeView.value
    if (!characterRenderer || !eyeRenderer || !mouthRenderer || !definition) return
    const state = {
      active: speech.active.value,
      position: speech.position.value,
      session: speech.session.value,
      plan: speech.behaviorPlan.value,
    }
    const attention = moving ? attentionDirector?.sample(ms, state) : undefined
    const expression = options.expressions.available
      ? options.expressions.sample(
          ms,
          { smile: options.expressions.smile.value, brows: options.expressions.brows.value },
          options.expressions.automatic.value,
          clock.reducedMotion.value,
          state,
          clock.paused.value,
          speech.loading.value ||
            speech.articulating.value ||
            (t < 1 && !['Rest', 'MBP'].includes(effectiveSource.value)) ||
            (t > 0 && !['Rest', 'MBP'].includes(effectiveTarget.value)),
        )
      : { smile: 0, brows: 0 }
    // Registered closed smiles never replace the cavity, tongue or shared teeth
    // during articulation. The request lead time settles the smile beforehand.
    if (
      speech.articulating.value ||
      (t < 1 && !['Rest', 'MBP'].includes(effectiveSource.value)) ||
      (t > 0 && !['Rest', 'MBP'].includes(effectiveTarget.value))
    )
      expression.smile = 0
    const reference = motion.expressions?.reference_laughter
    const referenceLaugh =
      reference && motion.images.value?.referenceLaughter && !clock.reducedMotion.value
        ? speech.laughAmount.value
        : 0
    if (referenceLaugh > 0) {
      const fade = Math.max(0, Math.min(1, referenceLaugh / 0.15))
      expression.smile = options.expressions.smile.value * (1 - fade * fade * (3 - 2 * fade))
    }
    const expressionKey = `${expression.smile},${expression.brows}`
    const expressionChanged = expressionKey !== lastExpression
    const offsets =
      attention && motion.attention
        ? gazeOffsets(motion.attention, attention.head, attention.glance)
        : ([
            [0, 0],
            [0, 0],
          ] as const)
    const quantized = offsets.map(
      (pair) => pair.map((v) => Math.trunc(v * 20) / 20) as [number, number],
    )
    const gazeKey = c === 1 ? 'closed' : quantized.flat().join(',')
    const replaced = lastBody !== characterRenderer || lastComparison !== comparison.value
    let changed = replaced
    let eyeChanged = replaced
    const laughFrames = motion.images.value?.laughter
    const laughing = Boolean(
      referenceLaugh > 0 ||
      (laughFrames &&
        motion.laughter &&
        !clock.reducedMotion.value &&
        speech.laughAmount.value > 0),
    )
    if (referenceLaugh > 0 && reference) {
      mouthRenderer.drawReference(referenceLaugh, speech.laughEnergy.value)
      eyeRenderer.drawReference(referenceLaugh, c, quantized)
      if (
        referenceCanvas.width !== reference.crop.width ||
        referenceCanvas.height !== reference.crop.height
      ) {
        referenceCanvas.width = reference.crop.width
        referenceCanvas.height = reference.crop.height
      }
      const ctx = referenceCanvas.getContext('2d')!
      ctx.clearRect(0, 0, referenceCanvas.width, referenceCanvas.height)
      ctx.drawImage(mouthRenderer.gpuCanvas, 0, 0)
      ctx.drawImage(
        eyeRenderer.canvas,
        reference.eyes.crop.x - reference.crop.x,
        reference.eyes.crop.y - reference.crop.y,
      )
      upload('reference-laughter', reference.crop, referenceCanvas)
      lastLaugh = referenceLaugh
      changed = eyeChanged = true
    } else if (laughing && laughFrames && motion.laughter) {
      const index = Math.round(speech.laughAmount.value * (laughFrames.length - 1))
      if (replaced || index !== lastLaugh) {
        upload('laughter', motion.laughter.crop, laughFrames[index]!)
        lastLaugh = index
        changed = true
      }
      expression.smile = expression.brows = 0
    } else if (lastLaugh >= 0) {
      if (reference && motion.images.value?.referenceLaughter)
        upload(
          'reference-laughter',
          reference.crop,
          motion.images.value.referenceLaughter.frames[0]!,
        )
      else if (laughFrames && motion.laughter)
        upload('laughter', motion.laughter.crop, laughFrames[0]!)
      lastLaugh = -1
      lastMouth = lastClosure = -1
      lastExpression = ''
      changed = true
    }
    if (
      !laughing &&
      (replaced || lastLips !== mouthRenderer || lastMouth !== t || lastOralRevision !== revision)
    ) {
      mouthRenderer.draw(t, false)
      upload('mouth', posePackage.crop, mouthRenderer.gpuCanvas)
      lastMouth = t
      lastLips = mouthRenderer
      lastOralRevision = revision
      changed = true
    }
    if (
      !laughing &&
      (replaced || lastEye !== eyeRenderer || lastClosure !== c || lastGaze !== gazeKey)
    ) {
      eyeRenderer.draw(c, quantized)
      upload('eyes', definition.eyes.crop, eyeRenderer.canvas)
      lastClosure = c
      lastEye = eyeRenderer
      lastGaze = gazeKey
      changed = true
      eyeChanged = true
    }
    if (detail && (eyeChanged || detail !== lastEyeView)) {
      if (detail.width !== eyeRenderer.canvas.width) {
        detail.width = eyeRenderer.canvas.width
        detail.height = eyeRenderer.canvas.height
      }
      detail.getContext('2d')!.drawImage(eyeRenderer.canvas, 0, 0)
      lastEyeView = detail
    }
    // Head motion changes vertex uniforms; gaze and blinks update only the eye
    // patch. Background blur never enters the character texture.
    if (changed || moving || lastMoving !== moving || expressionChanged) {
      characterRenderer.setExpression?.(expression)
      lastExpression = expressionKey
      const oldPose = contextualDirector?.sample(ms, state) ??
        director?.sample(ms, {
          active: speech.active.value,
          time: speech.position.value,
          beats: [...speech.beats.value],
        }) ?? { yaw: 0, pitch: 0, roll: 0 }
      const manual = options.review?.pose.value
      let proposed = manual ?? attention?.head ?? proposedDirector?.sample(ms, state) ?? oldPose
      if (definition.projection) {
        const limits = definition.projection.bounds
        proposed = {
          yaw: Math.max(-limits.yaw, Math.min(limits.yaw, proposed.yaw)),
          pitch: Math.max(-limits.pitch, Math.min(limits.pitch, proposed.pitch)),
          roll: Math.max(-limits.roll, Math.min(limits.roll, proposed.roll)),
        }
      }
      characterRenderer.draw(ms, moving, proposed)
      comparison.value?.matched.draw(ms, moving, proposed)
      const baseline =
        manual && definition.projection
          ? {
              yaw:
                (manual.yaw * (definition.behavior?.head.idle_degrees[0] ?? 0.22)) /
                definition.projection.bounds.yaw,
              pitch: manual.pitch * 0.1,
              roll: manual.roll,
            }
          : oldPose
      comparison.value?.baseline.draw(
        ms,
        moving,
        definition.projection?.articulation ? proposed : baseline,
      )
      // Private streaming publishes this exact completed draw, without another
      // animation loop or a stale canvas capture. Client-rendered views opt out.
      options.scheduling?.onFrame()
    }
    lastMoving = moving
    lastBody = characterRenderer
    lastComparison = comparison.value
  }
  // Mouth and motion have independent clocks. Coalesce their reactive updates so
  // the character is composed at most once per display frame.
  watch(
    [
      clock.elapsed,
      closure,
      amount,
      movement,
      body,
      eye,
      lips,
      eyeView,
      oralRevision,
      speech.frame,
      speech.active,
      speech.loading,
      speech.position,
      speech.laughAmount,
      speech.laughEnergy,
      comparison,
      options.expressions.smile,
      options.expressions.brows,
      options.expressions.automatic,
      clock.reducedMotion,
      () => options.review?.pose.value,
    ],
    () => {
      if (!paint) paint = requestAnimationFrame(drawFrame)
    },
    { immediate: true, flush: 'post' },
  )
  onScopeDispose(() => {
    cancelAnimationFrame(paint)
    body.value?.dispose()
    eye.value?.dispose()
    lips.value?.dispose()
    comparison.value?.baseline.dispose()
    comparison.value?.matched.dispose()
    localPatch.clear()
  })
  return { body, comparison, ready, error }
}
