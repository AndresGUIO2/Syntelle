<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'

const { t } = useLocaleStore()
defineEmits<{ navigate: [event: MouseEvent] }>()

const previewSteps = ['Portrait', 'Character', 'Conversation'] as const
</script>

<template>
  <section class="character-creation-page" aria-labelledby="creation-title">
    <a class="creation-back" href="/" @click="$emit('navigate', $event)">
      <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
        <path d="M12.75 4.5 7.25 10l5.5 5.5M7.75 10h9" />
      </svg>
      {{ t('Back to Syntelle') }}
    </a>

    <div class="creation-layout">
      <div class="creation-copy">
        <p class="coming-soon">
          <span class="status-mark" aria-hidden="true"></span>{{ t('Coming soon') }}
        </p>
        <h1 id="creation-title">{{ t('Create a character') }}</h1>
        <p class="creation-description">
          {{
            t(
              'We’re shaping a simple, guided way to turn a portrait into a character for Syntelle.',
            )
          }}
        </p>
        <div class="creation-note">
          <span class="note-icon" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none">
              <path d="M4 5.75A1.75 1.75 0 0 1 5.75 4h12.5A1.75 1.75 0 0 1 20 5.75v12.5A1.75 1.75 0 0 1 18.25 20H5.75A1.75 1.75 0 0 1 4 18.25V5.75Z" />
              <circle cx="9" cy="9" r="1.5" />
              <path d="m5 17 4.5-4.5 3 3 2-2L19 18" />
            </svg>
          </span>
          <p>{{ t('Character creation is not available yet.') }}</p>
        </div>
        <a class="button button-primary return-button" href="/" @click="$emit('navigate', $event)">
          {{ t('Return to Syntelle') }}
          <span aria-hidden="true">→</span>
        </a>
      </div>

      <aside class="creation-preview" :aria-label="t('Character preview')">
        <div class="preview-heading">
          <h2>{{ t('From portrait to character') }}</h2>
          <span>{{ t('Preview') }}</span>
        </div>
        <div class="portrait-canvas" aria-hidden="true">
          <svg viewBox="0 0 520 470" preserveAspectRatio="xMidYMid meet">
            <rect x="1" y="1" width="518" height="468" rx="14" class="canvas-background" />
            <path
              d="M107 469V218c0-83 67-150 150-150s150 67 150 150v251"
              class="portrait-arch"
            />
            <circle cx="257" cy="188" r="68" class="portrait-head" />
            <path
              d="M143 469c7-82 56-144 114-144s107 62 114 144"
              class="portrait-shoulders"
            />
            <path d="M216 117c18-21 60-24 82-2" class="portrait-detail" />
            <path d="M65 116h78M65 130h44M377 350h78M411 364h44" class="canvas-lines" />
            <circle cx="65" cy="116" r="4" class="canvas-dot" />
            <circle cx="455" cy="350" r="4" class="canvas-dot" />
          </svg>
        </div>
        <ol class="preview-steps">
          <li v-for="(step, index) in previewSteps" :key="step">
            <span class="step-number">0{{ index + 1 }}</span>
            <span>{{ t(step) }}</span>
          </li>
        </ol>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.character-creation-page {
  width: 100%;
  max-width: 1120px;
  min-height: 100%;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
}

.creation-back {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  gap: 8px;
  min-height: 44px;
  color: var(--muted);
  font-size: 14px;
  text-decoration: none;
}

.creation-back svg {
  width: 18px;
  height: 18px;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.creation-layout {
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 0.92fr) minmax(380px, 1fr);
  align-items: center;
  gap: clamp(44px, 7vw, 88px);
  padding: clamp(34px, 6vh, 68px) clamp(8px, 2vw, 24px);
}

.creation-copy {
  max-width: 490px;
}

.coming-soon {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 20px;
  padding: 6px 10px;
  border: 1px solid var(--divider);
  border-radius: 7px;
  background: var(--surface);
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.status-mark {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--accent);
}

.creation-copy h1 {
  max-width: 10ch;
  margin: 0;
  font-size: clamp(40px, 5vw, 58px);
  font-weight: 600;
  line-height: 1.06;
  letter-spacing: -0.045em;
  text-wrap: balance;
}

.creation-description {
  max-width: 46ch;
  margin: 22px 0 0;
  color: var(--muted);
  font-size: 16px;
  line-height: 1.7;
}

.creation-note {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 27px;
  padding-top: 18px;
  border-top: 1px solid var(--divider);
}

.note-icon {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--divider);
  border-radius: 10px;
  color: var(--accent);
  background: var(--surface);
}

.note-icon svg {
  width: 20px;
  height: 20px;
  stroke: currentColor;
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.creation-note p {
  margin: 0;
  color: var(--text);
  font-size: 14px;
  line-height: 1.5;
}

.return-button {
  min-height: 46px;
  gap: 14px;
  margin-top: 27px;
  border-radius: 8px;
  font-size: 14px;
  text-decoration: none;
}

.return-button span {
  font-size: 18px;
  line-height: 1;
}

.creation-preview {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--divider);
  border-radius: 18px;
  background: var(--surface-raised);
}

.preview-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 36px;
  margin: 0 2px 12px;
}

.preview-heading h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.preview-heading span {
  color: var(--muted);
  font-size: 12px;
}

.portrait-canvas {
  overflow: hidden;
  aspect-ratio: 520 / 390;
  border-radius: 12px;
  background: var(--subtle);
}

.portrait-canvas svg {
  display: block;
  width: 100%;
  height: 100%;
}

.canvas-background {
  fill: var(--subtle);
}

.portrait-arch {
  fill: var(--surface);
  stroke: var(--divider);
  stroke-width: 1.5;
}

.portrait-head {
  fill: var(--panel-edge);
  opacity: 0.64;
}

.portrait-shoulders {
  fill: var(--accent);
  opacity: 0.78;
}

.portrait-detail {
  fill: none;
  stroke: var(--surface);
  stroke-width: 5;
  stroke-linecap: round;
  opacity: 0.74;
}

.canvas-lines {
  fill: none;
  stroke: var(--divider);
  stroke-width: 2;
  stroke-linecap: round;
}

.canvas-dot {
  fill: var(--accent);
  opacity: 0.72;
}

.preview-steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
  padding: 16px 2px 1px;
  list-style: none;
}

.preview-steps li {
  display: flex;
  flex-direction: column;
  gap: 3px;
  color: var(--text);
  font-size: 13px;
  font-weight: 550;
}

.step-number {
  color: var(--accent);
  font-size: 11px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 820px) {
  .creation-layout {
    grid-template-columns: minmax(0, 1fr);
    gap: 34px;
    max-width: 620px;
    margin: 0 auto;
    padding: 32px 0 16px;
  }

  .creation-copy {
    max-width: 540px;
  }

  .creation-copy h1 {
    max-width: 16ch;
    font-size: clamp(38px, 8vw, 52px);
  }

  .creation-preview {
    width: 100%;
  }
}

@media (max-width: 480px) {
  .creation-layout {
    gap: 28px;
    padding-top: 20px;
  }

  .creation-copy h1 {
    font-size: 40px;
  }

  .creation-preview {
    padding: 12px;
    border-radius: 14px;
  }

  .portrait-canvas {
    aspect-ratio: 1.18;
  }

  .preview-steps {
    gap: 4px;
    padding-top: 13px;
  }

  .preview-steps li {
    font-size: 12px;
  }
}
</style>
