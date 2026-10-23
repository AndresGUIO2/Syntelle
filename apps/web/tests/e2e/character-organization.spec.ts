import { expect, test } from '@playwright/test'

test('historical character module URLs remain usable after source relocation', async ({ page }) => {
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const legacy = await import('/src/features/character/headMotion.ts' as string)
    const canonical = await import('/src/features/character/animation/headMotion.ts' as string)
    const oldTiming = await import('/src/features/character/timing.ts' as string)
    const oldSpeech = await import('/src/features/character/speechTimeline.ts' as string)
    const oldBehavior = await import('/src/features/character/speechBehavior.ts' as string)
    const settings = { closing_ms: 80, closed_ms: 40, opening_ms: 140 }
    return {
      sameTrajectory:
        JSON.stringify(legacy.idlePose(1234, 42, [1.5, 0.8, 0.6])) ===
        JSON.stringify(canonical.idlePose(1234, 42, [1.5, 0.8, 0.6])),
      closed: oldTiming.blinkAt(100, settings),
      audioDecoder: typeof oldSpeech.decodeSpeechWav,
      planner: typeof oldBehavior.planSpeechBehavior,
    }
  })
  expect(result).toEqual({
    sameTrajectory: true,
    closed: 1,
    audioDecoder: 'function',
    planner: 'function',
  })
})

test('explicit package contexts can load two characters in one document', async ({ page }) => {
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const { createApp, defineComponent, h } = await import(
      '/node_modules/.vite/deps/vue.js' as string
    )
    const { resolvePoseContext } = await import(
      '/src/experiments/viseme-a/poseCatalog.ts' as string
    )
    const { useOralAssets } = await import(
      '/src/features/character/composables/useOralAssets.ts' as string
    )
    const first = resolvePoseContext({ character: 'cafe' })
    const second = resolvePoseContext({ character: 'woodland' })
    const results: Array<{ character: string; source: string; width: number }> = []
    for (const context of [first, second]) {
      const host = document.createElement('div')
      document.body.append(host)
      let assets: ReturnType<typeof useOralAssets>
      const app = createApp(
        defineComponent({
          setup() {
            assets = useOralAssets(context)
            return () => h('div')
          },
        }),
      )
      app.mount(host)
      await new Promise<void>((resolve, reject) => {
        const started = performance.now()
        const poll = () => {
          if (assets.error.value) return reject(Error(assets.error.value))
          if (assets.images.value) return resolve()
          if (performance.now() - started > 10000) return reject(Error('Asset load timeout'))
          requestAnimationFrame(poll)
        }
        poll()
      })
      results.push({
        character: context.character!,
        source: assets!.images.value!.base.src,
        width: assets!.images.value!.base.width,
      })
      app.unmount()
      host.remove()
    }
    return {
      results,
      unchanged: first.character === 'cafe' && first.assetUrl('base.webp').includes('/cafe/'),
    }
  })
  expect(result.unchanged).toBe(true)
  expect(result.results.map((row) => row.character)).toEqual(['cafe', 'woodland'])
  expect(result.results.every((row) => row.width > 0)).toBe(true)
  expect(result.results[0]!.source).not.toBe(result.results[1]!.source)
})

test('pending image decodes cannot publish after their component is disposed', async ({ page }) => {
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const { createApp, defineComponent, h } = await import(
      '/node_modules/.vite/deps/vue.js' as string
    )
    const { usePreparedImages } = await import(
      '/src/features/character/composables/usePreparedImages.ts' as string
    )
    const original = window.Image
    const pending: Array<{ src: string; finish: () => void }> = []
    class DelayedImage {
      src = ''
      finish = () => {}
      decode() {
        return new Promise<void>((resolve) => {
          this.finish = resolve
          pending.push(this)
        })
      }
    }
    Object.defineProperty(window, 'Image', { configurable: true, value: DelayedImage })
    const host = document.createElement('div')
    document.body.append(host)
    let assets: ReturnType<typeof usePreparedImages>
    const app = createApp(
      defineComponent({
        setup() {
          assets = usePreparedImages(
            (load: (url: string) => Promise<HTMLImageElement>) => load('/delayed.png'),
            'failed',
          )
          return () => h('div')
        },
      }),
    )
    try {
      app.mount(host)
      app.unmount()
      const cancelled = pending.every((image) => image.src === '') && pending.length === 1
      pending.forEach((image) => image.finish())
      await Promise.resolve()
      await Promise.resolve()
      await Promise.resolve()
      return { cancelled, empty: assets!.images.value === null, error: assets!.error.value }
    } finally {
      Object.defineProperty(window, 'Image', { configurable: true, value: original })
      host.remove()
    }
  })
  expect(result).toEqual({ cancelled: true, empty: true, error: '' })
})
