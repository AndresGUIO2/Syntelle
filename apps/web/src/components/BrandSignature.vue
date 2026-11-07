<script setup lang="ts">
import { useLocaleStore } from '../stores/locale'

withDefaults(defineProps<{ href: string; variant?: 'expanded' | 'compact' | 'mobile' }>(), {
  variant: 'expanded',
})
const emit = defineEmits<{ navigate: [event: MouseEvent] }>()
const { t } = useLocaleStore()
</script>

<template>
  <div class="brand-signature" :class="variant">
    <a
      class="brand-wordmark"
      :href="href"
      :aria-label="t('Syntelle home')"
      @click="emit('navigate', $event)"
    >
      {{ variant === 'compact' ? 'S' : 'Syntelle' }}<span>.</span>
    </a>
    <div class="company-signature" aria-label="by Colors Studio">
      <span class="brand-by">by</span>
      <img
        :src="
          variant === 'expanded' ? '/brand/colors-studio.svg' : '/brand/colors-studio-symbol.svg'
        "
        alt="Colors Studio"
        :width="variant === 'expanded' ? 120 : 24"
        :height="variant === 'expanded' ? 42 : 24"
      />
    </div>
  </div>
</template>

<style scoped>
.brand-signature {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}
.brand-wordmark {
  width: fit-content;
  color: var(--navigation-text, #f3f0f7);
  font-size: 26px;
  line-height: 1.25;
  font-weight: 750;
  letter-spacing: -0.9px;
  text-decoration: none;
}
.brand-wordmark span {
  color: var(--navigation-accent, #bba3f5);
}
.brand-wordmark:focus-visible {
  outline: 2px solid var(--navigation-accent, #bba3f5);
  outline-offset: 4px;
  border-radius: 3px;
}
.company-signature {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.brand-by {
  color: var(--navigation-muted, #bdb6c9);
  font-size: 12px;
  line-height: 1;
  flex-shrink: 0;
}
.company-signature img {
  display: block;
  object-fit: contain;
  flex-shrink: 0;
}
.compact {
  align-items: center;
  gap: 7px;
}
.compact .brand-wordmark {
  font-size: 24px;
}
.compact .company-signature {
  gap: 3px;
}
.compact .brand-by {
  font-size: 10px;
}
.mobile {
  flex-direction: row;
  align-items: center;
  gap: 12px;
}
.mobile .brand-wordmark {
  font-size: 21px;
}
.mobile .company-signature {
  padding-left: 12px;
  border-left: 1px solid var(--navigation-edge, #494252);
  gap: 6px;
}
</style>
