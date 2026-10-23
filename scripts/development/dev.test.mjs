import test from 'node:test'
import assert from 'node:assert/strict'
import { cleanupPlan, ensureWorker } from './dev.mjs'
import {
  waitForDetachedWorker,
  nativeArguments,
} from '../../services/render-worker/scripts/start-local.mjs'

test('cleanup removes stopped development resources while retaining active images and other projects', () => {
  const containers = [
    { id: 'old', name: '/syntelle-native-media-build', running: false, image: 'compiler' },
    { id: 'current', name: '/syntelle-api-1', running: true, image: 'api' },
    { id: 'other', name: '/orglens-understanding-e2e-db', running: false, image: 'postgres16' },
    {
      id: 'active-build',
      name: '/syntelle-native-efficiency-build-57',
      running: true,
      image: 'builder',
    },
  ]
  const images = [
    { id: 'compiler', tags: ['syntelle-native-media-build:56'] },
    { id: 'api', tags: ['syntelle-api:latest'] },
    { id: 'postgres16', tags: ['postgres:16-alpine'] },
    { id: 'builder', tags: ['syntelle-api-check:latest'] },
    { id: 'old-owned', tags: [], labels: { 'io.syntelle.project': 'syntelle' } },
    { id: 'unrelated', tags: [] },
    { id: 'tracked-old', tags: [] },
    { id: 'reference', tags: ['syntelle-render-native:before-efficiency-57'] },
  ]
  assert.deepEqual(cleanupPlan(containers, images, new Set(['tracked-old'])), {
    containers: ['syntelle-native-media-build'],
    images: ['syntelle-native-media-build:56', 'old-owned', 'tracked-old'],
  })
})

const worker = {
  name: '/syntelle-render-native-local',
  running: true,
  imageName: 'syntelle-render-native:local',
  network: 'container:current-api',
}
test('repeated start reuses the connected renderer without launching or stopping anything', async () => {
  const forbidden = () => assert.fail('Connected worker must be reused')
  const deps = {
    inspect: async () => worker,
    apiID: 'current-api',
    status: async () => ({ available: true }),
    stop: forbidden,
    launch: forbidden,
    wait: forbidden,
  }
  assert.equal(await ensureWorker(deps), 'reused')
  assert.equal(await ensureWorker(deps), 'reused')
})
test('API recreation replaces only the owned stale namespace and starts one renderer', async () => {
  const calls = []
  assert.equal(
    await ensureWorker({
      inspect: async () => worker,
      apiID: 'new-api',
      status: async () => ({ available: false }),
      stop: async () => calls.push('stop'),
      launch: async () => calls.push('launch'),
      wait: () => assert.fail('Stale namespace cannot be reused'),
    }),
    'started',
  )
  assert.deepEqual(calls, ['stop', 'launch'])
})
test('a foreign container or separately connected renderer is never taken over', async () => {
  const forbidden = () => assert.fail('Unrelated renderer must be left alone')
  await assert.rejects(
    ensureWorker({
      inspect: async () => ({ ...worker, imageName: 'other-project:latest' }),
      stop: forbidden,
      launch: forbidden,
    }),
    /another resource/,
  )
  await assert.rejects(
    ensureWorker({
      inspect: async () => undefined,
      status: async () => ({ available: true }),
      launch: forbidden,
    }),
    /Another renderer/,
  )
})
test('launcher process failure cleans its new owned container before surfacing the error', async () => {
  let inspections = 0
  let removed = 0
  await assert.rejects(
    ensureWorker({
      inspect: async () => (++inspections === 1 ? undefined : worker),
      status: async () => ({ available: false }),
      launch: async () => {
        throw Error('Launcher timed out')
      },
      stop: async () => {
        removed++
      },
    }),
    /Launcher timed out/,
  )
  assert.equal(removed, 1)
})
test('detached startup returns on readiness and removes a failed new worker on error', async () => {
  const removed = []
  let ready = 0
  const deps = {
    inspect: async () => 'true',
    status: async () => ({ available: ++ready > 1 }),
    stop: async (id) => removed.push(id),
    pause: async () => {},
    attempts: 3,
  }
  await waitForDetachedWorker('ready', deps)
  assert.deepEqual(removed, [])
  await assert.rejects(
    waitForDetachedWorker('failed', { ...deps, inspect: async () => 'false' }),
    /exited/,
  )
  await assert.rejects(
    waitForDetachedWorker('timeout', { ...deps, status: async () => ({ available: false }) }),
    /did not connect/,
  )
  assert.deepEqual(removed, ['failed', 'timeout'])
})
test('native worker uses automatic removal and bounded logs, without restart/volume deletion', () => {
  const args = nativeArguments({
    image: 'syntelle-render-native:local',
    container: 'api',
    assets: 'private',
    api: 'http://127.0.0.1:8080',
    capacity: 2,
    name: 'syntelle-render-native-local',
  })
  assert.ok(args.includes('--rm'))
  assert.ok(args.includes('max-size=5m'))
  assert.ok(args.includes('max-file=2'))
  assert.ok(args.includes('io.syntelle.project=syntelle'))
  assert.equal(args.includes('--restart'), false)
  assert.equal(args.includes('--env-file'), false)
})
