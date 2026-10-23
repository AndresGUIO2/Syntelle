import { createHash } from 'node:crypto'
import {
  readFileSync,
  readdirSync,
  realpathSync,
  mkdirSync,
  writeFileSync,
  existsSync,
  unlinkSync,
} from 'node:fs'
import { resolve, relative, extname, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'
import type { Plugin } from 'vite'
import {
  runtimeOral,
  runtimeMotion,
  runtimeGaze,
  runtimeExpressions,
  runtimeBackground,
} from './demo-manifests.ts'

export const webRoot = fileURLToPath(new URL('../', import.meta.url))
export const demoModes = ['demo-baseline', 'demo-protected'] as const
export type DemoMode = (typeof demoModes)[number]
const characters = ['syntelle', 'cafe', 'woodland'] as const
const catalogId = '\0syntelle:demo-catalog'
const characterPrefix = '\0syntelle:character/'
const experimentRoot = resolve(webRoot, 'src/experiments')
const evidenceRoot = resolve(webRoot, '../../.tools/client-protection-31')
const normalize = (value: string) => value.replaceAll('\\', '/')
export const isEngineModule = (id: string) =>
  /\/src\/features\/character\/(?:animation|speech|rendering|composables|types)\/[^?]+\.ts$/.test(
    normalize(id),
  )
const digest = (bytes: string | Uint8Array) => createHash('sha256').update(bytes).digest('hex')

// No injected control flow, decoding, property renaming, anti-debug timers or
// self-defending code. The fixed seed is for reproducibility, not a secret.
export const obfuscationOptions = {
  target: 'browser' as const,
  compact: true,
  seed: 20261020,
  identifierNamesGenerator: 'hexadecimal' as const,
  renameGlobals: false,
  renameProperties: false,
  controlFlowFlattening: false,
  deadCodeInjection: false,
  debugProtection: false,
  debugProtectionInterval: 0,
  disableConsoleOutput: false,
  selfDefending: false,
  numbersToExpressions: false,
  splitStrings: false,
  transformObjectKeys: false,
  unicodeEscapeSequence: false,
  stringArray: true,
  stringArrayThreshold: 0.5,
  stringArrayEncoding: [],
  stringArrayCallsTransform: false,
  stringArrayWrappersType: 'variable' as const,
  stringArrayWrappersCount: 1,
  stringArrayWrappersChainedCalls: false,
  sourceMap: false,
  advertisement: false,
}

export function assertPublicText(text: string, label: string) {
  const forbidden =
    /sourceMappingURL|sourceURL=|sourcesContent|\/@(?:vite|fs)\/|\.tools[\\/]|[A-Z]:[\\/]+(?:Users|Carpetas)|(?:\.\.\/|src\/)experiments\/|algorithm_(?:sources|files)|semantic_evidence|generated_landmarks|reference_landmarks|face_landmarker\.task/i
  const credential =
    /AIza[\w-]{35}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|(?:GEMINI_API_KEY|AZURE_SPEECH_KEY|DATABASE_URL)\s*[=:]\s*["'][^"']+["']/
  if (forbidden.test(text) || credential.test(text))
    throw Error(`Private data or development metadata in ${label}`)
}

export function distributionFiles(directory: string) {
  const files: { file: string; bytes: number; gzip_bytes: number; sha256: string }[] = []
  function scan(folder: string) {
    for (const item of readdirSync(folder, { withFileTypes: true })) {
      const full = resolve(folder, item.name)
      if (item.isDirectory()) scan(full)
      else {
        if (!item.isFile()) throw Error('Unexpected link in demo distribution')
        const file = normalize(relative(directory, full))
        const bytes = readFileSync(full)
        if (!/\.(?:js|css|html|png|webp|bin|woff2|svg|txt)$/.test(file))
          throw Error(`Unexpected demo output: ${file}`)
        if (/\.(?:js|css|html|svg|txt)$/.test(file)) assertPublicText(bytes.toString(), file)
        files.push({
          file,
          bytes: bytes.length,
          gzip_bytes: gzipSync(bytes).length,
          sha256: digest(bytes),
        })
      }
    }
  }
  scan(directory)
  return files.sort((a, b) => a.file.localeCompare(b.file, 'en'))
}

export function verifyDemoDistribution(mode: DemoMode, directory: string) {
  const report = JSON.parse(readFileSync(resolve(evidenceRoot, `${mode}.json`), 'utf8'))
  const current = distributionFiles(directory)
  if (report.mode !== mode || JSON.stringify(current) !== JSON.stringify(report.files))
    throw Error('Demo audit is missing or the distribution changed. Rebuild before previewing.')
  return current
}

export function demoBuildPlugin(mode: DemoMode): Plugin {
  const protectedBuild = mode === 'demo-protected'
  const assets = new Map<string, { reference: string; sha256: string; sources: string[] }>()
  const inputs: { file: string; sha256: string; original_bytes: number; runtime_bytes: number }[] =
    []
  const obfuscated = new Set<string>()
  let outputDirectory = ''
  const reportPath = resolve(evidenceRoot, `${mode}.json`)
  return {
    name: 'syntelle-demo-build',
    enforce: 'pre',
    configResolved(config) {
      outputDirectory = resolve(config.root, config.build.outDir)
    },
    buildStart() {
      assets.clear()
      inputs.length = 0
      obfuscated.clear()
      if (existsSync(reportPath)) unlinkSync(reportPath)
      for (const [file, source] of [
        ['brand/colors-studio.svg', 'public/brand/colors-studio.svg'],
        ['brand/colors-studio-symbol.svg', 'public/brand/colors-studio-symbol.svg'],
        ['licenses/dm-sans-OFL.txt', 'public/licenses/dm-sans-OFL.txt'],
        ['licenses/vue-MIT.txt', 'node_modules/vue/LICENSE'],
        ['licenses/pinia-MIT.txt', 'node_modules/pinia/LICENSE'],
      ])
        this.emitFile({
          type: 'asset',
          fileName: file!,
          source: readFileSync(resolve(webRoot, source!)),
        })
    },
    resolveId(id, importer) {
      if (id === 'virtual:syntelle:demo-catalog') return catalogId
      if (id.startsWith('virtual:syntelle:character/')) return '\0' + id.slice('virtual:'.length)
      if (importer && /(?:^|\/)catalog(?:\.ts)?$/.test(id)) {
        const full = resolve(importer.split('?')[0]!, '..', id)
        if (
          [
            resolve(webRoot, 'src/features/demo/catalog'),
            resolve(webRoot, 'src/features/demo/catalog.ts'),
          ].includes(full)
        )
          return catalogId
      }
    },
    load(id) {
      const emitMaterial = (folder: string, name: string, expectedHash?: string) => {
        if (
          !/^[\w./-]+$/.test(name) ||
          name.split('/').includes('..') ||
          !/\.(webp|png|bin)$/.test(name)
        )
          throw Error('Invalid prepared material path')
        const source = realpathSync(resolve(folder, name))
        const local = relative(realpathSync(folder), source)
        if (local.startsWith('..' + sep) || local === '..' || resolve(folder, local) !== source)
          throw Error('Prepared material escapes its package')
        const bytes = readFileSync(source),
          hash = digest(bytes)
        if (expectedHash && hash !== expectedHash) throw Error('Prepared material checksum differs')
        const key = hash + extname(source)
        let item = assets.get(key)
        if (!item) {
          item = {
            reference: this.emitFile({
              type: 'asset',
              name: `material${extname(source)}`,
              source: bytes,
            }),
            sha256: hash,
            sources: [],
          }
          assets.set(key, item)
        }
        const sourceName = normalize(relative(webRoot, source))
        if (!item.sources.includes(sourceName)) item.sources.push(sourceName)
        return `import.meta.ROLLUP_FILE_URL_${item.reference}`
      }
      const read = <T>(file: string, convert: (input: unknown) => T): T => {
        const bytes = readFileSync(file),
          result = convert(JSON.parse(bytes.toString()))
        const serialized = JSON.stringify(result)
        assertPublicText(serialized, 'runtime manifest')
        inputs.push({
          file: normalize(relative(webRoot, file)),
          sha256: digest(bytes),
          original_bytes: bytes.length,
          runtime_bytes: Buffer.byteLength(serialized),
        })
        return result
      }
      if (id === catalogId) {
        const backgrounds = JSON.parse(
          readFileSync(resolve(experimentRoot, 'motion/backgrounds/catalog.json'), 'utf8'),
        ).items.map(runtimeBackground)
        if (backgrounds.length !== 3)
          throw Error('The demo requires the three reviewed backgrounds')
        const backgroundUrls = backgrounds.flatMap(
          (background: { image: string; thumbnail: string }) =>
            [background.image, background.thumbnail].map(
              (name) =>
                `${JSON.stringify(name)}:${emitMaterial(resolve(experimentRoot, 'motion/backgrounds'), name)}`,
            ),
        )
        const thumbnails = characters.map(
          (character) =>
            `${JSON.stringify(character)}:${emitMaterial(resolve(experimentRoot, `viseme-a/characters/${character}`), 'base.webp')}`,
        )
        const loaders = characters.map(
          (character) => `${character}:()=>import('virtual:syntelle:character/${character}')`,
        )
        return `
          import {visibleIdleProfile} from ${JSON.stringify(normalize(resolve(webRoot, 'src/features/character/animation/idlePosture.ts')))};
          export const demoCharacters=[{id:'syntelle',name:'Syntelle',voice:'en-US-Ava:DragonHDLatestNeural'},{id:'cafe',name:'Café',voice:'en-US-Andrew:DragonHDLatestNeural'},{id:'woodland',name:'Woodland',voice:'en-US-Andrew:DragonHDLatestNeural'}];
          export const demoBackgrounds=${JSON.stringify(backgrounds)};
          const backgrounds={${backgroundUrls.join(',')}},thumbnails={${thumbnails.join(',')}},loaders={${loaders.join(',')}};
          function required(map,key){if(!Object.hasOwn(map,key))throw Error('A prepared character resource is missing.');return map[key]}
          export function demoBackgroundUrl(name){return required(backgrounds,name)}
          export function characterThumbnail(id){return required(thumbnails,id)}
          export async function loadDemoCharacter(id){
            const data=(await required(loaders,id)()).default;
            return {id,oral:Object.freeze({character:id,package:data.oral,completed:true,semantic:true,assetUrl:name=>required(data.urls,'oral/'+name)}),
              motion:Object.freeze({definition:data.motion,idle:visibleIdleProfile,attention:data.attention,conversationalAttention:new URLSearchParams(location.search).get('attention')!=='previous'&&new URLSearchParams(location.search).get('performance')!=='previous',expressions:data.expressions,assetUrl:name=>required(data.urls,'motion/'+name)})};
          }`
      }
      if (id.startsWith(characterPrefix)) {
        const character = id.slice(characterPrefix.length)
        if (!characters.includes(character as (typeof characters)[number]))
          throw Error('Unknown demo character')
        const oralRoot = resolve(experimentRoot, `viseme-a/characters/${character}`),
          motionRoot = resolve(experimentRoot, `motion/rgbd/${character}`),
          gazeRoot = resolve(experimentRoot, `motion/gaze/${character}`),
          expressionRoot = resolve(experimentRoot, `motion/expressions/${character}`)
        const oral = read(resolve(oralRoot, 'package.json'), runtimeOral),
          motion = read(resolve(motionRoot, 'package.json'), runtimeMotion),
          attention = read(resolve(gazeRoot, 'gaze.json'), runtimeGaze),
          expressions = read(resolve(expressionRoot, 'expressions.json'), runtimeExpressions)
        if (
          !motion.projection ||
          motion.oral_build_id !== oral.build_id ||
          attention.motion_build_id !== motion.build_id ||
          expressions.motion_build_id !== motion.build_id ||
          expressions.oral_build_id !== oral.build_id ||
          [
            attention.source_sha256,
            expressions.source_sha256,
            motion.projection.source_sha256,
          ].some((hash) => hash !== motion.original_sha256)
        )
          throw Error('Demo preparation bindings differ')
        const urls = new Map<string, string>()
        const add = (prefix: string, folder: string, name: string, hash?: string) => {
          const key = `${prefix}/${name}`,
            url = emitMaterial(folder, name, hash)
          if (urls.has(key) && urls.get(key) !== url) throw Error('Conflicting prepared material')
          urls.set(key, url)
        }
        for (const name of ['base.webp', oral.reference.upper, oral.reference.lower])
          add('oral', oralRoot, name)
        for (const pose of Object.values(oral.poses))
          for (const name of [pose.endpoint, pose.surface, pose.mask, pose.cavity])
            if (name) add('oral', oralRoot, name)
        for (const name of [
          motion.foreground,
          motion.depth,
          motion.eyes.open,
          motion.eyes.closed,
          motion.eyes.mask,
        ])
          add('motion', resolve(experimentRoot, `motion/assets/${character}`), name)
        add(
          'motion',
          motionRoot,
          motion.projection.geometry.file,
          motion.projection.geometry.sha256,
        )
        for (const asset of Object.values(attention.assets))
          add('motion', gazeRoot, asset.file, asset.sha256)
        add('motion', expressionRoot, expressions.field.file, expressions.field.sha256)
        if (expressions.smile)
          add(
            'motion',
            expressionRoot,
            expressions.smile.material.file,
            expressions.smile.material.sha256,
          )
        return `export default {oral:${JSON.stringify(oral)},motion:${JSON.stringify(motion)},attention:${JSON.stringify(attention)},expressions:${JSON.stringify(expressions)},urls:{${[...urls].map(([key, url]) => `${JSON.stringify(key)}:${url}`).join(',')}}};`
      }
    },
    renderChunk: {
      order: 'post',
      async handler(code, chunk) {
        const engine = chunk.moduleIds.filter(isEngineModule)
        if (!engine.length) return
        if (
          chunk.moduleIds.some((id) =>
            /node_modules|\.vue(?:\?|$)|capture\.worklet/.test(normalize(id)),
          )
        )
          throw Error('Character engine chunk includes excluded code')
        if (!protectedBuild) return
        // Normal development and validation-only builds do not load the new
        // build tool, including already-running development containers.
        const { default: JavaScriptObfuscator } = await import('javascript-obfuscator')
        const result = JavaScriptObfuscator.obfuscate(code, obfuscationOptions).getObfuscatedCode()
        if (!result || result === code) throw Error('Character engine was not obfuscated')
        engine.forEach((id) => obfuscated.add(id))
        return { code: result, map: null }
      },
    },
    writeBundle(_options, bundle) {
      const chunks = Object.values(bundle).filter((item) => item.type === 'chunk')
      const modules = chunks.flatMap((chunk) => chunk.moduleIds)
      if (
        modules.some((id) =>
          /\/src\/experiments\/|\/(?:docs|services|\.tools)\//.test(normalize(id)),
        )
      )
        throw Error('Private or laboratory module entered the demo')
      const engine = [...new Set(modules.filter(isEngineModule))].sort()
      if (!engine.length || (protectedBuild && engine.some((id) => !obfuscated.has(id))))
        throw Error('Expected character engine modules bypassed protection')
      const materialReport = [...assets.values()].map((item) => {
        const file = this.getFileName(item.reference)
        if (digest(readFileSync(resolve(outputDirectory, file))) !== item.sha256)
          throw Error('Emitted prepared material differs from its source')
        return { file, sha256: item.sha256, sources: item.sources.sort() }
      })
      for (const input of inputs)
        if (digest(readFileSync(resolve(webRoot, input.file))) !== input.sha256)
          throw Error('Preparation manifest changed during the build')
      const files = distributionFiles(outputDirectory)
      const materialNames = new Set(materialReport.map((item) => item.file))
      const publicSources: Record<string, string> = {
        'brand/colors-studio.svg': 'public/brand/colors-studio.svg',
        'brand/colors-studio-symbol.svg': 'public/brand/colors-studio-symbol.svg',
        'licenses/dm-sans-OFL.txt': 'public/licenses/dm-sans-OFL.txt',
        'licenses/vue-MIT.txt': 'node_modules/vue/LICENSE',
        'licenses/pinia-MIT.txt': 'node_modules/pinia/LICENSE',
      }
      const fontRoot = resolve(webRoot, 'node_modules/@fontsource-variable/dm-sans/files')
      const fontHashes = new Set(
        readdirSync(fontRoot)
          .filter((name) => name.endsWith('.woff2'))
          .map((name) => digest(readFileSync(resolve(fontRoot, name)))),
      )
      const chunkNames = new Set(chunks.map((chunk) => chunk.fileName))
      for (const file of files) {
        if (materialNames.has(file.file) || chunkNames.has(file.file)) continue
        if (publicSources[file.file]) {
          if (digest(readFileSync(resolve(webRoot, publicSources[file.file]))) !== file.sha256)
            throw Error('A public notice or brand asset changed during emission')
        } else if (file.file.endsWith('.woff2') && fontHashes.has(file.sha256)) continue
        else if (
          file.file === 'index.html' ||
          /^assets\/[\w-]+\.css$/.test(file.file) ||
          /^assets\/capture\.worklet-[\w-]+\.js$/.test(file.file)
        )
          continue
        else throw Error(`Resource outside the demo allowlist: ${file.file}`)
      }
      mkdirSync(evidenceRoot, { recursive: true })
      writeFileSync(
        reportPath,
        JSON.stringify(
          {
            mode,
            obfuscator: protectedBuild ? { version: '5.9.0', options: obfuscationOptions } : null,
            engine_modules: engine.map((id) => normalize(relative(webRoot, id))),
            chunks: chunks.map((chunk) => ({
              file: chunk.fileName,
              modules: chunk.moduleIds.map((id) => normalize(relative(webRoot, id))),
            })),
            inputs,
            materials: materialReport,
            files,
          },
          null,
          2,
        ),
      )
    },
    configurePreviewServer(server) {
      if (
        !['127.0.0.1', 'localhost', '::1'].includes(String(server.config.preview.host)) ||
        !server.config.preview.strictPort
      )
        throw Error('Demo preview requires a loopback host and strict port')
      verifyDemoDistribution(mode, resolve(server.config.root, server.config.build.outDir))
    },
  }
}
