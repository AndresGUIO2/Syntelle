<script setup lang="ts">
import { useLocaleStore } from '../stores/locale'
import ThemeToggle from './ThemeToggle.vue'
import LanguageSelector from './LanguageSelector.vue'
import AppIcon from './AppIcon.vue'
import { useThemeStore } from '../stores/theme'
import { nextTick, onMounted, onScopeDispose, shallowRef, useId, useTemplateRef, watch } from 'vue'

const { t } = useLocaleStore()
const emit = defineEmits<{ uploadImage: []; account: []; navigate: [event: MouseEvent] }>()
const props = withDefaults(
  defineProps<{
    showUpload?: boolean
    showTheme?: boolean
    showAccount?: boolean
    showPricing?: boolean
    showCreateCharacter?: boolean
    pricingActive?: boolean
    characterCreationActive?: boolean
    workspacesActive?: boolean
    adminActive?: boolean
    chatActive?: boolean
    signedIn?: boolean
    staffEligible?: boolean
  }>(),
  { showUpload: true, showTheme: true },
)

const open = shallowRef(false)
const menuId = useId()
const menu = useTemplateRef<HTMLElement>('menu')
const menuButton = useTemplateRef<HTMLButtonElement>('menuButton')

function closeMenu(restoreFocus = false) {
  if (!open.value) return
  open.value = false
  if (restoreFocus) menuButton.value?.focus()
}
async function focusFirstLink() {
  open.value = true
  await nextTick()
  menu.value?.querySelector<HTMLAnchorElement>('nav a')?.focus()
}
function outside(event: PointerEvent) {
  if (event.target instanceof Node && !menu.value?.contains(event.target)) closeMenu()
}
function focusLeft(event: FocusEvent) {
  if (!(event.relatedTarget instanceof Node) || !menu.value?.contains(event.relatedTarget))
    closeMenu()
}
function followLink(event: MouseEvent) {
  emit('navigate', event)
  if (event.defaultPrevented) closeMenu(true)
}
function accountFromMenu() {
  closeMenu(true)
  emit('account')
}
function uploadFromMenu() {
  closeMenu(true)
  emit('uploadImage')
}
watch(
  () => [
    props.signedIn,
    props.chatActive,
    props.characterCreationActive,
    props.pricingActive,
    props.workspacesActive,
    props.adminActive,
  ],
  () => closeMenu(),
)
onMounted(() => document.addEventListener('pointerdown', outside))
onScopeDispose(() => document.removeEventListener('pointerdown', outside))
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
      <a
        v-if="showPricing"
        class="desktop-action header-link"
        href="/pricing"
        :aria-current="pricingActive ? 'page' : undefined"
        @click="$emit('navigate', $event)"
        >{{ t('Pricing') }}</a
      >
      <button
        v-if="showAccount && !signedIn"
        type="button"
        class="desktop-action header-link"
        aria-haspopup="dialog"
        @click="$emit('account')"
      >
        {{ t('Sign in') }}
      </button>
      <button
        v-if="showUpload"
        type="button"
        class="desktop-action header-link"
        @click="$emit('uploadImage')"
      >
        <AppIcon name="upload" />{{ t('Upload photo') }}
      </button>
      <ThemeToggle v-if="showTheme" />
      <div
        ref="menu"
        class="navigation-control"
        :class="{ 'member-menu': signedIn }"
        @focusout="focusLeft"
        @keydown.esc.stop.prevent="closeMenu(true)"
      >
        <button
          ref="menuButton"
          type="button"
          class="menu-trigger"
          :aria-expanded="open"
          :aria-controls="menuId"
          @click="open = !open"
          @keydown.down.prevent="focusFirstLink"
        >
          <span class="menu-lines" aria-hidden="true"><i /><i /><i /></span>
          <span>{{ t('Menu') }}</span>
        </button>
        <nav v-show="open" :id="menuId" class="navigation-panel" :aria-label="t('Menu')">
          <a href="/" :aria-current="chatActive ? 'page' : undefined" @click="followLink"
            ><AppIcon name="message" />{{ t('Chat') }}</a
          >
          <a
            v-if="showCreateCharacter && signedIn"
            href="/characters/new"
            :aria-current="characterCreationActive ? 'page' : undefined"
            @click="followLink"
            ><AppIcon name="image" />{{ t('Characters') }}</a
          >
          <a
            v-if="signedIn && showAccount"
            href="/?view=workspaces"
            :aria-current="workspacesActive ? 'page' : undefined"
            @click="followLink"
            >{{ t('Workspaces') }}</a
          >
          <a
            v-if="showPricing"
            class="compact-action"
            href="/pricing"
            :aria-current="pricingActive ? 'page' : undefined"
            @click="followLink"
            >{{ t('Pricing') }}</a
          >
          <button v-if="showUpload" class="compact-action" type="button" @click="uploadFromMenu">
            {{ t('Upload photo') }}
          </button>
          <a
            v-if="signedIn && staffEligible"
            href="/?view=admin"
            :aria-current="adminActive ? 'page' : undefined"
            @click="followLink"
            >{{ t('Administration') }}</a
          >
          <button
            v-if="showAccount"
            class="account-action"
            type="button"
            aria-haspopup="dialog"
            @click="accountFromMenu"
          >
            {{ t(signedIn ? 'Your account' : 'Sign in') }}
          </button>
        </nav>
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
.header-language {
  margin-left: auto;
}
.header-link,
.menu-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 44px;
  padding: 8px 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
}
.header-link[aria-current='page'] {
  color: var(--accent);
}
.navigation-control {
  display: none;
  position: relative;
  flex: 0 0 auto;
}
.member-menu {
  display: block;
}
.menu-trigger[aria-expanded='true'] {
  background: var(--subtle);
}
.menu-lines {
  display: grid;
  gap: 3px;
}
.menu-lines i {
  width: 16px;
  height: 1.5px;
  background: currentColor;
  border-radius: 1px;
}
.navigation-panel {
  position: absolute;
  z-index: 15;
  top: calc(100% + 4px);
  right: 0;
  display: grid;
  width: min(225px, calc(100vw - 24px));
  padding: 6px;
  border: 1px solid var(--divider);
  border-radius: 14px;
  background: var(--surface-raised);
  box-shadow: 0 12px 28px #0003;
  max-height: calc(100dvh - 64px);
  overflow: auto;
  overscroll-behavior: contain;
}
.navigation-panel :is(a, button) {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 44px;
  padding: 9px 12px;
  background: transparent;
  color: var(--text);
  border: 0;
  border-radius: 7px;
  font: inherit;
  font-size: 14px;
  text-align: start;
  text-decoration: none;
  cursor: pointer;
}
.navigation-panel .compact-action {
  display: none;
}
.navigation-panel :deep(.icon) {
  width: 16px;
  height: 16px;
}
.navigation-panel [aria-current='page'] {
  background: var(--subtle);
  color: var(--accent);
  font-weight: 650;
}
.navigation-panel .account-action {
  margin-top: 4px;
  border-top: 1px solid var(--divider);
  border-top-left-radius: 0;
  border-top-right-radius: 0;
}
.menu-trigger:focus-visible,
.header-link:focus-visible,
.navigation-panel :focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}
@media (hover: hover) {
  .header-link:hover,
  .menu-trigger:hover,
  .navigation-panel :is(a, button):hover {
    background: var(--subtle);
  }
}
@media (max-width: 760px) {
  .header-inner {
    gap: 4px;
    padding: 0 12px;
  }
  .brand {
    gap: 0;
  }
  .company-signature,
  .desktop-action {
    display: none;
  }
  .wordmark {
    font-size: 18px;
  }
  .navigation-control {
    display: block;
  }
  .navigation-panel .compact-action {
    display: flex;
  }
  .menu-trigger {
    gap: 6px;
    padding: 4px 8px;
  }
  .header-language :deep(.language-trigger) {
    gap: 4px;
    padding: 6px;
    font-size: 12px;
  }
  .header-language :deep(.language-globe) {
    display: none;
  }
}
</style>
