import { shallowRef } from 'vue'
import { createExpressionDirector } from '../animation/expressions'

export function useExpressions(
  available: boolean,
  seed: number,
  smileAvailable = false,
  directed = false,
  expressive = false,
) {
  const smile = shallowRef(0),
    brows = shallowRef(0),
    automatic = shallowRef(true)
  const director = createExpressionDirector(seed, smileAvailable, directed, expressive)
  return { available, smileAvailable, smile, brows, automatic, sample: director.sample }
}
