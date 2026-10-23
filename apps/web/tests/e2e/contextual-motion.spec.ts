import { test, expect } from '@playwright/test'

test('acoustic cues distinguish long pauses from consonant gaps and produce sparse gestures', async ({
  page,
}) => {
  await page.goto('/?trial=motion&character=syntelle&head=contextual-candidate')
  const result = await page.evaluate(async () => {
    const { acousticCues, planSpeechBehavior } = await import(
      '/src/features/character/animation/speechBehavior.ts' as string
    )
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const settings = resolveMotionContext('syntelle', true, 'contextual').definition.behavior
    const rate = 1000,
      pcm = new Float32Array(rate * 12)
    for (let i = 0; i < pcm.length; i++) {
      const t = i / rate,
        voiced =
          (t > 0.2 && t < 2.2) || (t > 2.8 && t < 5) || (t > 5.6 && t < 8) || (t > 8.6 && t < 11.6)
      const consonant = t > 1 && t < 1.08
      pcm[i] = voiced && !consonant ? Math.sin(i * 0.9) * (0.12 + 0.08 * Math.sin(t * 5) ** 2) : 0
    }
    const a = planSpeechBehavior(pcm, rate, 42, settings),
      b = planSpeechBehavior(pcm, rate, 42, settings)
    return {
      cues: acousticCues(pcm, rate),
      plan: a,
      deterministic: JSON.stringify(a) === JSON.stringify(b),
      other: planSpeechBehavior(pcm, rate, 591, settings),
      silence: acousticCues(new Float32Array(12000), rate),
      gap: settings.head.minimum_gesture_gap_seconds,
    }
  })
  expect(result.silence).toEqual([])
  expect(result.cues.filter((c: any) => c.kind === 'pause')).toHaveLength(3)
  expect(result.cues.some((c: any) => c.kind === 'pause' && c.time < 2)).toBe(false)
  expect(result.cues.find((c: any) => c.kind === 'onset')!.time).toBeCloseTo(0.2, 1)
  expect(result.cues.find((c: any) => c.kind === 'ending')!.time).toBeGreaterThan(11.3)
  expect(result.deterministic).toBe(true)
  expect(result.plan.gestures.length).toBeGreaterThan(0)
  expect(result.plan.gestures.length).toBeLessThan(5)
  expect(result.plan.gestures).not.toEqual(result.other.gestures)
  for (let i = 1; i < result.plan.gestures.length; i++)
    expect(
      result.plan.gestures[i]!.time - result.plan.gestures[i - 1]!.time,
    ).toBeGreaterThanOrEqual(result.gap - 1e-6)
})

test('quiet idle has no nod routine; speech nods and interruption stay continuous in all rigs', async ({
  page,
}) => {
  await page.goto('/?trial=motion&character=syntelle&head=contextual-candidate')
  const results = await page.evaluate(async () => {
    const { createContextualHeadDirector, quietPosture } = await import(
      '/src/features/character/animation/speechBehavior.ts' as string
    )
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    return ['syntelle', 'cafe', 'woodland'].map((name) => {
      const m = resolveMotionContext(name, true, 'contextual').definition
      const director = createContextualHeadDirector(42, m.behavior, m.rig.bounds)
      const plan = {
        version: 'speech-context/1',
        duration: 5,
        cues: [],
        gestures: [{ time: 1, duration: 1.2, pitch: -1.1, yaw: 0, kind: 'pause' }],
      }
      let previous = director.sample(0, { active: false, position: 0, session: 0, plan: null })
      let step = 0,
        lowestPitch = 0,
        idlePitch = 0,
        maxYaw = 0
      for (let ms = 16; ms < 15000; ms += 16) {
        const active = ms >= 4000 && ms < 5440
        const value = director.sample(ms, {
          active,
          position: active ? (ms - 4000) / 1000 : 0,
          session: active ? 1 : ms < 4000 ? 0 : 2,
          plan: active ? plan : null,
        })
        for (const key of ['yaw', 'pitch', 'roll'])
          step = Math.max(step, Math.abs(value[key] - previous[key]))
        if (active) lowestPitch = Math.min(lowestPitch, value.pitch)
        if (ms < 4000 || ms > 6200) idlePitch = Math.max(idlePitch, Math.abs(value.pitch))
        maxYaw = Math.max(maxYaw, Math.abs(value.yaw))
        previous = value
      }
      const quiet = []
      for (let ms = 0; ms < 60000; ms += 100)
        quiet.push(quietPosture(ms, 42, m.behavior.head.idle_degrees))
      return {
        name,
        step,
        lowestPitch,
        idlePitch,
        maxYaw,
        quietMax: Math.max(...quiet.map((p) => Math.abs(p.pitch))),
      }
    })
  })
  for (const value of results) {
    expect(value.lowestPitch, value.name).toBeLessThan(-0.7)
    expect(value.step, value.name).toBeLessThan(0.07)
    expect(value.idlePitch).toBeLessThanOrEqual(0.100001)
    expect(value.maxYaw).toBeLessThanOrEqual(0.220001)
    expect(value.quietMax).toBeLessThanOrEqual(0.100001)
  }
})

test('contextual blinks share a frequency budget, do not replay stale cues and remain irregular', async ({
  page,
}) => {
  await page.goto('/?trial=motion&character=syntelle&head=contextual-candidate')
  const result = await page.evaluate(async () => {
    const { createContextualBlinkPlanner } = await import(
      '/src/features/character/animation/speechBehavior.ts' as string
    )
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const settings = resolveMotionContext('syntelle', true, 'contextual').definition.behavior.blink
    function run(contextual: boolean) {
      const planner = createContextualBlinkPlanner(
          42,
          contextual ? { ...settings, pause_probability: 1 } : settings,
        ),
        times: number[] = []
      const plan = {
        version: 'speech-context/1',
        duration: 10,
        gestures: [],
        cues: Array.from({ length: 50 }, (_, i) => ({ time: i * 0.2, kind: 'pause', strength: 1 })),
      }
      for (let ms = 0; ms < 600000; ms += 16) {
        if (
          planner.sample(ms, {
            active: contextual,
            position: (ms % 10000) / 1000,
            session: Math.floor(ms / 10000),
            plan,
          })
        )
          times.push(ms)
      }
      return times
    }
    const idle = run(false),
      contextual = run(true),
      replay = run(true)
    const suppressed = createContextualBlinkPlanner(42, { ...settings, pause_probability: 1 })
    const plan = {
      version: 'speech-context/1',
      duration: 10,
      gestures: [],
      cues: [{ time: 1, kind: 'pause', strength: 1 }],
    }
    suppressed.sample(1000, { active: true, position: 1, session: 1, plan }, false)
    const stale = suppressed.sample(1016, { active: true, position: 1.016, session: 1, plan })
    const intervals = idle.slice(1).map((t, i) => t - idle[i])
    return {
      idle: idle.length,
      contextual: contextual.length,
      deterministic: JSON.stringify(contextual) === JSON.stringify(replay),
      minGap: Math.min(...contextual.slice(1).map((t, i) => t - contextual[i])),
      allowance: settings.rate_per_minute * 10 + 2,
      intervalRange: Math.max(...intervals) - Math.min(...intervals),
      stale,
    }
  })
  expect(result.deterministic).toBe(true)
  expect(result.contextual).toBeLessThanOrEqual(result.allowance)
  expect(result.minGap).toBeGreaterThanOrEqual(800)
  expect(result.idle).toBeGreaterThan(90)
  expect(result.intervalRange).toBeGreaterThan(2500)
  expect(result.stale).toBe(false)
})

test('contextual behavior is optional and preserves the previous head-turn comparison', async ({
  page,
}) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto('/?trial=motion&character=syntelle&head=contextual-candidate')
  await expect(page.getByRole('button', { name: 'Blink once', exact: true })).toBeEnabled()
  await expect(
    page.getByRole('link', { name: 'Compare previous head turns', exact: true }),
  ).toHaveAttribute('href', '?trial=motion&character=syntelle&head=rotation-candidate')
  await page.getByRole('link', { name: 'Compare previous head turns', exact: true }).click()
  await expect(
    page.getByRole('link', { name: 'Try contextual movement', exact: true }),
  ).toBeVisible()
  expect(errors).toEqual([])
})
