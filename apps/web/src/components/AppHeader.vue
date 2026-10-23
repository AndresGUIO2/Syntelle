<script setup lang="ts">
import { useLocaleStore } from '../stores/locale'
import ThemeToggle from './ThemeToggle.vue'
import LanguageSelector from './LanguageSelector.vue'
import AppIcon from './AppIcon.vue'
import { useThemeStore } from '../stores/theme'

const { t } = useLocaleStore()
defineEmits<{ uploadImage: []; account: []; navigate: [event: MouseEvent] }>()
withDefaults(
  defineProps<{
    showUpload?: boolean
    showTheme?: boolean
    showAccount?: boolean
    showPricing?: boolean
    showCreateCharacter?: boolean
    pricingActive?: boolean
    characterCreationActive?: boolean
    signedIn?: boolean
  }>(),
  {
    showUpload: true,
    showTheme: true,
  },
)
const theme = useThemeStore()
const companyAssets = {
  wordmark: '/brand/colors-studio.svg',
  lightWordmark: '/brand/colors-studio-light.svg',
  symbol: '/brand/colors-studio-symbol.svg',
}
</script>

<template>
  <header class="app-header">
    <div class="header-inner">
      <div class="brand">
        <a
          class="wordmark"
          href="/"
          :aria-label="t('Syntelle home')"
          @click="$emit('navigate', $event)"
          >Syntelle</a
        >
        <div class="company-signature">
          <span class="attribution">{{ t('by') }}</span>
          <picture class="company-logo">
            <source media="(max-width: 600px)" :srcset="companyAssets.symbol" />
            <img
              class="company-logo-image"
              :src="theme.theme === 'light' ? companyAssets.lightWordmark : companyAssets.wordmark"
              alt="Colors Studio"
              width="460"
              height="160"
            />
          </picture>
        </div>
      </div>
      <LanguageSelector class="header-language" />
      <div class="header-actions">
        <a
          v-if="showPricing"
          class="header-pricing"
          href="/pricing"
          :aria-current="pricingActive ? 'page' : undefined"
          @click="$emit('navigate', $event)"
        >
          {{ t('Pricing') }}
        </a>
        <a
          v-if="showCreateCharacter && signedIn"
          class="header-create-character"
          href="/characters/new"
          :aria-label="t('Create a character')"
          :aria-current="characterCreationActive ? 'page' : undefined"
          @click="$emit('navigate', $event)"
        >
          <AppIcon name="plus" />
          <span>{{ t('Create') }}</span>
        </a>
        <button
          v-if="showAccount"
          class="header-account"
          type="button"
          aria-haspopup="dialog"
          @click="$emit('account')"
        >
          {{ t(signedIn ? 'Your account' : 'Sign in') }}
        </button>
        <ThemeToggle v-if="showTheme" />
        <button
          v-if="showUpload"
          class="button button-primary header-upload"
          type="button"
          :aria-label="t('Upload photo')"
          @click="$emit('uploadImage')"
        >
          <AppIcon name="upload" /><span class="upload-label"> {{ t('Upload photo') }} </span>
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  height: 48px;
  background: var(--surface);
  border-bottom: 1px solid var(--divider);
}
.header-inner {
  max-width: var(--workspace-max-width, none);
  height: 100%;
  margin: 0 auto;
  padding: 0 var(--workspace-gutter, 20px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}
.wordmark {
  color: var(--text);
  text-decoration: none;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.4px;
}
.company-signature {
  display: flex;
  align-items: center;
  gap: 8px;
  border-left: 1px solid var(--divider);
  padding-left: 16px;
}
.attribution {
  color: var(--muted);
  font-size: 12px;
}
.company-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 120px;
  height: 44px;
}
.company-logo-image {
  display: block;
  width: 120px;
  height: 42px;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.header-language {
  margin-left: auto;
}
.header-account,
.header-pricing,
.header-create-character {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 8px 12px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  cursor: pointer;
  text-decoration: none;
}
.header-create-character {
  gap: 6px;
  color: var(--accent);
}
.header-create-character :deep(.icon) {
  width: 16px;
  height: 16px;
}
.header-pricing[aria-current='page'],
.header-create-character[aria-current='page'] {
  color: var(--accent);
}
@media (hover: hover) {
  .header-account:hover,
  .header-pricing:hover,
  .header-create-character:hover {
    background: var(--subtle);
  }
}
@media (max-width: 800px) {
  .header-inner {
    padding: 0 var(--workspace-gutter, 16px);
  }
}
@media (max-width: 600px) {
  .app-header {
    height: auto;
    min-height: 48px;
  }
  .header-inner {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0 12px;
  }
  .header-actions {
    grid-column: 1 / -1;
    justify-content: flex-end;
  }
  .brand {
    gap: 12px;
  }
  .company-signature {
    padding-left: 12px;
    gap: 6px;
  }
  .company-logo {
    width: 32px;
    height: 36px;
  }
  .company-logo-image {
    width: 32px;
    height: 32px;
  }
  .header-actions {
    gap: 8px;
  }
  .header-upload {
    width: 44px;
    padding: 10px;
  }
  .upload-label {
    display: none;
  }
}
@media (max-width: 420px) {
  .brand {
    gap: 10px;
  }
  .company-signature {
    padding-left: 10px;
  }
  .company-logo,
  .company-logo-image {
    width: 28px;
  }
  .header-actions {
    gap: 4px;
  }
  .header-account,
  .header-pricing,
  .header-create-character {
    padding-right: 8px;
    padding-left: 8px;
  }
}
</style>
