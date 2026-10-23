import { createApp, h } from 'vue'
import { useSpeechPlayback } from '../../src/features/character/composables/useSpeechPlayback'
import type { SpeechResponse } from '../../src/features/character/speech/speechTimeline'
import type { LaughterPackage } from '../../src/features/character/types/laughterTypes'

export function mountPerformanceFixture(speech: SpeechResponse, text: string, hash: string) {
  const host = document.createElement('div')
  document.body.append(host)
  createApp({
    setup() {
      const playback = useSpeechPlayback('syntelle', undefined, 0, true, () => ({
        package: {
          version: 'registered-laughter/1',
          source_sha256: 'fixture',
          motion_build_id: 'fixture',
          oral_build_id: 'fixture',
          crop: { x: 0, y: 0, width: 1, height: 1 },
          frames: [],
          validation: { valid: true },
          review_status: 'development-composition-accepted',
          generated_dental_installed: false,
          audio: [
            {
              file: 'fixture-laugh.wav',
              sha256: hash,
              voice: speech.voice,
              language: 'en-US',
              region: 'canadacentral',
              duration_ms: 1650,
            },
          ],
        } satisfies LaughterPackage,
        assetUrl: () => '/fixture-laugh.wav',
      }))
      ;(window as any).performanceFixture = { playback, speech, text }
      return () =>
        h(
          'button',
          { onClick: () => playback.playPreparedSpeech(speech, text) },
          'Play offline sequence',
        )
    },
  }).mount(host)
}
