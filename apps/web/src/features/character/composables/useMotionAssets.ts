import type { MotionContext } from '../types/packageContext'
import { usePreparedImages } from './usePreparedImages'
import { onScopeDispose } from 'vue'
import { decodeProjection } from '../types/projectionTypes'
import type { ReferenceLaughterImages } from '../types/expressionTypes'

export function useMotionAssets(context: MotionContext, oralBuild: string) {
  const definition = context.definition
  const controller = new AbortController()
  onScopeDispose(() => controller.abort())
  const resources = usePreparedImages(
    async (loadImage) => {
      if (!definition)
        throw new Error('Motion preparation for this character is not available yet.')
      if (definition.oral_build_id !== oralBuild)
        throw new Error('The mouth and movement resources belong to different builds.')
      const load = (name: string) => loadImage(context.assetUrl(name))
      const [foreground, depth, opened, closed, mask] = await Promise.all([
        load(definition.foreground),
        load(definition.depth),
        load(definition.eyes.open),
        load(definition.eyes.closed),
        load(definition.eyes.mask),
      ])
      const checked = async (asset: { file: string; sha256: string }) => {
        const response = await fetch(context.assetUrl(asset.file), { signal: controller.signal })
        if (!response.ok) throw Error('Prepared facial material is unavailable')
        const bytes = await response.arrayBuffer()
        const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), (b) =>
          b.toString(16).padStart(2, '0'),
        ).join('')
        if (hash !== asset.sha256) throw Error('Prepared facial material checksum differs')
        const url = URL.createObjectURL(new Blob([bytes], { type: 'image/png' }))
        try {
          return await loadImage(url)
        } finally {
          URL.revokeObjectURL(url)
        }
      }
      let expressionField, smileMaterial
      let referenceLaughter: ReferenceLaughterImages | undefined
      if (context.expressions) {
        const pkg = context.expressions
        if (
          !['local-expressions/1', 'local-expressions/2'].includes(pkg.version) ||
          !pkg.validation.valid ||
          pkg.inverse_iterations !== 5 ||
          pkg.source_sha256 !== definition.original_sha256 ||
          pkg.motion_build_id !== definition.build_id ||
          pkg.oral_build_id !== oralBuild ||
          !Number.isFinite(pkg.units_per_byte) ||
          pkg.units_per_byte <= 0
        )
          throw Error('Expression resources do not match the character preparation.')
        expressionField = await checked(pkg.field)
        if (
          expressionField.width !== pkg.field_size[0] ||
          expressionField.height !== pkg.field_size[1]
        )
          throw Error('Expression field has incompatible dimensions')
        if (pkg.smile) {
          if (
            pkg.version !== 'local-expressions/2' ||
            pkg.smile.version !== 'registered-closed-smile/1' ||
            !pkg.smile.closed_mouth_only
          )
            throw Error('Unsupported smile preparation')
          smileMaterial = await checked(pkg.smile.material)
          if (smileMaterial.width !== pkg.crop.width || smileMaterial.height !== pkg.crop.height)
            throw Error('Smile material has incompatible dimensions')
        }
        const reference = pkg.reference_laughter
        if (reference) {
          if (
            reference.version !== 'reference-laughter/1' ||
            !reference.validation.valid ||
            !reference.dental_ready ||
            reference.source_sha256 !== definition.original_sha256 ||
            reference.motion_build_id !== definition.build_id ||
            reference.oral_build_id !== oralBuild ||
            reference.frames.length !== 25 ||
            reference.eyes.profiles.length !== 25 ||
            JSON.stringify(reference.eyes.crop) !== JSON.stringify(definition.eyes.crop) ||
            reference.oral.inner.length !== 25 ||
            reference.oral.outer.length !== 25 ||
            reference.oral.lower_translation.length !== 25 ||
            !Number.isFinite(reference.oral.jaw_limit_px) ||
            reference.oral.jaw_limit_px <= 0
          )
            throw Error('Reference laughter does not match the prepared character.')
          const [frames, cavity, upper, lower] = await Promise.all([
            Promise.all(reference.frames.map(checked)),
            checked(reference.oral.cavity),
            reference.oral.observed_upper ? checked(reference.oral.observed_upper) : undefined,
            reference.oral.observed_lower ? checked(reference.oral.observed_lower) : undefined,
          ])
          if (
            frames.some(
              (image) =>
                image.width !== reference.crop.width || image.height !== reference.crop.height,
            )
          )
            throw Error('Reference laughter facial material has incompatible dimensions')
          referenceLaughter = { frames, cavity, upper, lower }
        }
      }
      let gaze
      if (context.attention) {
        const pkg = context.attention
        if (
          pkg.version !== 'observed-iris/1' ||
          !pkg.validation.valid ||
          pkg.source_sha256 !== definition.original_sha256 ||
          pkg.motion_build_id !== definition.build_id ||
          JSON.stringify(pkg.crop) !== JSON.stringify(definition.eyes.crop)
        )
          throw Error('Gaze resources do not match the character preparation.')
        const [sclera, ownership] = await Promise.all([
          checked(pkg.assets['sclera.png']),
          checked(pkg.assets['ownership.png']),
        ])
        if (
          [sclera, ownership].some(
            (image) => image.width !== opened.width || image.height !== opened.height,
          )
        )
          throw Error('Gaze material has incompatible dimensions')
        gaze = { sclera, ownership }
      }
      let laughter: HTMLImageElement[] | undefined
      if (context.laughter) {
        const pkg = context.laughter
        // Optional direction resources must never prevent valid speech or the
        // preceding character package from loading.
        if (
          pkg.version === 'registered-laughter/1' &&
          pkg.review_status === 'development-composition-accepted' &&
          pkg.validation.valid &&
          pkg.source_sha256 === definition.original_sha256 &&
          pkg.motion_build_id === definition.build_id &&
          pkg.oral_build_id === oralBuild &&
          pkg.generated_dental_installed === false &&
          pkg.frames.length === 25
        ) {
          try {
            laughter = await Promise.all(pkg.frames.map(checked))
          } catch {
            laughter = undefined
          }
        }
      }
      let geometry: ArrayBuffer | undefined
      if (definition.projection) {
        const rig = definition.projection
        if (rig.source_sha256 !== definition.original_sha256 || !rig.validation.valid)
          throw Error('Projection package belongs to an invalid source')
        const response = await fetch(context.assetUrl(rig.geometry.file), {
          signal: controller.signal,
        })
        if (!response.ok) throw Error('Prepared projection geometry is unavailable')
        geometry = await response.arrayBuffer()
        const digest = await crypto.subtle.digest('SHA-256', geometry)
        const hash = Array.from(new Uint8Array(digest), (b) =>
          b.toString(16).padStart(2, '0'),
        ).join('')
        if (hash !== rig.geometry.sha256) throw Error('Prepared projection checksum differs')
        decodeProjection(geometry, rig)
      }
      return {
        foreground,
        depth,
        opened,
        closed,
        mask,
        geometry,
        gaze,
        expressionField,
        smileMaterial,
        laughter,
        referenceLaughter,
      }
    },
    (failure) => (failure instanceof Error ? failure.message : 'Motion resources could not load.'),
  )
  return {
    definition,
    attention: context.attention,
    expressions: context.expressions,
    laughter: context.laughter,
    ...resources,
  }
}
