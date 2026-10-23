import { fileURLToPath } from 'node:url'

// Old browser review scripts import these exact development URLs.
// Live application code uses the canonical feature folders directly.
const paths = {
  headMotion: 'animation/headMotion',
  timing: 'animation/timing',
  speechBehavior: 'animation/speechBehavior',
  speechTimeline: 'speech/speechTimeline',
  motionTypes: 'types/motionTypes',
  oralTypes: 'types/oralTypes',
  packageContext: 'types/packageContext',
}

export const legacyCharacterModuleAliases = Object.entries(paths).map(([name, target]) => ({
  find: `/src/features/character/${name}.ts`,
  replacement: fileURLToPath(new URL(`../src/features/character/${target}.ts`, import.meta.url)),
}))
