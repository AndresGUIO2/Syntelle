import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile, readdir, access } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const root = fileURLToPath(new URL('../../../', import.meta.url))
async function sources(dir) {
  const rows = await readdir(dir, { withFileTypes: true })
  return (await Promise.all(rows.map(async row => {
    const path = join(dir, row.name)
    return row.isDirectory() ? sources(path) : row.name.endsWith('.go') ? [path] : []
  }))).flat()
}

test('worker builds without API implementations and API owns no renderer', async () => {
  for (const folder of ['cmd', 'internal']) {
    for (const file of await sources(join(root, 'services/render-worker', folder))) {
      assert.doesNotMatch(await readFile(file, 'utf8'), /"syntelle\/api(?:\/|")/, file)
    }
    for (const file of await sources(join(root, 'services/api', folder))) {
      assert.doesNotMatch(await readFile(file, 'utf8'), /"(?:syntelle\/render-worker|github.com\/pion)(?:\/|")/, file)
    }
  }
  for (const name of ['nativeadapter','nativeanimation','nativeclock','nativeframe','nativemedia','nativerender'])
    await assert.rejects(access(join(root, 'services/api/internal', name)), { code:'ENOENT' })
  const mod = await readFile(join(root, 'services/render-worker/go.mod'), 'utf8')
  assert.doesNotMatch(mod, /syntelle\/api|pgx|cognitive-services|oauth2/)
  const docker = await readFile(join(root, 'services/render-worker/native.Dockerfile'), 'utf8')
  assert.doesNotMatch(docker, /COPY services\/api/)
  assert.match(docker, /COPY packages\/render-contract/)
})
