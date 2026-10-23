import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createServer } from 'vite'
import { isPrivatePlayerModule, playerBoundary } from '../../config/player-boundary.ts'

test('public boundary blocks internal files while the interface loads', async () => {
  for (const id of [
    '/src/features/character/rendering/gl.ts?raw',
    '/src/features/demo/DemoWorkspace.vue',
    '/src/features/conversation/composables/useReplySpeech.ts',
    '/src/features/streaming/components/StreamComposer.vue',
    '/src/new-algorithm.ts',
  ])
    assert.equal(isPrivatePlayerModule(id), true, id)
  assert.equal(
    isPrivatePlayerModule('/src/features/streaming/StreamWorkspace.vue?vue&type=script'),
    false,
  )
  const server = await createServer({ server: { port: 0, strictPort: false } })
  try {
    await server.listen()
    const base = server.resolvedUrls!.local[0]!
    for (const path of [
      'src/features/character/rendering/gl.ts?raw',
      'src/features/conversation/composables/useReplySpeech.ts',
      'src/%65xperiments/motion/backgrounds/light-studio-thumb.webp',
      'config/demo-build.ts',
      'tests/build/demo-build.test.ts',
      'package.json',
      '@fs/E:/Carpetas/Documentos/01-MisProyectos/ImageAssistant/docs/README.md',
    ])
      assert.equal((await fetch(new URL(path, base))).status, 403, path)
    for (const path of ['', 'src/main.ts', 'src/features/streaming/StreamWorkspace.vue'])
      assert.equal((await fetch(new URL(path, base))).status, 200, path)
  } finally {
    await server.close()
  }
})

test('build rejects unreviewed modules and private resources', () => {
  const generate = playerBoundary().generateBundle as (...args: unknown[]) => void
  assert.throws(
    () =>
      generate(
        {},
        {
          chunk: { type: 'chunk', fileName: 'assets/index.js', modules: { '/src/secret.ts': {} } },
        },
      ),
    /Unreviewed source/,
  )
  for (const fileName of [
    'assets/geometry.bin',
    'assets/package.json',
    'assets/source.js.map',
    'assets/source.jpg',
  ])
    assert.throws(() => generate({}, { asset: { type: 'asset', fileName } }), /Unreviewed resource/)
})
