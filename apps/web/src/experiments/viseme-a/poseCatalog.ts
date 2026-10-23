import rawPackage from './poses/package.json'
import completedPackage from './completed/package.json'
import semanticPackage from './semantic/package.json'
import type { PosePackage } from '../../features/character/types/oralTypes'
import type { OralContext, TrialSelection } from '../../features/character/types/packageContext'

const assetUrls = import.meta.glob<string>('./poses/**/*.{webp,png}', {
  eager: true,
  query: '?url',
  import: 'default',
})
const completedUrls = import.meta.glob<string>('./completed/**/*.{webp,png}', {
  eager: true,
  query: '?url',
  import: 'default',
})
const semanticUrls = import.meta.glob<string>('./semantic/**/*.{webp,png}', {
  eager: true,
  query: '?url',
  import: 'default',
})
// Local corpus inspection reuses the trial; preparation remains private/offline.
const developmentPackages = import.meta.env.DEV
  ? import.meta.glob('./characters/*/package.json', { eager: true, import: 'default' })
  : {}
const developmentUrls = import.meta.env.DEV
  ? import.meta.glob<string>('./characters/**/*.{webp,png}', {
      eager: true,
      query: '?url',
      import: 'default',
    })
  : {}
export const developmentCharacters = Object.keys(developmentPackages).map(
  (key) => key.split('/')[2]!,
)

export function resolvePoseContext(selection: TrialSelection = {}): OralContext {
  const requestedCharacter =
    selection.character ?? (selection.trial === 'motion' ? 'syntelle' : null)
  const character =
    requestedCharacter && developmentPackages[`./characters/${requestedCharacter}/package.json`]
      ? requestedCharacter
      : null
  const semantic =
    Boolean(character) || (selection.oral !== 'completed' && selection.oral !== 'previous')
  const completed = Boolean(character) || selection.oral !== 'previous'
  const manifest = (
    character
      ? developmentPackages[`./characters/${character}/package.json`]
      : semantic
        ? semanticPackage
        : completed
          ? completedPackage
          : rawPackage
  ) as PosePackage
  const prefix = character
    ? `./characters/${character}/`
    : semantic
      ? './semantic/'
      : completed
        ? './completed/'
        : './poses/'
  const urls = character
    ? developmentUrls
    : semantic
      ? semanticUrls
      : completed
        ? completedUrls
        : assetUrls
  return Object.freeze({
    package: manifest,
    character,
    semantic,
    completed,
    assetUrl(name: string) {
      const url = urls[prefix + name]
      if (!url) throw new Error('Missing sample resource')
      return `${url}${url.includes('?') ? '&' : '?'}v=${manifest.build_id}`
    },
  })
}
