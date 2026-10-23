import test from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { nativeArguments, nativeAssetMount, localMediaPort, gpuArguments, gpuAvailable, videoEncoder, rendererMode, gpuPipeline } from '../scripts/start-local.mjs'

test('retired browser engine fails before private configuration or Docker access', () => {
  const script = fileURLToPath(new URL('../scripts/start-local.mjs', import.meta.url))
  const result = spawnSync(process.execPath, [script, '--engine', 'chromium'], {
    env: { ...process.env, SYNTELLE_RENDER_WORKER_TOKEN: '' }, encoding: 'utf8', timeout: 2000,
  })
  assert.equal(result.error, undefined)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /Only the native Go render worker is supported/)
})

test('local media mapping validates the port and permits explicit direct ICE', () => {
  assert.equal(localMediaPort(), 8081)
  assert.equal(localMediaPort('0'), 0)
  for (const invalid of ['-1', '80', '65536', '8081/udp', '1;command'])
    assert.throws(() => localMediaPort(invalid), /Local media port/)
})

test('explicit asset volume is read-only with no image-to-volume copying', () => {
  const args = nativeArguments({
    image: 'syntelle-render-native:local', container: 'syntelle-api-1',
    assetsVolume: 'syntelle-native-efficiency-57-assets', api: 'http://127.0.0.1:8080',
    capacity: 2, name: 'syntelle-render-native-local',
  })
  assert.equal(args[args.indexOf('--mount') + 1],
    'type=volume,source=syntelle-native-efficiency-57-assets,target=/opt/syntelle-assets,readonly,volume-nocopy')
  assert.deepEqual(args.filter((_, i) => args[i - 1] === '--env'), ['SYNTELLE_RENDER_WORKER_TOKEN', 'SYNTELLE_VIDEO_ENCODER=auto', 'SYNTELLE_RENDERER=auto', 'SYNTELLE_GPU_PIPELINE=device'])
  assert.equal(args.includes('--env-file'), false)
})

test('GPU access includes video capability without privileges or host mounts', () => {
  assert.deepEqual(gpuArguments(false), [])
  assert.deepEqual(gpuArguments(true), ['--gpus', 'all', '--env', 'NVIDIA_DRIVER_CAPABILITIES=compute,video,utility'])
  for (const mode of ['auto', 'nvenc', 'openh264']) assert.equal(videoEncoder(mode), mode)
  assert.throws(() => videoEncoder('nvenc;command'), /Video encoder/)
  for (const mode of ['auto', 'gpu', 'cpu']) assert.equal(rendererMode(mode), mode)
  assert.throws(() => rendererMode('gpu;command'), /Renderer/)
  assert.equal(gpuPipeline('host'),'host')
  assert.throws(() => gpuPipeline('host;command'), /GPU pipeline/)
})

test('unavailable Docker GPU cleans its probe and selects software availability', async () => {
  const removed = []
  for (const available of [true, false]) {
    const result = await gpuAvailable('local-image', {
      run: async (args) => {
        assert.equal(args.includes('--network'), true)
        assert.equal(args.includes('SYNTELLE_RENDER_WORKER_TOKEN'), false)
        if (!available) throw Error('driver unavailable')
      },
      remove: async (name) => removed.push(name),
    })
    assert.equal(result, available)
  }
  assert.equal(removed.length, 2)
})

test('asset directory selection keeps its existing read-only bind', () => {
  assert.equal(nativeAssetMount({ assets: 'E:/private assets/package' }),
    'type=bind,source=E:/private assets/package,target=/opt/syntelle-assets,readonly')
})

test('asset volume selection rejects paths, mount options and ambiguous sources', () => {
  for (const assetsVolume of ['', '-flag', '/path', 'E:/private', 'volume,readonly', 'two volumes'])
    assert.throws(() => nativeAssetMount({ assetsVolume }), /existing Docker volume by name/)
  assert.throws(() => nativeAssetMount({ assets: 'private/package', assetsVolume: 'existing-volume' }), /cannot be combined/)
})

test('conflicting CLI asset options fail before private API or Docker access', () => {
  const script = fileURLToPath(new URL('../scripts/start-local.mjs', import.meta.url))
  const result = spawnSync(process.execPath, [script, '--engine', 'native',
    '--assets', 'private/package', '--assets-volume', 'existing-volume'], {
    env: { ...process.env, SYNTELLE_RENDER_WORKER_TOKEN: '' }, encoding: 'utf8', timeout: 2000,
  })
  assert.equal(result.error, undefined)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /Choose --assets or --assets-volume; they cannot be combined/)
})
