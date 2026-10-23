import { expect, test } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'

test('conversational attention breaks fixation without clip resets or unsafe eye motion', async ({
  page,
}, info) => {
  await page.route('**/api/**', (r) => r.fulfill({ json: { configured: false } }))
  await page.goto('/?view=validation')
  const results = await page.evaluate(async () => {
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const { createAttentionDirector, gazeOffsets } = await import(
      '/src/features/character/animation/attention.ts' as string
    )
    const { createSpeakingAttention } = await import(
      '/src/features/character/animation/speakingAttention.ts' as string
    )
    const rows = []
    for (const id of ['syntelle', 'cafe', 'woodland']) {
      const context = await loadDemoCharacter(id),
        definition = context.motion.definition,
        gaze = context.motion.attention,
        seed = definition.animation_seed,
        bounds = definition.projection.bounds,
        director = createAttentionDirector(seed, bounds, true, true),
        uninterrupted = createAttentionDirector(seed, bounds, true, true)
      let previous = { yaw: 0, pitch: 0, roll: 0 },
        maxVelocity = 0,
        away = 0,
        longestContact = 0,
        contact = 0,
        excursions = 0,
        wasAway = false,
        left = false,
        right = false
      let pausedFrame
      for (let ms = 0; ms < 60000; ms += 20) {
        const speech = {
          active: true,
          position: (ms % 3000) / 1000,
          session: Math.floor(ms / 3000),
          plan: null,
        }
        const frame = director.sample(ms, speech)
        const continuous = uninterrupted.sample(ms, { ...speech, session: 1, position: ms / 1000 })
        if (JSON.stringify(frame) !== JSON.stringify(continuous))
          throw Error('Clip reset attention')
        if (ms === 10000) {
          pausedFrame = frame
          if (JSON.stringify(frame) !== JSON.stringify(director.sample(ms, speech)))
            throw Error('Paused animation changed')
        }
        const lookingAway = Math.abs(frame.glance[0]) > 0.3
        if (lookingAway) {
          away += 20
          if (!wasAway) excursions++
          contact = 0
          left ||= frame.glance[0] < 0
          right ||= frame.glance[0] > 0
        } else {
          contact += 20
          longestContact = Math.max(longestContact, contact)
        }
        wasAway = lookingAway
        for (const axis of ['yaw', 'pitch', 'roll'] as const) {
          if (Math.abs(frame.head[axis]) > bounds[axis] + 1e-8)
            throw Error('Head envelope exceeded')
          maxVelocity = Math.max(maxVelocity, Math.abs(frame.head[axis] - previous[axis]) / 0.02)
        }
        const offsets = gazeOffsets(gaze, frame.head, frame.glance)
        for (let i = 0; i < 2; i++)
          for (let axis = 0; axis < 2; axis++) {
            const v = offsets[i][axis]
            if (Math.abs(v) > gaze.eyes[i].bounds_px[axis][v < 0 ? 0 : 1] + 1e-8)
              throw Error('Eye left observed source support')
            if (v * offsets[1 - i][axis] < 0) throw Error('Crossed eyes')
          }
        previous = frame.head
      }
      const idle = { head: { yaw: 1, pitch: 0.4, roll: 0.1 }, glance: [0, 0] }
      const bridged = createSpeakingAttention(seed),
        unbroken = createSpeakingAttention(seed)
      let maximumBridgeStep = 0,
        previousGlance = [0, 0]
      for (let ms = 0; ms <= 7000; ms += 20) {
        const frame = bridged.sample(ms, ms < 3000 || ms >= 3060, idle)
        const other = unbroken.sample(ms, true, idle)
        maximumBridgeStep = Math.max(
          maximumBridgeStep,
          Math.abs(frame.glance[0] - previousGlance[0]),
        )
        if (ms > 4000 && JSON.stringify(frame) !== JSON.stringify(other))
          throw Error('Brief phrase gap restarted glance schedule')
        previousGlance = frame.glance
      }
      const stopped = bridged.sample(7020, false, idle)
      const settled = bridged.sample(8500, false, idle)
      if (JSON.stringify(settled) !== JSON.stringify(idle)) throw Error('Stop failed to settle')
      rows.push({
        id,
        enabled: context.motion.conversationalAttention,
        excursions,
        awayFraction: away / 60000,
        longestContact,
        left,
        right,
        maxVelocity,
        maximumBridgeStep,
        pausedFrame,
        stopped,
      })
    }
    return rows
  })
  for (const row of results) {
    expect(row.enabled).toBe(true)
    expect(row.excursions).toBeGreaterThanOrEqual(7)
    expect(row.excursions).toBeLessThanOrEqual(14)
    expect(row.awayFraction).toBeGreaterThan(0.17)
    expect(row.awayFraction).toBeLessThan(0.4)
    expect(row.longestContact).toBeLessThan(6500)
    expect(row.left && row.right).toBe(true)
    expect(row.maxVelocity).toBeLessThan(4)
    expect(row.maximumBridgeStep).toBeLessThan(0.3)
  }
  await info.attach('speaking-attention-trajectories', {
    body: JSON.stringify(results, null, 2),
    contentType: 'application/json',
  })
})

test('cached progressive conversation keeps eyes and posture alive while speaking', async ({
  page,
}, info) => {
  test.setTimeout(30000)
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit has no Web Audio')
  const speech = JSON.parse(
    await readFile(
      new URL('../../../../.tools/head-motion-12/speech/ava-en/response.json', import.meta.url),
      'utf8',
    ),
  )
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.route('**/api/**', (r) => {
    if (r.request().method() !== 'GET') throw Error('Offline review must not call a provider')
    return r.fulfill({ json: { configured: true, recognition: true } })
  })
  let publish: (value: any) => void = () => {}
  await page.routeWebSocket('**/api/v1/conversation/ws', (ws) => {
    ws.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      if (event.type === 'hello') {
        publish = (value) =>
          ws.send(JSON.stringify({ conversation_id: event.conversation_id, ...value }))
        publish({ type: 'ready', data: [] })
      }
    })
  })
  await page.addInitScript(() => {
    const locations = new WeakMap<WebGLUniformLocation, string>()
    const getLocation = WebGLRenderingContext.prototype.getUniformLocation
    WebGLRenderingContext.prototype.getUniformLocation = function (program, name) {
      const location = getLocation.call(this, program, name)
      if (location) locations.set(location, name)
      return location
    }
    const uniform = WebGLRenderingContext.prototype.uniform4f
    ;(window as any).eyeUpdates = []
    WebGLRenderingContext.prototype.uniform4f = function (location, x, y, z, w) {
      if (location && locations.get(location) === 'offsets')
        (window as any).eyeUpdates.push([performance.now(), x, y, z, w])
      return uniform.call(this, location, x, y, z, w)
    }
  })
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('heading', { name: 'Chat with Syntelle' }).click()
  const turn_id = 'offline-attention-review',
    text = 'An offline continuous speaking review.'
  publish({ type: 'transcript_final', turn_id, text: 'Replay cached speech.' })
  publish({ type: 'accepted', turn_id, text: 'Replay cached speech.' })
  publish({ type: 'reply_complete', turn_id, text, data: { spoken: text } })
  for (let index = 0; index < 2; index++)
    publish({
      type: 'speech_part',
      turn_id,
      data: { speech, text, index, final: index === 1, offset_ms: index * speech.duration_ms },
    })
  publish({ type: 'speech_complete', turn_id })
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  const began = await page.evaluate(() => performance.now())
  const heads: number[][] = []
  for (let i = 0; i < 12; i++) {
    await page.waitForTimeout(450)
    heads.push(
      await page
        .getByRole('img', { name: 'Animated character', exact: true })
        .evaluate((canvas: HTMLCanvasElement) => {
          const gl = canvas.getContext('webgl')!,
            program = gl.getParameter(gl.CURRENT_PROGRAM)
          return Array.from(gl.getUniform(program, gl.getUniformLocation(program, 'angles')!))
        }),
    )
    if (i === 3 || i === 6) await page.screenshot({ path: info.outputPath(`speaking-${i}.png`) })
  }
  const eyeUpdates = await page.evaluate(
    (after) => (window as any).eyeUpdates.filter((row: number[]) => row[0]! >= after),
    began,
  )
  expect(new Set(eyeUpdates.map((row: number[]) => row.slice(1).join(','))).size).toBeGreaterThan(8)
  expect(Math.max(...eyeUpdates.map((row: number[]) => Math.abs(row[1]!)))).toBeGreaterThan(2)
  expect(Math.max(...heads.map((p) => Math.abs(p[0]!)))).toBeGreaterThan(0.002)
  expect(heads.every((p) => Math.abs(p[0]!) < (3 * Math.PI) / 180)).toBe(true)
  await page.getByRole('button', { name: 'Stop reply' }).click()
  await expect(page.getByText('Speaking', { exact: true })).toHaveCount(0)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.waitForTimeout(100)
  const reduced = await page
    .getByRole('img', { name: 'Animated character', exact: true })
    .evaluate((canvas: HTMLCanvasElement) => {
      const gl = canvas.getContext('webgl')!,
        program = gl.getParameter(gl.CURRENT_PROGRAM)
      return Array.from(gl.getUniform(program, gl.getUniformLocation(program, 'angles')!))
    })
  expect(reduced).toEqual([0, 0, 0])
  expect(errors).toEqual([])
  await info.attach('actual-cached-progressive-attention', {
    body: JSON.stringify({ heads, eyeUpdates, provider_calls: 0 }),
    contentType: 'application/json',
  })
})

test('previous attention keeps neutral cached speech centered without a completion smile', async ({
  page,
}, info) => {
  const speech = JSON.parse(
    await readFile(
      new URL('../../../../.tools/head-motion-12/speech/ava-en/response.json', import.meta.url),
      'utf8',
    ),
  )
  const text = 'An offline neutral response.'
  speech.performance = {
    intent: { version: 'performance-intent/1', text, tone: 'neutral', cues: [] },
    resolved: [],
    omitted: [],
  }
  await page.route('**/api/**', (r) => r.fulfill({ json: { configured: true, recognition: true } }))
  await page.routeWebSocket('**/api/v1/conversation/ws', (ws) =>
    ws.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      const send = (type: string, text = '', data?: any) =>
        ws.send(
          JSON.stringify({
            type,
            text,
            data,
            turn_id: event.turn_id,
            conversation_id: event.conversation_id,
          }),
        )
      if (event.type === 'hello') send('ready', '', [])
      if (event.type === 'mic_start') {
        send('transcript_final', 'Replay the local neutral fixture')
        send('accepted', 'Replay the local neutral fixture')
        send('reply_delta', text)
        send('reply_complete', text, { spoken: text })
        send('prepared_speech', text, speech)
      }
    }),
  )
  await page.addInitScript(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      const audio = new AudioContext()
      const output = audio.createMediaStreamDestination()
      return output.stream
    }
  })
  await page.goto('/?attention=previous')
  test.skip(
    !(await page.evaluate(() => typeof AudioContext !== 'undefined')),
    'Actual audio playback is covered by Linux WebKit.',
  )
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('button', { name: 'Enable microphone' }).click()
  await expect(page.getByRole('button', { name: 'Stop reply' })).toBeVisible()
  await page.waitForTimeout(1200)
  const uniforms = () =>
    page
      .getByRole('img', { name: 'Animated character', exact: true })
      .evaluate((canvas: HTMLCanvasElement) => {
        const gl = canvas.getContext('webgl')!,
          program = gl.getParameter(gl.CURRENT_PROGRAM) as WebGLProgram
        return {
          head: Array.from(
            gl.getUniform(program, gl.getUniformLocation(program, 'angles')!) as Float32Array,
          ),
          expression: Array.from(
            gl.getUniform(
              program,
              gl.getUniformLocation(program, 'expressionAmount')!,
            ) as Float32Array,
          ),
        }
      })
  const speaking = await uniforms()
  expect(speaking.head.every((v) => Math.abs(v) < 0.0001)).toBe(true)
  expect(speaking.expression).toEqual([0, 0])
  await page.screenshot({ path: info.outputPath('stable-speaking.png') })
  await expect(page.getByRole('button', { name: 'Stop reply' })).toHaveCount(0, { timeout: 12000 })
  await page.waitForTimeout(450)
  expect((await uniforms()).expression[0]).toBe(0)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.waitForTimeout(100)
  expect((await uniforms()).head).toEqual([0, 0, 0])
})

test('direction limits, semantic anchors, stable speech and explicit expressions', async ({
  page,
}) => {
  await page.route('**/api/**', (r) => r.fulfill({ json: { configured: false } }))
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const { admitPerformance, directedPlan } = await import(
      '/src/features/character/animation/performance.ts' as string
    )
    const { createAttentionDirector } = await import(
      '/src/features/character/animation/attention.ts' as string
    )
    const { createExpressionDirector } = await import(
      '/src/features/character/animation/expressions.ts' as string
    )
    const { gestureAt } = await import(
      '/src/features/character/animation/speechBehavior.ts' as string
    )
    const text = 'Yes, that is correct. Another point follows. That joke worked.'
    const value = {
      intent: {
        version: 'performance-intent/1',
        text,
        tone: 'playful',
        cues: [
          { kind: 'nod', anchor: 'correct' },
          { kind: 'shake', anchor: 'Another' },
          { kind: 'emphasis', anchor: 'point' },
          { kind: 'smile', anchor: 'joke' },
          { kind: 'laugh', anchor: 'worked' },
        ],
      },
      omitted: [],
      resolved: [
        { kind: 'nod', time: 0.5, duration: 0.85 },
        { kind: 'shake', time: 1, duration: 1.2 },
        { kind: 'emphasis', time: 3.2, duration: 0.85 },
        { kind: 'smile', time: 8.08, duration: 1.7 },
        { kind: 'laugh', time: 8.12, duration: 1.65 },
      ],
    }
    const first = admitPerformance(value, text, 0, false)
    const second = admitPerformance(value, text, 1000, false)
    const laugh1 = admitPerformance(value, text, 65000, true)
    const laugh2 = admitPerformance(value, text, 130000, true)
    admitPerformance(undefined, text, 200000, true)
    const laugh3 = admitPerformance(value, text, 201000, true)
    const malformed = admitPerformance(
      { ...value, resolved: [{ kind: 'nod', time: NaN, duration: 1 }] },
      text,
      250000,
    )
    const empty = directedPlan(new Float32Array(8000), 1000, undefined)
    const director = createAttentionDirector(42, { yaw: 3, pitch: 1.2, roll: 0.3 }, true)
    const old = createAttentionDirector(42, { yaw: 3, pitch: 1.2, roll: 0.3 })
    const state = { active: true, position: 2, session: 1, plan: empty }
    director.sample(1000, state)
    old.sample(1000, state)
    const stable = director.sample(2500, state),
      wandering = old.sample(2500, state)
    const expression = createExpressionDirector(42, true, true)
    expression.sample(1000, { smile: 0, brows: 0 }, true, false, state)
    const ended = expression.sample(1350, { smile: 0, brows: 0 }, true, false, {
      ...state,
      active: false,
      position: 8,
    })
    const reduced = expression.sample(1500, { smile: 0, brows: 0 }, true, true, state)
    const shake = {
      ...empty,
      gestures: [{ kind: 'shake', time: 0, duration: 1.25, pitch: 0, yaw: 1.5 }],
    }
    const poses = Array.from({ length: 51 }, (_, i) => gestureAt(shake, (i * 1.25) / 50))
    return {
      first: first.resolved,
      second: second.resolved,
      laugh1: laugh1.resolved,
      laugh2: laugh2.resolved,
      laugh3: laugh3.resolved,
      malformed: malformed.resolved,
      stable,
      wandering,
      ended,
      reduced,
      poses,
    }
  })
  expect(result.first.map((c: any) => c.kind)).toEqual(['nod', 'emphasis', 'smile'])
  expect(result.second.some((c: any) => c.kind === 'smile')).toBe(false)
  expect(result.laugh1.some((c: any) => c.kind === 'laugh')).toBe(true)
  expect(result.laugh1.some((c: any) => c.kind === 'smile')).toBe(false)
  expect(result.laugh2.some((c: any) => c.kind === 'laugh')).toBe(false)
  expect(result.laugh3.some((c: any) => c.kind === 'laugh')).toBe(true)
  expect(result.malformed).toEqual([])
  expect(result.stable.head).toEqual({ yaw: 0, pitch: 0, roll: 0 })
  expect(Math.abs(result.wandering.head.yaw)).toBeGreaterThan(0.2)
  expect(result.ended).toEqual({ smile: 0, brows: 0 })
  expect(result.reduced).toEqual({ smile: 0, brows: 0 })
  expect(Math.min(...result.poses.map((p: any) => p.yaw))).toBeLessThan(-0.2)
  expect(Math.max(...result.poses.map((p: any) => p.yaw))).toBeGreaterThan(0.2)
  expect(result.poses.at(-1)).toEqual({ yaw: 0, pitch: 0, roll: 0 })
})

test('cached laughter sequence follows pause/resume, interruption and consecutive-turn policy', async ({
  page,
}, info) => {
  const speech = JSON.parse(
    await readFile(
      new URL('../../../../.tools/head-motion-12/speech/ava-en/response.json', import.meta.url),
      'utf8',
    ),
  )
  const laugh = await readFile(
    new URL('../../../../.tools/character-performance-34/tts-en.wav', import.meta.url),
  )
  await page.route('**/api/**', (r) => r.fulfill({ json: { configured: false } }))
  await page.route('**/fixture-laugh.wav', (r) =>
    r.fulfill({ body: laugh, contentType: 'audio/wav' }),
  )
  await page.goto('/?view=validation')
  test.skip(
    !(await page.evaluate(() => typeof AudioContext !== 'undefined')),
    'Windows WebKit has no Web Audio; run this case in Linux WebKit.',
  )
  const text = 'That joke worked.'
  const duration = speech.duration_ms / 1000
  speech.region = 'canadacentral'
  speech.performance = {
    intent: {
      version: 'performance-intent/1',
      text,
      tone: 'playful',
      cues: [{ kind: 'laugh', anchor: 'joke' }],
    },
    omitted: [],
    resolved: [{ kind: 'laugh', time: duration + 0.12, duration: 1.65 }],
  }
  await page.evaluate(
    async ({ speech, text, hash }) => {
      const { mountPerformanceFixture } = await import(
        '/tests/fixtures/performanceHarness.ts' as string
      )
      mountPerformanceFixture(speech, text, hash)
    },
    { speech, text, hash: createHash('sha256').update(laugh).digest('hex') },
  )
  await page.getByRole('button', { name: 'Play offline sequence' }).click()
  await expect
    .poll(
      () => page.evaluate(() => (window as any).performanceFixture.playback.laughAmount.value),
      { timeout: 18000 },
    )
    .toBeGreaterThan(0.2)
  await page.evaluate(() => (window as any).performanceFixture.playback.pause())
  const paused = await page.evaluate(() => {
    const p = (window as any).performanceFixture.playback
    return { time: p.position.value, amount: p.laughAmount.value }
  })
  await page.waitForTimeout(200)
  expect(
    await page.evaluate(() => {
      const p = (window as any).performanceFixture.playback
      return { time: p.position.value, amount: p.laughAmount.value }
    }),
  ).toEqual(paused)
  await page.evaluate(() => (window as any).performanceFixture.playback.resume())
  await expect
    .poll(() => page.evaluate(() => (window as any).performanceFixture.playback.position.value))
    .toBeGreaterThan(paused.time + 0.05)
  await page.evaluate(() => (window as any).performanceFixture.playback.stop())
  expect(
    await page.evaluate(() => {
      const p = (window as any).performanceFixture.playback
      return { active: p.active.value, amount: p.laughAmount.value, plan: p.behaviorPlan.value }
    }),
  ).toEqual({ active: false, amount: 0, plan: null })
  await page.getByRole('button', { name: 'Play offline sequence' }).click()
  await expect
    .poll(() => page.evaluate(() => (window as any).performanceFixture.playback.active.value))
    .toBe(true)
  expect(
    await page.evaluate(
      () => (window as any).performanceFixture.playback.behaviorPlan.value.performance.resolved,
    ),
  ).toEqual([])
  await page.evaluate(() => (window as any).performanceFixture.playback.stop())
  await page.screenshot({ path: info.outputPath('offline-sequence.png') })
})
