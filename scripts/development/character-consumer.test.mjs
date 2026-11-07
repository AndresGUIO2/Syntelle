import test from 'node:test'
import assert from 'node:assert/strict'
import { consumerConfiguration, ownsConsumer, ensureConsumer } from './character-consumer.mjs'

const configured = { SYNTELLE_CHARACTER_PACKAGES: '.tools/private/packages', SYNTELLE_CHARACTER_SCRATCH: '.tools/private/work', SYNTELLE_CHARACTER_CHECKER: '.tools/render-frame.exe' }
const identity = { pid: 12345, birth: 'immutable-start-time', commandHash: 'exact-command-hash' }
const record = { version: 'character-consumer/1', ...identity, configurationHash: 'config' }

test('creation remains disabled by default and never invents paid authorization', () => {
  assert.equal(consumerConfiguration(process.cwd(), {}), null)
  assert.throws(() => consumerConfiguration(process.cwd(), { SYNTELLE_CHARACTER_PACKAGES: 'packages' }), /together/)
  const config = consumerConfiguration(process.cwd(), configured)
  assert.ok(config.args.includes('app.workflows.characters.worker'))
  assert.ok(!config.args.includes('--approval'))
  assert.ok(!JSON.stringify(config).includes('DATABASE_URL'))
  const approved = consumerConfiguration(process.cwd(), { ...configured, SYNTELLE_CHARACTER_APPROVAL: '.tools/operator-approval.json' })
  assert.ok(approved.args.includes('--approval'))
  assert.notEqual(config.hash, approved.hash)
})

test('PID reuse and changed command lines never establish ownership', () => {
  assert.ok(ownsConsumer(record, identity))
  for (const actual of [null, { foreign: true }, { ...identity, birth: 'other-start' }, { ...identity, commandHash: 'other-command' }, { ...identity, pid: 9 }])
    assert.equal(ownsConsumer(record, actual), false)
})

test('repeated start reuses exactly the running configured consumer', async () => {
  const forbidden = async () => assert.fail('must not start or stop a process')
  assert.equal(await ensureConsumer({ hash: 'config' }, 'state', { read: async () => record, inspect: async () => identity, stop: forbidden, launch: forbidden, remove: forbidden }), 'reused')
})

test('foreign or PID-reused process is left untouched', async () => {
  const forbidden = async () => assert.fail('foreign process must be untouched')
  for (const actual of [{ foreign: true }, { ...identity, birth: 'new' }]) {
    await assert.rejects(ensureConsumer({ hash: 'config' }, 'state', { read: async () => record, inspect: async () => actual, stop: forbidden, launch: forbidden, remove: forbidden }), /ownership changed/)
  }
})

test('stop ends only verified own process and removes its state', async () => {
  const calls = []
  assert.equal(await ensureConsumer(null, 'state', { read: async () => record, inspect: async () => identity, stop: async (r) => calls.push(['stop', r.pid]), remove: async () => calls.push(['remove']) }), 'disabled')
  assert.deepEqual(calls, [['stop', 12345], ['remove']])
})

test('untracked consumer prevents duplicate launch', async () => {
  await assert.rejects(ensureConsumer({ hash: 'config' }, 'state', { read: async () => null, find: async () => [identity], launch: async () => assert.fail('duplicate') }), /untracked/)
})

test('stale record is replaced only after checking for an untracked consumer', async () => {
  const calls = []
  assert.equal(await ensureConsumer({ hash: 'config' }, 'state', {
    read: async () => record, inspect: async () => null, remove: async () => calls.push('remove'), find: async () => { calls.push('find'); return [] },
    launch: async () => { calls.push('launch'); return identity }, write: async (r) => { calls.push('write'); assert.equal(r.configurationHash, 'config'); assert.equal(r.pid, 12345) },
  }), 'started')
  assert.deepEqual(calls, ['remove', 'find', 'launch', 'write'])
})
