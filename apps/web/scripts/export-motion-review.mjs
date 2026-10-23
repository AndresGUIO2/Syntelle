// Local review only: uses the runtime compositors and cached, declared assets.
import { chromium } from '@playwright/test'
import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
const character = process.argv[2] ?? 'syntelle'
const rotation = process.argv.includes('--rotation-candidate')
const contextual = process.argv.includes('--contextual-candidate')
const vitality = process.argv.includes('--vitality-candidate')
const projection = process.argv.includes('--rgbd-candidate')
const natural =
  process.argv.includes('--natural') || rotation || contextual || vitality || projection
const output = resolve(
  process.env.SYNTELLE_REVIEW_OUTPUT ??
    `../../.tools/${projection ? 'rgbd-motion-18' : vitality ? 'vitality-motion-17' : contextual ? 'contextual-motion-15' : natural ? 'head-motion-14' : 'motion-09'}/${character}/browser-review`,
)
mkdirSync(output, { recursive: true })
const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
await page.goto(
  `http://127.0.0.1:5173/?trial=motion&character=${character}${natural ? `&head=${projection ? 'rgbd-candidate' : vitality ? 'vitality-candidate' : contextual ? 'contextual-candidate' : rotation ? 'rotation-candidate' : 'natural'}` : ''}`,
)
await page.getByRole('button', { name: 'Blink once', exact: true }).waitFor()
await page.waitForFunction(() => document.querySelector('.character-layer canvas'))
const report = await page.evaluate(
  async ({ character, natural, rotation, contextual, vitality, projection }) => {
    const { motionDefinition, motionAssetUrl } =
      await import('/src/experiments/motion/useMotionAssets.ts')
    const { createEyeMorph } = await import('/src/experiments/motion/eyeMorph.ts')
    const { createBodyRenderer } = await import('/src/features/character/rendering/bodyRenderer.ts')
    const { blinkAt, randomSequence } = await import('/src/experiments/motion/useMotionPlayback.ts')
    const { resolvePoseContext } = await import('/src/experiments/viseme-a/poseCatalog.ts')
    const { package: p, assetUrl: poseAssetUrl } = resolvePoseContext({
      character,
      trial: 'motion',
    })
    const { createCompletedMorph } = await import('/src/experiments/viseme-a/completedMorph.ts')
    const { resolveMotionContext } = await import('/src/experiments/motion/motionCatalog.ts')
    const { idlePose } = await import('/src/features/character/animation/headMotion.ts')
    const { quietPosture, gestureAt } =
      await import('/src/features/character/animation/speechBehavior.ts')
    const context = resolveMotionContext(
      character,
      natural,
      projection
        ? 'rgbd'
        : vitality
          ? 'vitality'
          : contextual
            ? 'contextual'
            : rotation
              ? 'rotation'
              : 'current',
    )
    const m = natural
      ? resolveMotionContext(
          character,
          true,
          projection
            ? 'rgbd'
            : vitality
              ? 'vitality'
              : contextual
                ? 'contextual'
                : rotation
                  ? 'rotation'
                  : 'current',
        ).definition
      : motionDefinition(character)
    const posture = (ms) =>
      m.behavior
        ? quietPosture(ms, 42, m.projection?.proposed_idle_degrees ?? m.behavior.head.idle_degrees)
        : natural
          ? idlePose(ms, 42, m.rig.bounds.idle, m.rig.posture)
          : undefined
    const load = async (url) => {
      const im = new Image()
      im.src = url
      await im.decode()
      return im
    }
    const read = (image, w = image.width, h = image.height) => {
      const c = document.createElement('canvas')
      c.width = w
      c.height = h
      c.getContext('2d').drawImage(image, 0, 0, w, h)
      return c.getContext('2d').getImageData(0, 0, w, h).data
    }
    const assets = await Promise.all(
      [m.foreground, m.depth, m.eyes.open, m.eyes.closed, m.eyes.mask].map((file) =>
        load(motionAssetUrl(character, file)),
      ),
    )
    const [fg, depth, opened, closed, mask] = assets
    const eye = createEyeMorph(opened, closed, mask, m.eyes.curves, () => {
      throw Error('Eye context lost')
    })
    const geometry = m.projection
      ? await (await fetch(context.assetUrl(m.projection.geometry.file))).arrayBuffer()
      : undefined
    const body = createBodyRenderer(
      { foreground: fg, depth, opened, closed, mask, geometry },
      m,
      () => {
        throw Error('Body context lost')
      },
    )
    const source = read(opened),
      target = read(closed),
      alpha = read(mask),
      images = {},
      blinkChecks = []
    let maximumOutsideEyeChange = 0,
      maximumExposedIrisChange = 0,
      maximumClosedError = 0
    for (const c of [0, 0.01, 0.25, 0.5, 0.75, 0.99, 1]) {
      eye.draw(c)
      const data = read(eye.canvas),
        w = opened.width,
        h = opened.height
      images[`eyes-${c}`] = eye.canvas.toDataURL()
      for (let y = 0; y < h; y++)
        for (let x = 0; x < w; x++) {
          const i = (y * w + x) * 4,
            curve = m.eyes.curves[x],
            a = (1 - c) * curve[0] + c * curve[2],
            b = (1 - c) * curve[1] + c * curve[2]
          for (let k = 0; k < 3; k++) {
            if (alpha[i] === 0)
              maximumOutsideEyeChange = Math.max(
                maximumOutsideEyeChange,
                Math.abs(data[i + k] - source[i + k]),
              )
            if (curve[3] && y > a + 1 && y < b - 1)
              maximumExposedIrisChange = Math.max(
                maximumExposedIrisChange,
                Math.abs(data[i + k] - source[i + k]),
              )
            if (c === 1 && alpha[i] === 255)
              maximumClosedError = Math.max(
                maximumClosedError,
                Math.abs(data[i + k] - target[i + k]),
              )
          }
        }
    }
    for (let ms = 0; ms <= 272; ms += 16) {
      const c = blinkAt(ms, m.blink)
      eye.draw(c)
      images[`blink-${String(ms).padStart(3, '0')}`] = eye.canvas.toDataURL()
      blinkChecks.push({ ms, closure: c })
    }
    const random1 = randomSequence(1837),
      random2 = randomSequence(1837)
    const timing = {
      deterministic: Array.from({ length: 100 }, () => random1() === random2()).every(Boolean),
      values: [-1, 0, 40, 80, 119, 120, 190, 260].map((ms) => [ms, blinkAt(ms, m.blink)]),
    }
    const upper = await load(poseAssetUrl(p.reference.upper)),
      lower = await load(poseAssetUrl(p.reference.lower)),
      poses = {}
    for (const [id, data] of Object.entries(p.poses))
      poses[id] = {
        data,
        surface: await load(poseAssetUrl(data.surface)),
        mask: await load(poseAssetUrl(data.mask)),
        cavity: data.cavity ? await load(poseAssetUrl(data.cavity)) : undefined,
      }
    const patch = (rect, frame) => {
      const c = document.createElement('canvas')
      c.width = rect.width
      c.height = rect.height
      const ctx = c.getContext('2d')
      ctx.drawImage(fg, -rect.x, -rect.y)
      ctx.globalCompositeOperation = 'source-atop'
      ctx.drawImage(frame, 0, 0)
      body.update(rect, c)
    }
    const endpoints = {},
      checks = [],
      projectionChecks = []
    let maxEndpointJump = 0
    for (const [key, pair] of Object.entries(p.pairs)) {
      if (!pair.safe) throw Error(`Invalid oral geometry ${key}`)
      const [a, b] = key.split('-'),
        mouth = createCompletedMorph(
          poses[a],
          poses[b],
          pair.mesh,
          upper,
          lower,
          () => {
            throw Error('Oral context lost')
          },
          p.oral,
        )
      for (const t of [0, 0.35, 0.41, 0.5, 1]) {
        mouth.draw(t)
        const pixels = read(mouth.canvas)
        if (t === 0 || t === 1) {
          const id = t === 0 ? a : b
          if (endpoints[id])
            for (let i = 0; i < pixels.length; i++)
              maxEndpointJump = Math.max(maxEndpointJump, Math.abs(pixels[i] - endpoints[id][i]))
          else endpoints[id] = pixels
        }
        patch(p.crop, mouth.canvas)
        for (const c of [0, 0.5, 1]) {
          eye.draw(c)
          patch(m.eyes.crop, eye.canvas)
          body.draw(0, false)
          checks.push({ pair: key, t, eyes: c })
        }
        if (a === b && t === 0) {
          images[`endpoint-${a}`] = body.canvas.toDataURL()
          images[`mouth-${a}`] = mouth.canvas.toDataURL()
        }
        if ((key === 'EI-FV' && t === 0.35) || (key === 'FV-A' && t === 0.41)) {
          images[`reported-${key}-${t}`] = body.canvas.toDataURL()
        }
        if (
          m.projection &&
          ((a === b && t === 0) ||
            (key === 'EI-FV' && t === 0.35) ||
            (key === 'FV-A' && t === 0.41))
        ) {
          for (const c of [0, 0.5, 1])
            for (const yaw of [-3, 0, 3]) {
              eye.draw(c)
              patch(m.eyes.crop, eye.canvas)
              body.draw(0, true, { yaw, pitch: 0, roll: 0 })
              projectionChecks.push({ key, t, eyes: c, yaw })
              images[`combined-${key}-${t}-${c}-${yaw}`] = body.canvas.toDataURL()
            }
        }
      }
      mouth.dispose()
    }
    eye.draw(0)
    patch(m.eyes.crop, eye.canvas)
    const neutralMouth = createCompletedMorph(
      poses.Rest,
      poses.Rest,
      p.pairs['Rest-Rest'].mesh,
      upper,
      lower,
      () => {
        throw Error('Oral context lost')
      },
      p.oral,
    )
    neutralMouth.draw(0)
    patch(p.crop, neutralMouth.canvas)
    neutralMouth.dispose()
    body.draw(0, false)
    images['head-neutral'] = body.canvas.toDataURL()
    if (m.projection)
      for (const yaw of [-3, -2, -1, 0, 1, 2, 3])
        for (const pitch of [-1, 0, 1]) {
          body.draw(0, true, { yaw, pitch, roll: 0 })
          images[`projection-${yaw}-${pitch}`] = body.canvas.toDataURL()
        }
    for (const ms of [0, 600, 1100, 2100, 2900, 5800, 9100]) {
      body.draw(ms, true, posture(ms))
      images[`motion-${ms}`] = body.canvas.toDataURL()
    }
    if (natural)
      for (const sign of [-1, 1]) {
        const limits = m.behavior
          ? [
              m.behavior.head.maximum_turn_degrees + m.behavior.head.idle_degrees[0],
              m.behavior.head.maximum_nod_degrees + m.behavior.head.idle_degrees[1],
              m.behavior.head.idle_degrees[2],
            ]
          : m.rig.bounds.speaking
        const [yaw, pitch, roll] = limits.map((value) => value * sign)
        body.draw(0, true, { yaw, pitch, roll })
        images[`head-bound-${sign}`] = body.canvas.toDataURL()
        body.draw(0, true, {
          yaw: (m.behavior?.head.idle_degrees[0] ?? m.rig.bounds.idle[0]) * sign,
          pitch: 0,
          roll: 0,
        })
        images[`head-side-${sign}`] = body.canvas.toDataURL()
      }
    if (m.behavior) {
      const plan = {
        version: 'speech-context/1',
        duration: 2,
        cues: [],
        gestures: [
          {
            time: 0,
            duration: 1.3,
            pitch: -m.behavior.head.maximum_nod_degrees,
            yaw: 0,
            kind: 'pause',
          },
        ],
      }
      for (const ms of [0, 200, 400, 550, 800, 1100, 1300]) {
        eye.draw(blinkAt(ms - 400, m.blink))
        patch(m.eyes.crop, eye.canvas)
        body.draw(ms, true, gestureAt(plan, ms / 1000))
        images[`nod-${ms}`] = body.canvas.toDataURL()
      }
    }
    const gl = body.canvas.getContext('webgl'),
      durations = []
    // Include actual GPU completion, not just command submission or display Hz.
    for (let i = 0; i < 120; i++) {
      const start = performance.now()
      eye.draw(blinkAt(i % 260, m.blink))
      patch(m.eyes.crop, eye.canvas)
      body.draw(i * 16.67, true, posture(i * 16.67))
      gl.finish()
      durations.push(performance.now() - start)
    }
    durations.sort((a, b) => a - b)
    const performanceResult = {
      frames: durations.length,
      median_ms: durations[60],
      p95_ms: durations[114],
      mode: 'local headless Chromium, GPU completion included; not a physical mobile benchmark',
    }
    eye.dispose()
    body.dispose()
    return {
      images,
      checks,
      timing,
      blinkChecks,
      maximumOutsideEyeChange,
      maximumExposedIrisChange,
      maximumClosedError,
      maxEndpointJump,
      performance: performanceResult,
      uploads: body.stats,
      build_id: m.build_id,
      oral_build_id: p.build_id,
      projectionChecks,
      source_geometry_sha256: m.projection?.geometry.sha256,
    }
  },
  { character, natural, rotation, contextual, vitality, projection },
)
for (const [name, image] of Object.entries(report.images))
  writeFileSync(resolve(output, `${name}.png`), Buffer.from(image.split(',')[1], 'base64'))
delete report.images
await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
await page.getByLabel('Subtle movement', { exact: true }).uncheck()
await page.getByLabel('Eyelid closure', { exact: true }).press('Home')
await page.screenshot({ path: resolve(output, 'browser.png'), fullPage: true })
writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2))
await browser.close()
console.log(
  JSON.stringify(
    { ...report, checks: report.checks.length, blinkChecks: report.blinkChecks.length },
    null,
    2,
  ),
)
