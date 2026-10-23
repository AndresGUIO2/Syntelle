import type { OralContext, MotionContext } from '../character/types/packageContext'
import type { PosePackage } from '../character/types/oralTypes'
import type { MotionPackage, Background } from '../character/types/motionTypes'
import backgrounds from '../../experiments/motion/backgrounds/catalog.json'
import { visibleIdleProfile } from '../character/animation/idlePosture'
import type { GazePackage } from '../character/types/gazeTypes'
import type { ExpressionPackage } from '../character/types/expressionTypes'

export const demoCharacters = [
  { id: 'syntelle', name: 'Syntelle', voice: 'en-US-Ava:DragonHDLatestNeural' },
  { id: 'cafe', name: 'Café', voice: 'en-US-Andrew:DragonHDLatestNeural' },
  { id: 'woodland', name: 'Woodland', voice: 'en-US-Andrew:DragonHDLatestNeural' },
] as const
export type DemoCharacterId = (typeof demoCharacters)[number]['id']
export interface PreparedDemoCharacter {
  id: DemoCharacterId
  oral: OralContext
  motion: MotionContext
}

// Only the current three packages and runtime materials enter this catalog.
// Review overlays, raw generations and historical movement packages stay in the lab.
const oralPackages = import.meta.glob<PosePackage>(
  '../../experiments/viseme-a/characters/{syntelle,cafe,woodland}/package.json',
  { import: 'default' },
)
const motionPackages = import.meta.glob<MotionPackage>(
  '../../experiments/motion/rgbd/{syntelle,cafe,woodland}/package.json',
  { import: 'default' },
)
const gazePackages = import.meta.glob<GazePackage>(
  '../../experiments/motion/gaze/{syntelle,cafe,woodland}/gaze.json',
  { import: 'default' },
)
const expressionPackages = import.meta.glob<ExpressionPackage>(
  '../../experiments/motion/expressions/{syntelle,cafe,woodland}/expressions.json',
  { import: 'default' },
)
const expressionUrls = import.meta.glob<string>(
  '../../experiments/motion/expressions/{syntelle,cafe,woodland}/*.png',
  { eager: true, query: '?url', import: 'default' },
)
const gazeUrls = import.meta.glob<string>(
  '../../experiments/motion/gaze/{syntelle,cafe,woodland}/*.png',
  { eager: true, query: '?url', import: 'default' },
)
const oralUrls = import.meta.glob<string>(
  [
    '../../experiments/viseme-a/characters/{syntelle,cafe,woodland}/{base,upper-teeth,lower-teeth}.webp',
    '../../experiments/viseme-a/characters/{syntelle,cafe,woodland}/*/{surface,cavity}.webp',
    '../../experiments/viseme-a/characters/{syntelle,cafe,woodland}/*/alpha.png',
    '../../experiments/viseme-a/characters/{syntelle,cafe,woodland}/*/endpoint.png',
  ],
  { eager: true, query: '?url', import: 'default' },
)
const motionUrls = import.meta.glob<string>(
  [
    '../../experiments/motion/assets/{syntelle,cafe,woodland}/**/*.{webp,png}',
    '../../experiments/motion/rgbd/{syntelle,cafe,woodland}/*.bin',
  ],
  { eager: true, query: '?url', import: 'default' },
)
const backgroundUrls = import.meta.glob<string>('../../experiments/motion/backgrounds/*.webp', {
  eager: true,
  query: '?url',
  import: 'default',
})
export const demoBackgrounds = backgrounds.items as Background[]
export function demoBackgroundUrl(name: string) {
  return required(backgroundUrls, `../../experiments/motion/backgrounds/${name}`)
}
function required(urls: Record<string, string>, path: string) {
  const url = urls[path]
  if (!url) throw Error('A prepared character resource is missing.')
  return url
}
export function characterThumbnail(id: DemoCharacterId) {
  return required(oralUrls, `../../experiments/viseme-a/characters/${id}/base.webp`)
}
export async function loadDemoCharacter(
  id: DemoCharacterId,
  idle: 'attention' | 'previous' = 'attention',
  expressionsEnabled = true,
): Promise<PreparedDemoCharacter> {
  const oralPrefix = `../../experiments/viseme-a/characters/${id}/`
  const motionPrefix = `../../experiments/motion/rgbd/${id}/`
  const [oral, motion] = await Promise.all([
    oralPackages[`${oralPrefix}package.json`]!(),
    motionPackages[`${motionPrefix}package.json`]!(),
  ])
  const gazePrefix = `../../experiments/motion/gaze/${id}/`
  const attention =
    idle === 'attention' ? await gazePackages[`${gazePrefix}gaze.json`]!() : undefined
  const expressionPrefix = `../../experiments/motion/expressions/${id}/`
  const expressions = expressionsEnabled
    ? await expressionPackages[`${expressionPrefix}expressions.json`]!()
    : undefined
  return {
    id,
    oral: Object.freeze({
      character: id,
      package: oral,
      completed: true,
      semantic: true,
      assetUrl: (name: string) => required(oralUrls, oralPrefix + name),
    }),
    motion: Object.freeze({
      expressive: new URLSearchParams(location.search).get('expressive') !== 'previous',
      performance: new URLSearchParams(location.search).get('performance') !== 'previous',
      conversationalAttention:
        new URLSearchParams(location.search).get('attention') !== 'previous' &&
        new URLSearchParams(location.search).get('performance') !== 'previous',
      definition: motion,
      idle: visibleIdleProfile,
      attention,
      expressions,
      assetUrl: (name: string) =>
        name === 'field.png' || name === 'smile-material.png'
          ? required(expressionUrls, expressionPrefix + name)
          : name === 'sclera.png' || name === 'ownership.png'
            ? required(gazeUrls, gazePrefix + name)
            : required(
                motionUrls,
                name === motion.projection?.geometry.file
                  ? motionPrefix + name
                  : `../../experiments/motion/assets/${id}/${name}`,
              ),
    }),
  }
}
