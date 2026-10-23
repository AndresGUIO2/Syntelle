// Legacy URL-bound adapter for archived review scripts. New consumers receive
// an explicit selection and context; shared feature code never reads the URL.
import { resolvePoseContext } from './poseCatalog'
import { useOralAssets } from '../../features/character/composables/useOralAssets'
export { resolvePoseContext, developmentCharacters } from './poseCatalog'
const params = new URLSearchParams(location.search)
const context = resolvePoseContext({
  trial: params.get('trial'),
  character: params.get('character'),
  oral: params.get('oral'),
})
export const posePackage = context.package
export const developmentCharacter = context.character
export const semanticTrial = context.semantic
export const completedTrial = context.completed
export const poseAssetUrl = context.assetUrl
export function usePosePackage() {
  return useOralAssets(context)
}
