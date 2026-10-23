import type { MotionPackage, Background } from './motionTypes'
import catalog from './backgrounds/catalog.json'
import type { MotionContext } from '../../features/character/types/packageContext'

const packages = import.meta.glob('./assets/*/package.json', { eager: true, import: 'default' })
const naturalPackages = import.meta.glob('./natural/*/package.json', {
  eager: true,
  import: 'default',
})
const previousNaturalPackages = import.meta.glob('./natural-previous/*/package.json', {
  eager: true,
  import: 'default',
})
const rotationPackages = import.meta.glob('./natural-rotation/*/package.json', {
  eager: true,
  import: 'default',
})
const contextualPackages = import.meta.glob('./contextual/*/package.json', {
  eager: true,
  import: 'default',
})
const vitalityPackages = import.meta.glob('./vitality/*/package.json', {
  eager: true,
  import: 'default',
})
const projectionPackages = import.meta.glob('./rgbd/*/package.json', {
  eager: true,
  import: 'default',
})
const neckPackages = import.meta.glob('./neck/*/package.json', { eager: true, import: 'default' })
const neckUrls = import.meta.glob<string>('./neck/*/*.bin', {
  eager: true,
  query: '?url',
  import: 'default',
})
const projectionUrls = import.meta.glob<string>('./rgbd/*/*.bin', {
  eager: true,
  query: '?url',
  import: 'default',
})
const urls = import.meta.glob<string>('./assets/**/*.{webp,png}', {
  eager: true,
  query: '?url',
  import: 'default',
})
const backgrounds = import.meta.glob<string>('./backgrounds/*.webp', {
  eager: true,
  query: '?url',
  import: 'default',
})
export const backgroundCatalog = catalog.items as Background[]
export const backgroundUrl = (name: string) => backgrounds[`./backgrounds/${name}`]!
export const motionCharacters = Object.keys(packages).map((name) => name.split('/')[2]!)
export function motionDefinition(character: string) {
  return packages[`./assets/${character}/package.json`] as MotionPackage | undefined
}
export function motionAssetUrl(character: string, name: string) {
  const url = urls[`./assets/${character}/${name}`]
  if (!url) throw Error('Missing prepared motion resource')
  return `${url}${url.includes('?') ? '&' : '?'}v=${motionDefinition(character)?.build_id}`
}
export function resolveMotionContext(
  character: string,
  natural = false,
  variant:
    'current' | 'previous' | 'rotation' | 'contextual' | 'vitality' | 'rgbd' | 'neck' = 'current',
): MotionContext {
  const definition = (
    natural
      ? variant === 'neck'
        ? neckPackages[`./neck/${character}/package.json`]
        : variant === 'rgbd'
          ? projectionPackages[`./rgbd/${character}/package.json`]
          : variant === 'vitality'
            ? vitalityPackages[`./vitality/${character}/package.json`]
            : variant === 'contextual'
              ? contextualPackages[`./contextual/${character}/package.json`]
              : variant === 'rotation'
                ? rotationPackages[`./natural-rotation/${character}/package.json`]
                : variant === 'previous'
                  ? previousNaturalPackages[`./natural-previous/${character}/package.json`]
                  : naturalPackages[`./natural/${character}/package.json`]
      : motionDefinition(character)
  ) as MotionPackage | undefined
  return Object.freeze({
    definition,
    assetUrl: (name: string) =>
      (definition?.projection?.articulation
        ? neckUrls[`./neck/${character}/${name}`]
        : projectionUrls[`./rgbd/${character}/${name}`]) ?? motionAssetUrl(character, name),
  })
}
