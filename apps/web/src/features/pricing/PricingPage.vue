<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'

const { t } = useLocaleStore()
defineEmits<{ navigate: [event: MouseEvent] }>()

const upcomingDetails = [
  'Plans',
  'What’s included',
  'Usage details',
] as const
</script>

<template>
  <section class="pricing-page" aria-labelledby="pricing-title">
    <a class="pricing-back" href="/" @click="$emit('navigate', $event)">
      <span aria-hidden="true">←</span>{{ t('Back to Syntelle') }}
    </a>

    <div class="pricing-hero">
      <div class="pricing-intro">
        <p class="pricing-eyebrow">
          <span class="eyebrow-dot" aria-hidden="true"></span>{{ t('Syntelle pricing') }}
        </p>
        <h1 id="pricing-title" class="pricing-title">{{ t('Clear pricing is on its way.') }}</h1>
        <p class="pricing-description">
          {{
            t(
              'We’re finalizing the details. This page will explain the plans, what’s included, and how usage works once everything is ready.',
            )
          }}
        </p>
        <a class="button button-primary pricing-cta" href="/" @click="$emit('navigate', $event)">
          {{ t('Return to the demo') }}
          <span class="cta-arrow" aria-hidden="true">→</span>
        </a>
      </div>

      <aside class="pricing-preview" :aria-label="t('Pricing details in progress')">
        <div class="preview-topline">
          <span class="preview-label">{{ t('A clearer picture is coming') }}</span>
          <span class="preview-mark" aria-hidden="true"><span></span></span>
        </div>
        <h2 class="preview-title">{{ t('What you’ll find here') }}</h2>
        <p class="preview-description">{{ t('The details are still being prepared.') }}</p>

        <ul class="details-list">
          <li v-for="(detail, index) in upcomingDetails" :key="detail" class="details-row">
            <span class="detail-index" aria-hidden="true">0{{ index + 1 }}</span>
            <span class="detail-name">{{ t(detail) }}</span>
            <span class="detail-status">{{ t('In progress') }}</span>
          </li>
        </ul>

        <p class="preview-footnote">
          <span aria-hidden="true">✳</span>{{ t('No prices have been published yet.') }}
        </p>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.pricing-page {
  width: 100%;
  max-width: 1120px;
  min-height: 100%;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  padding-bottom: 12px;
}

.pricing-back {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  gap: 8px;
  min-height: 44px;
  color: var(--muted);
  font-size: 14px;
  text-decoration: none;
}

.pricing-hero {
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(340px, 0.82fr);
  align-items: center;
  gap: clamp(48px, 7vw, 96px);
  padding: clamp(48px, 7vh, 76px) clamp(12px, 3vw, 36px);
}

.pricing-intro {
  max-width: 560px;
}

.pricing-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  margin: 0 0 24px;
  color: var(--accent);
  font-size: 13px;
  font-weight: 650;
  letter-spacing: 0.04em;
}

.eyebrow-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 5px var(--orb-wash);
}

.pricing-title {
  max-width: 11ch;
  margin: 0;
  font-size: clamp(42px, 5.1vw, 64px);
  font-weight: 600;
  line-height: 1.04;
  letter-spacing: -0.055em;
  text-wrap: balance;
}

.pricing-description {
  max-width: 48ch;
  margin: 24px 0 30px;
  color: var(--muted);
  font-size: 17px;
  line-height: 1.7;
}

.pricing-cta {
  min-height: 48px;
  gap: 14px;
  padding: 11px 18px;
  border-radius: 8px;
  font-size: 14px;
  text-decoration: none;
}

.cta-arrow {
  font-size: 18px;
  line-height: 1;
  transition: transform 150ms ease-out;
}

.pricing-preview {
  position: relative;
  overflow: hidden;
  padding: clamp(26px, 3vw, 38px);
  border: 1px solid var(--divider);
  border-radius: 24px;
  background:
    radial-gradient(ellipse at 100% 0%, var(--orb-wash), transparent 54%),
    var(--surface-raised);
  box-shadow: var(--panel-shadow);
}

.preview-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.preview-label {
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.preview-mark {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  flex: 0 0 auto;
  border: 1px solid var(--stage-edge);
  border-radius: 50%;
  background: var(--orb-wash);
}

.preview-mark span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 6px var(--orb-ring);
}

.preview-title {
  margin: 22px 0 4px;
  font-size: 23px;
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1.3;
}

.preview-description {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
}

.details-list {
  margin: 28px 0 0;
  padding: 0;
  list-style: none;
}

.details-row {
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  min-height: 62px;
  border-top: 1px solid var(--divider);
}

.detail-index {
  color: var(--accent);
  font-size: 11px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}

.detail-name {
  min-width: 0;
  font-size: 14px;
  font-weight: 550;
}

.detail-status {
  padding: 5px 9px;
  border-radius: 999px;
  background: var(--subtle);
  color: var(--muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.preview-footnote {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin: 22px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.preview-footnote span {
  color: var(--accent);
  font-size: 15px;
  line-height: 1.15;
}

@media (hover: hover) {
  .pricing-back:hover {
    color: var(--text);
    text-decoration: underline;
    text-underline-offset: 4px;
  }

  .pricing-cta:hover .cta-arrow {
    transform: translateX(3px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .cta-arrow {
    transition: none;
  }
}

@media (max-width: 760px) {
  .pricing-page {
    min-height: auto;
  }

  .pricing-hero {
    grid-template-columns: minmax(0, 1fr);
    gap: 36px;
    padding: clamp(28px, 7vh, 56px) 0 24px;
  }

  .pricing-title {
    max-width: 13ch;
    font-size: clamp(40px, 9vw, 56px);
  }

  .pricing-description {
    font-size: 16px;
  }

  .pricing-preview {
    max-width: 560px;
  }
}

@media (max-width: 420px) {
  .pricing-hero {
    gap: 28px;
    padding-top: 28px;
  }

  .pricing-preview {
    padding: 24px 20px;
    border-radius: 20px;
  }

  .details-row {
    grid-template-columns: 26px minmax(0, 1fr) auto;
    gap: 8px;
  }

  .detail-name {
    font-size: 13px;
  }
}
</style>
