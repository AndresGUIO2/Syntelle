<script setup lang="ts">
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'

const emit = defineEmits<{ navigate: [event: MouseEvent] }>()
const { t } = useLocaleStore()
</script>

<template>
  <a href="/characters/new" class="create-entry" @click="emit('navigate', $event)">
    <div class="entry-art" aria-hidden="true">
      <div class="portrait-stack">
        <span class="portrait-sheet sheet-left" />
        <span class="portrait-sheet sheet-right" />
        <span class="portrait-sheet sheet-front">
          <AppIcon name="user" />
          <span class="portrait-add"><AppIcon name="plus" /></span>
        </span>
      </div>
      <span class="art-note image-note"><AppIcon name="image" /></span>
      <span class="art-note voice-note"><AppIcon name="mic" /></span>
    </div>
    <div class="entry-details">
      <span class="entry-label">{{ t('Your first character') }}</span>
      <h2>{{ t('Start with a portrait.') }}</h2>
      <p>{{ t('A familiar face, a new idea, a personality of your own.') }}</p>
      <div class="entry-journey" aria-hidden="true">
        <span>{{ t('Portrait') }}</span
        ><AppIcon class="point-right" name="chevron-left" /> <span>{{ t('Character') }}</span
        ><AppIcon class="point-right" name="chevron-left" />
        <span>{{ t('Your website') }}</span>
      </div>
      <span class="entry-link">
        {{ t('Bring it to life') }}
        <span class="entry-arrow"><AppIcon class="point-right" name="chevron-left" /></span>
      </span>
    </div>
  </a>
</template>

<style scoped>
.point-right {
  transform: rotate(180deg);
}
.create-entry {
  display: flex;
  flex-direction: column;
  min-height: 500px;
  height: 100%;
  border: 1px solid transparent;
  border-radius: 22px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
  box-shadow:
    0 0 0 1px color-mix(in srgb, var(--text) 8%, transparent),
    0 4px 8px #15131b06,
    0 16px 32px #15131b0a;
}
.entry-art {
  position: relative;
  isolation: isolate;
  height: 218px;
  flex-shrink: 0;
  margin: 8px 8px 0;
  overflow: hidden;
  border-radius: 14px;
  background: linear-gradient(
    145deg,
    color-mix(in srgb, var(--accent) 12%, var(--surface)),
    color-mix(in srgb, var(--accent) 3%, var(--surface))
  );
}
.entry-art::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background-image: radial-gradient(
    color-mix(in srgb, var(--accent) 24%, transparent) 0.8px,
    transparent 0.8px
  );
  background-size: 16px 16px;
  mask-image: radial-gradient(ellipse, black 25%, transparent 76%);
}
.portrait-stack {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 108px;
  height: 132px;
  transform: translate(-50%, -50%);
}
.portrait-sheet {
  position: absolute;
  inset: 0;
  border: 1px solid color-mix(in srgb, var(--accent) 24%, var(--surface));
  border-radius: 14px;
  background: color-mix(in srgb, var(--accent) 5%, var(--surface));
}
.sheet-left {
  transform: translate(-33px, 8px) rotate(-14deg);
}
.sheet-right {
  transform: translate(33px, 8px) rotate(14deg);
}
.sheet-front {
  display: grid;
  place-items: center;
  color: var(--accent);
  box-shadow:
    0 2px 4px #15131b08,
    0 12px 24px #15131b14;
}
.sheet-front > svg {
  width: 58px;
  height: 58px;
  stroke-width: 1.2;
}
.portrait-add {
  position: absolute;
  right: -13px;
  bottom: -12px;
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border: 3px solid var(--surface);
  border-radius: 50%;
  background: var(--primary);
  color: var(--on-primary);
}
.portrait-add svg {
  width: 18px;
  height: 18px;
}
.art-note {
  position: absolute;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border: 1px solid color-mix(in srgb, var(--accent) 18%, var(--surface));
  border-radius: 11px;
  background: var(--surface);
  color: var(--accent);
}
.art-note svg {
  width: 18px;
  height: 18px;
}
.image-note {
  top: 35px;
  left: 28px;
  transform: rotate(-8deg);
}
.voice-note {
  right: 25px;
  bottom: 29px;
  transform: rotate(8deg);
}
.entry-details {
  display: flex;
  flex: 1;
  flex-direction: column;
  padding: 23px 24px 20px;
}
.entry-label {
  margin-bottom: 10px;
  color: var(--muted);
  font-size: 10px;
  font-weight: 550;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.entry-details h2 {
  margin: 0 0 11px;
  font-size: 25px;
  line-height: 1.15;
  font-weight: 600;
  letter-spacing: -0.7px;
  text-wrap: balance;
}
.entry-details p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.65;
}
.entry-journey {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin: 19px 0 21px;
  color: var(--muted);
  font-size: 10px;
}
.entry-journey svg {
  width: 12px;
  height: 12px;
}
.entry-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 49px;
  margin-top: auto;
  padding-top: 15px;
  border-top: 1px solid var(--divider);
  color: var(--accent);
  font-size: 13px;
  font-weight: 600;
}
.entry-arrow {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--primary-border);
  border-radius: 50%;
  background: var(--primary);
  color: var(--on-primary);
}
.entry-arrow svg {
  width: 17px;
  height: 17px;
}
.create-entry:focus-visible {
  outline: 3px solid var(--accent);
  outline-offset: 4px;
}
@media (hover: hover) {
  .create-entry:hover {
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--accent) 65%, transparent),
      0 4px 8px #15131b0a,
      0 16px 32px #15131b14;
  }
  .create-entry:hover .entry-arrow {
    background: var(--primary-hover);
  }
}
@media (hover: hover) and (prefers-reduced-motion: no-preference) {
  .create-entry {
    transition:
      transform 150ms ease-out,
      box-shadow 150ms ease-out;
  }
  .portrait-sheet {
    transition: transform 150ms ease-out;
  }
  .create-entry:hover {
    transform: translateY(-2px);
  }
  .create-entry:hover .sheet-left {
    transform: translate(-38px, 8px) rotate(-17deg);
  }
  .create-entry:hover .sheet-right {
    transform: translate(38px, 8px) rotate(17deg);
  }
}
</style>
