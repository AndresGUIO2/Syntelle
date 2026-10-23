import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import JavaScriptObfuscator from 'javascript-obfuscator'
import {
  runtimeOral,
  runtimeMotion,
  runtimeGaze,
  runtimeExpressions,
} from '../../config/demo-manifests.ts'
import {
  assertPublicText,
  isEngineModule,
  obfuscationOptions,
  webRoot,
} from '../../config/demo-build.ts'
import { decodeProjection } from '../../src/features/character/types/projectionTypes.ts'

const read = (file: string) =>
  JSON.parse(readFileSync(resolve(webRoot, 'src/experiments', file), 'utf8'))

for (const character of ['syntelle', 'cafe', 'woodland']) {
  test(`${character}: runtime projections retain geometry and bindings, omit preparation evidence`, () => {
    const sourceOral = read(`viseme-a/characters/${character}/package.json`)
    sourceOral.private_future_field = { prompt: 'PRIVATE_SENTINEL' }
    sourceOral.poses.EI.private_future_field = 'PRIVATE_SENTINEL'
    const oral = runtimeOral(sourceOral)
    assertPublicText(JSON.stringify(oral), 'oral projection')
    assert.equal(JSON.stringify(oral).includes('PRIVATE_SENTINEL'), false)
    assert.equal('points' in oral.poses.EI!, false)
    assert.equal('comparison' in oral.poses.EI!, false)
    for (const [key, pair] of Object.entries(oral.pairs)) {
      assert.equal(pair.safe, sourceOral.pairs[key].safe)
      if (pair.mesh) {
        for (const field of ['rest', 'open', 'triangles', 'cavity_triangles'] as const)
          assert.deepEqual(pair.mesh[field], sourceOral.pairs[key].mesh[field])
        assert.equal('landmark_ids' in pair.mesh, false)
      }
    }
    const sourceMotion = read(`motion/rgbd/${character}/package.json`)
    sourceMotion.projection.calibration.private_future_field = 'PRIVATE_SENTINEL'
    const motion = runtimeMotion(sourceMotion)
    assert.equal('calibration' in motion.projection!, false)
    assert.equal('algorithm_files' in motion, false)
    assert.deepEqual(motion.eyes.curves, sourceMotion.eyes.curves)
    assert.deepEqual(motion.rig!.vertices, sourceMotion.rig.vertices)
    assert.equal(motion.oral_build_id, oral.build_id)
    assert.deepEqual(motion.projection!.geometry, sourceMotion.projection.geometry)
    const gaze = runtimeGaze(read(`motion/gaze/${character}/gaze.json`))
    assert.equal(gaze.motion_build_id, motion.build_id)
    assert.equal(gaze.source_sha256, motion.original_sha256)
    const expressions = runtimeExpressions(read(`motion/expressions/${character}/expressions.json`))
    assert.equal(expressions.oral_build_id, oral.build_id)
    assert.equal(expressions.motion_build_id, motion.build_id)
    if (expressions.smile) assert.equal('generated_landmarks' in expressions.smile, false)
    const bytes = readFileSync(
      resolve(
        webRoot,
        `src/experiments/motion/rgbd/${character}`,
        motion.projection!.geometry.file,
      ),
    )
    const buffer = Uint8Array.from(bytes).buffer
    decodeProjection(buffer, motion.projection!)
    new DataView(buffer).setFloat32(0, NaN, true)
    assert.throws(
      () => decodeProjection(buffer, motion.projection!),
      /Invalid prepared projection coordinates/,
    )
  })
}

test('missing checksums and nonfinite coordinates fail; failed validation is preserved', () => {
  const gaze = read('motion/gaze/syntelle/gaze.json')
  delete gaze.assets['sclera.png'].sha256
  assert.throws(() => runtimeGaze(gaze), /Missing runtime manifest field/)
  const oral = read('viseme-a/characters/syntelle/package.json')
  oral.pairs['EI-FV'].mesh.rest[0][0] = Infinity
  assert.throws(() => runtimeOral(oral), /Invalid runtime manifest field/)
  const motion = read('motion/rgbd/syntelle/package.json')
  motion.projection.validation.valid = false
  assert.equal(runtimeMotion(motion).projection!.validation.valid, false)
  const expressions = read('motion/expressions/syntelle/expressions.json')
  expressions.motion_build_id = 'MISMATCH'
  assert.equal(runtimeExpressions(expressions).motion_build_id, 'MISMATCH')
})

test('private text audit and engine selection reject development code and Vue script queries', () => {
  for (const text of [
    '//# sourceMappingURL=source.map',
    '.tools/private.json',
    'E:\\Carpetas\\private',
    '../experiments/old/package.json',
    'algorithm_sources',
    '-----BEGIN PRIVATE KEY-----',
  ])
    assert.throws(() => assertPublicText(text, 'fixture'), /Private data/)
  assertPublicText('Prepared material checksum differs', 'public error')
  assert.equal(isEngineModule('/project/src/features/character/rendering/gl.ts'), true)
  assert.equal(
    isEngineModule('E:\\project\\src\\features\\character\\animation\\attention.ts'),
    true,
  )
  for (const id of [
    '/src/features/character/components/CharacterStage.vue?vue&type=script&lang.ts',
    '/node_modules/vue/index.js',
    '/src/features/conversation/audio/capture.worklet.ts',
  ])
    assert.equal(isEngineModule(id), false)
})

test('conservative obfuscation is deterministic and preserves ESM exports and property names', async () => {
  const code =
    'export function evaluate(input){const label="coordinate";return {label,value:input.value+2}}'
  const first = JavaScriptObfuscator.obfuscate(code, obfuscationOptions).getObfuscatedCode()
  assert.equal(first, JavaScriptObfuscator.obfuscate(code, obfuscationOptions).getObfuscatedCode())
  assert.notEqual(first, code)
  const module = await import(
    `data:text/javascript;base64,${Buffer.from(first).toString('base64')}`
  )
  assert.deepEqual(module.evaluate({ value: 4 }), { label: 'coordinate', value: 6 })
})
