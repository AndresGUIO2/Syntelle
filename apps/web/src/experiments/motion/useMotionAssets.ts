// Compatibility boundary for archived review scripts.
export * from './motionCatalog'
import { resolveMotionContext } from './motionCatalog'
import { useMotionAssets as useAssets } from '../../features/character/composables/useMotionAssets'
export function useMotionAssets(character: string, oralBuild: string) {
  return useAssets(resolveMotionContext(character), oralBuild)
}
