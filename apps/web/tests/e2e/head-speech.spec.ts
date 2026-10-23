import { expect, test } from '@playwright/test'

function tone() {
  const rate = 24000,
    count = rate * 3,
    out = Buffer.alloc(44 + count * 2)
  out.write('RIFF', 0)
  out.writeUInt32LE(out.length - 8, 4)
  out.write('WAVEfmt ', 8)
  out.writeUInt32LE(16, 16)
  out.writeUInt16LE(1, 20)
  out.writeUInt16LE(1, 22)
  out.writeUInt32LE(rate, 24)
  out.writeUInt32LE(rate * 2, 28)
  out.writeUInt16LE(2, 32)
  out.writeUInt16LE(16, 34)
  out.write('data', 36)
  out.writeUInt32LE(count * 2, 40)
  for (let i = 0; i < count; i++) {
    const t = i / rate,
      pulse = Math.max(0, Math.sin(t * Math.PI * 2 * 1.2))
    out.writeInt16LE(Math.round(Math.sin(t * 220 * Math.PI * 2) * pulse * 4000), 44 + i * 2)
  }
  return out.toString('base64')
}

for (const character of ['syntelle', 'cafe', 'woodland']) {
  test(`${character}: natural head and synthetic audio-clock playback`, async ({ page }, info) => {
    let requests = 0
    await page.route('**/api/v1/speech/status', (route) =>
      route.fulfill({ json: { configured: true } }),
    )
    await page.route('**/api/v1/speech/synthesize', (route) => {
      requests++
      return route.fulfill({
        json: {
          audio_base64: tone(),
          mime: 'audio/wav',
          duration_ms: 3000,
          voice: 'en-US-Ava:DragonHDLatestNeural',
          language: 'en-US',
          sdk_version: 'fixture',
          cached: false,
          visemes: [
            { id: 0, offset_ms: 0 },
            { id: 2, offset_ms: 100 },
            { id: 6, offset_ms: 450 },
            { id: 21, offset_ms: 800 },
            { id: 18, offset_ms: 1200 },
            { id: 16, offset_ms: 1600 },
            { id: 7, offset_ms: 2000 },
            { id: 0, offset_ms: 2800 },
          ],
        },
      })
    })
    const errors: string[] = []
    page.on('pageerror', (e) => errors.push(e.message))
    await page.goto(`/?trial=motion&character=${character}&head=natural`)
    const speak = page.getByRole('button', { name: 'Speak', exact: true })
    await expect(speak).toBeEnabled()
    if (!(await page.evaluate(() => typeof AudioContext !== 'undefined'))) {
      await speak.click()
      await expect(page.getByRole('alert')).toContainText('Web Audio is required')
      expect(requests).toBe(0)
      await info.attach('audio-capability', {
        body: 'This browser build has no Web Audio. Verified explicit failure before any synthesis; playback is not validated in this lane.',
        contentType: 'text/plain',
      })
      return
    }
    await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
    await speak.click()
    await expect(page.getByRole('button', { name: 'Pause speech', exact: true })).toBeVisible()
    await page.waitForTimeout(450)
    await page.getByRole('button', { name: 'Pause speech', exact: true }).click()
    const canvas = page.getByRole('img', { name: 'Animated character', exact: true })
    await page.waitForTimeout(80)
    const paused = await canvas.evaluate((c: HTMLCanvasElement) => c.toDataURL())
    await page.waitForTimeout(160)
    expect(await canvas.evaluate((c: HTMLCanvasElement) => c.toDataURL())).toBe(paused)
    await page.getByRole('button', { name: 'Resume speech', exact: true }).click()
    await page.waitForTimeout(200)
    await page.getByRole('button', { name: 'Stop speech', exact: true }).click()
    await expect(speak).toBeEnabled()
    await speak.click()
    await expect(page.getByRole('button', { name: 'Pause speech', exact: true })).toBeVisible()
    expect(requests).toBe(1)
    await page.screenshot({
      path: info.outputPath(`${character}-natural-synthetic-speech.png`),
      fullPage: true,
    })
    await page.getByRole('button', { name: 'Stop speech', exact: true }).click()
    expect(errors).toEqual([])
  })
}

test('head director is deterministic, continuous and bounded; native closures survive', async ({
  page,
}) => {
  await page.goto('/?trial=motion&head=natural&character=syntelle')
  const result = await page.evaluate(async () => {
    const { idlePose, createHeadDirector, speechBeats } = await import(
      '/src/features/character/animation/headMotion.ts' as string
    )
    const { speechTimeline, mouthAt } = await import(
      '/src/features/character/speech/speechTimeline.ts' as string
    )
    const bounds = { idle: [1.5, 0.8, 0.6], speaking: [3, 2, 1.2] },
      director = createHeadDirector(42, bounds)
    let previous = director.sample(0, { active: false, time: 0, beats: [] }),
      maxStep = 0,
      maxAngle = 0
    for (let ms = 16; ms < 30000; ms += 16) {
      const active = ms >= 4000 && ms < 17000
      const pose = director.sample(ms, {
        active,
        time: (ms - 4000) / 1000,
        beats: active
          ? [
              { time: 1, strength: 1 },
              { time: 3, strength: 0.8 },
            ]
          : [],
      })
      for (const key of ['yaw', 'pitch', 'roll']) {
        maxStep = Math.max(maxStep, Math.abs(pose[key] - previous[key]))
        maxAngle = Math.max(maxAngle, Math.abs(pose[key]))
      }
      previous = pose
    }
    const a = idlePose(14542, 41, bounds.idle),
      b = idlePose(14542, 41, bounds.idle)
    const timeline = speechTimeline(
      [
        { id: 2, offset_ms: 0 },
        { id: 21, offset_ms: 300 },
        { id: 18, offset_ms: 450 },
        { id: 0, offset_ms: 900 },
      ],
      1000,
    )
    const silence = speechBeats(new Float32Array(24000), 24000)
    let missingRejected = false
    try {
      speechTimeline([], 1000)
    } catch {
      missingRejected = true
    }
    return {
      deterministic: JSON.stringify(a) === JSON.stringify(b),
      maxStep,
      maxAngle,
      closure: mouthAt(timeline, 0.36).to,
      contact: mouthAt(timeline, 0.51).to,
      silence: silence.length,
      missingRejected,
    }
  })
  expect(result.deterministic).toBe(true)
  expect(result.maxStep).toBeLessThan(0.2)
  expect(result.maxAngle).toBeLessThanOrEqual(3)
  expect(result.closure).toBe('MBP')
  expect(result.contact).toBe('FV')
  expect(result.silence).toBe(0)
  expect(result.missingRejected).toBe(true)
})

test('missing native events are explicit; reduced motion retains essential mouth playback', async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.route('**/api/v1/speech/synthesize', (route) =>
    route.fulfill({ json: { audio_base64: tone(), duration_ms: 3000, visemes: [] } }),
  )
  await page.goto('/?trial=motion&head=natural&character=syntelle')
  await expect(page.getByRole('button', { name: 'Speak', exact: true })).toBeEnabled()
  await expect(page.getByLabel('Subtle movement', { exact: true })).not.toBeChecked()
  await page.getByRole('button', { name: 'Speak', exact: true }).click()
  const webAudio = await page.evaluate(() => typeof AudioContext !== 'undefined')
  await expect(page.getByRole('alert')).toContainText(
    webAudio ? 'no spoken visemes' : 'Web Audio is required',
  )
})

test('stopping during browser audio activation prevents a late synthesis request', async ({
  page,
}) => {
  let requests = 0
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.route('**/api/v1/speech/synthesize', (route) => {
    requests++
    return route.fulfill({ status: 500, json: {} })
  })
  await page.goto('/?trial=motion&head=natural&character=syntelle')
  if (!(await page.evaluate(() => typeof AudioContext !== 'undefined'))) return
  await page.evaluate(() => {
    const original = AudioContext.prototype.resume
    AudioContext.prototype.resume = async function () {
      await original.call(this)
      await new Promise<void>((resolve) => {
        ;(window as unknown as { releaseSpeechActivation: () => void }).releaseSpeechActivation =
          resolve
      })
    }
  })
  await page.getByRole('button', { name: 'Speak', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Stop speech', exact: true })).toBeEnabled()
  await page.waitForFunction(
    () =>
      typeof (window as unknown as { releaseSpeechActivation?: unknown })
        .releaseSpeechActivation === 'function',
  )
  await page.getByRole('button', { name: 'Stop speech', exact: true }).click()
  await page.evaluate(() =>
    (window as unknown as { releaseSpeechActivation: () => void }).releaseSpeechActivation(),
  )
  await page.waitForTimeout(100)
  expect(requests).toBe(0)
  await expect(page.getByRole('button', { name: 'Speak', exact: true })).toBeEnabled()
  expect(errors).toEqual([])
})

test('head turns rotate without translation, stay bounded and return smoothly into idle', async ({
  page,
}) => {
  await page.goto('/?trial=motion&head=rotation-candidate&character=syntelle')
  const cases = await page.evaluate(async () => {
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const { idlePose, createHeadDirector } = await import(
      '/src/features/character/animation/headMotion.ts' as string
    )
    return ['syntelle', 'cafe', 'woodland'].map((character) => {
      const m = resolveMotionContext(character, true, 'rotation').definition,
        rig = m.rig,
        profile = rig.posture
      const director = createHeadDirector(42, rig.bounds, profile)
      let previous = director.sample(0, { active: false, time: 0, beats: [] }),
        maxAngleStep = 0,
        minYaw = 0,
        maxYaw = 0,
        rotationOnly = true,
        bounded = true
      for (let ms = 16; ms < 40000; ms += 16) {
        const active = ms >= 8000 && ms < 16000
        const pose = director.sample(ms, {
          active,
          time: (ms - 8000) / 1000,
          beats: active
            ? [
                { time: 1, strength: 1 },
                { time: 3, strength: 0.8 },
              ]
            : [],
        })
        for (const [i, key] of ['yaw', 'pitch', 'roll'].entries()) {
          maxAngleStep = Math.max(maxAngleStep, Math.abs(pose[key] - previous[key]))
          bounded &&= Math.abs(pose[key]) <= rig.bounds.speaking[i] + 1e-8
        }
        minYaw = Math.min(minYaw, pose.yaw)
        maxYaw = Math.max(maxYaw, pose.yaw)
        rotationOnly &&= Object.keys(pose).every((key) => ['yaw', 'pitch', 'roll'].includes(key))
        previous = pose
      }
      return {
        character,
        bounded,
        maxAngleStep,
        rotationOnly,
        minYaw,
        maxYaw,
        idleLimit: rig.bounds.idle[0],
        quiet: idlePose(2900, 42, rig.bounds.idle, profile).yaw,
        deterministic:
          JSON.stringify(idlePose(1040, 42, rig.bounds.idle, profile)) ===
          JSON.stringify(idlePose(1040, 42, rig.bounds.idle, profile)),
      }
    })
  })
  for (const item of cases) {
    expect(item.bounded, item.character).toBe(true)
    expect(item.deterministic).toBe(true)
    expect(item.minYaw).toBeLessThan(-item.idleLimit * 0.6)
    expect(item.maxYaw).toBeGreaterThan(item.idleLimit * 0.6)
    expect(item.rotationOnly).toBe(true)
    expect(item.maxAngleStep).toBeLessThan(0.6)
    expect(item.quiet).toBe(0)
  }
})
