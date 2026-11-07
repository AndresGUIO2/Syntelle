<script setup lang="ts">
import { computed, nextTick, onScopeDispose, shallowRef, useId, useTemplateRef, watch } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import BrandSignature from '../../components/BrandSignature.vue'
import { useLocaleStore } from '../../stores/locale'
import NavigationPanel from './NavigationPanel.vue'
import type { NavigationSection } from '../../composables/usePageNavigation'
import type { Workspace } from '../workspaces/useWorkspaces'

const props = defineProps<{
  signedIn: boolean
  accountLabel: string
  staffEligible: boolean
  active: NavigationSection
  workspaces?: Workspace[]
  workspaceId?: string
  conversationHref: string
}>()
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  account: []
  workspace: [id: string]
  createWorkspace: []
}>()
const { t } = useLocaleStore()
const compactQuery = matchMedia('(max-width: 1023px)')
const wideQuery = matchMedia('(min-width: 1440px)')
const mobile = shallowRef(compactQuery.matches)
const wide = shallowRef(wideQuery.matches)
const preference = shallowRef<boolean | null>(null)
const sidebarWidth = shallowRef(264)
let releaseResize: (() => void) | undefined
try {
  const saved = Number(localStorage.getItem('syntelle.navigation.width'))
  if (saved >= 240 && saved <= 320) sidebarWidth.value = saved
} catch {
  /* Optional preference. */
}
function saveWidth() {
  try {
    localStorage.setItem('syntelle.navigation.width', String(sidebarWidth.value))
  } catch {
    /* Optional preference. */
  }
}
function resizeSidebar(event: PointerEvent) {
  if (event.button !== 0 || collapsed.value) return
  event.preventDefault()
  releaseResize?.()
  const start = event.clientX,
    width = sidebarWidth.value
  const move = (next: PointerEvent) => {
    sidebarWidth.value = Math.max(240, Math.min(320, width + next.clientX - start))
  }
  const end = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', end)
    window.removeEventListener('pointercancel', end)
    releaseResize = undefined
    saveWidth()
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', end)
  window.addEventListener('pointercancel', end)
  releaseResize = end
}
function resizeKey(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  sidebarWidth.value =
    event.key === 'Home'
      ? 240
      : event.key === 'End'
        ? 320
        : Math.max(240, Math.min(320, sidebarWidth.value + (event.key === 'ArrowRight' ? 8 : -8)))
  saveWidth()
}
try {
  const value = localStorage.getItem('syntelle.navigation.collapsed')
  if (value === 'true' || value === 'false') preference.value = value === 'true'
} catch {
  /* Navigation preferences are optional. */
}
const collapsed = computed(() => preference.value ?? !wide.value)
const drawerOpen = shallowRef(false)
const drawer = useTemplateRef<HTMLDialogElement>('drawer')
const trigger = useTemplateRef<HTMLButtonElement>('trigger')
const drawerId = useId()
let previousOverflow: string | undefined
function unlockScroll() {
  if (previousOverflow === undefined) return
  document.body.style.overflow = previousOverflow
  previousOverflow = undefined
}
function closeDrawer(restoreFocus = true) {
  drawerOpen.value = false
  drawer.value?.close()
  unlockScroll()
  if (restoreFocus && mobile.value) void nextTick(() => trigger.value?.focus())
}
function resize() {
  mobile.value = compactQuery.matches
  wide.value = wideQuery.matches
  if (!mobile.value) closeDrawer(false)
}
function toggle() {
  preference.value = !collapsed.value
  try {
    localStorage.setItem('syntelle.navigation.collapsed', String(preference.value))
  } catch {
    /* Keep the choice in memory when storage is unavailable. */
  }
}
function follow(event: MouseEvent) {
  emit('navigate', event)
  if (event.defaultPrevented) closeDrawer(false)
}
function createWorkspace() {
  closeDrawer()
  emit('createWorkspace')
}
function account() {
  closeDrawer()
  emit('account')
}
defineExpose({ closeNavigation: () => closeDrawer(false) })
function backdrop(event: MouseEvent) {
  const element = drawer.value
  if (!element || event.target !== element) return
  const bounds = element.getBoundingClientRect()
  if (
    event.clientX < bounds.left ||
    event.clientX > bounds.right ||
    event.clientY < bounds.top ||
    event.clientY > bounds.bottom
  )
    closeDrawer()
}
watch([drawerOpen, drawer], ([open, element]) => {
  if (!open || !element || element.open) return
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  element.showModal()
})
watch(
  () => props.active,
  () => closeDrawer(false),
)
compactQuery.addEventListener('change', resize)
wideQuery.addEventListener('change', resize)
onScopeDispose(() => {
  compactQuery.removeEventListener('change', resize)
  wideQuery.removeEventListener('change', resize)
  drawer.value?.close()
  unlockScroll()
  releaseResize?.()
})
</script>

<template>
  <div
    class="navigation-shell"
    :style="{ '--sidebar-width': `${sidebarWidth}px` }"
    :class="{
      'navigation-collapsed': collapsed,
      'navigation-mobile': mobile,
      'console-shell': active !== 'chat',
    }"
  >
    <aside v-if="!mobile" class="desktop-sidebar">
      <NavigationPanel
        v-bind="props"
        :compact="collapsed"
        @navigate="follow"
        @account="account"
        @toggle="toggle"
        @workspace="emit('workspace', $event)"
        @create-workspace="emit('createWorkspace')"
      />
      <div
        v-if="!collapsed"
        class="sidebar-resizer"
        role="separator"
        tabindex="0"
        aria-orientation="vertical"
        :aria-label="t('Resize navigation')"
        :aria-valuenow="sidebarWidth"
        :aria-valuemin="240"
        :aria-valuemax="320"
        @pointerdown="resizeSidebar"
        @keydown="resizeKey"
      />
    </aside>
    <header v-else class="mobile-header">
      <button
        ref="trigger"
        class="drawer-trigger"
        type="button"
        :aria-label="t('Open navigation')"
        :aria-expanded="drawerOpen"
        :aria-controls="drawerId"
        aria-haspopup="dialog"
        @click="drawerOpen = true"
      >
        <AppIcon name="menu" />
      </button>
      <BrandSignature
        :href="signedIn ? '/console' : conversationHref"
        variant="mobile"
        @navigate="follow"
      />
    </header>
    <div class="navigation-content"><slot /></div>
    <dialog
      v-if="mobile"
      :id="drawerId"
      ref="drawer"
      class="mobile-drawer"
      :aria-label="t('Main navigation')"
      @cancel.prevent="closeDrawer()"
      @close="closeDrawer()"
      @click="backdrop"
    >
      <NavigationPanel
        v-bind="props"
        mobile
        @navigate="follow"
        @account="account"
        @close="closeDrawer()"
        @workspace="emit('workspace', $event)"
        @create-workspace="createWorkspace"
      />
    </dialog>
  </div>
</template>

<style scoped>
.navigation-shell {
  display: grid;
  grid-template-columns: var(--sidebar-width, 264px) minmax(0, 1fr);
  height: 100dvh;
  min-height: 0;
  background: var(--page);
}
.console-shell .navigation-content {
  background: var(--console-canvas);
}
.navigation-shell.navigation-collapsed {
  grid-template-columns: 72px minmax(0, 1fr);
}
.desktop-sidebar {
  position: relative;
  z-index: 20;
  min-height: 0;
  background: #0d1016;
  border-right: 1px solid #ffffff1a;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
}
.navigation-content {
  min-width: 0;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}
.navigation-shell.navigation-mobile {
  grid-template-columns: minmax(0, 1fr);
  grid-template-rows: 60px minmax(0, 1fr);
}
.mobile-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--navigation-edge, #494252);
  background: #0d1016;
  color: var(--navigation-text, #f3f0f7);
}
.drawer-trigger {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--navigation-text, #f3f0f7);
  cursor: pointer;
}
.drawer-trigger:focus-visible {
  outline: 2px solid var(--navigation-accent, #bba3f5);
  outline-offset: 2px;
}
.mobile-drawer {
  position: fixed;
  inset: 0 auto 0 0;
  margin: 0;
  padding: 0;
  width: min(320px, calc(100vw - 40px));
  max-width: none;
  height: 100dvh;
  max-height: none;
  border: 0;
  border-right: 1px solid var(--navigation-edge, #494252);
  background: #0d1016;
  color: var(--navigation-text, #f3f0f7);
  overflow-y: auto;
  overscroll-behavior: contain;
  box-shadow: 12px 0 40px #0003;
}
.mobile-drawer::backdrop {
  background: #08060d88;
}
@media (hover: hover) {
  .drawer-trigger:hover {
    background: var(--navigation-hover, #292430);
  }
}
@media (max-height: 620px) and (min-width: 1024px) {
  .desktop-sidebar {
    overflow-y: auto;
    overscroll-behavior: contain;
  }
}
@media (max-height: 600px) {
  .mobile-drawer {
    overflow-y: auto;
  }
  .mobile-drawer :deep(.navigation-panel) {
    min-height: 560px;
  }
}
.sidebar-resizer {
  position: fixed;
  top: 0;
  bottom: 0;
  left: calc(var(--sidebar-width, 264px) - 4px);
  z-index: 31;
  width: 8px;
  cursor: col-resize;
  touch-action: none;
}
.sidebar-resizer::after {
  content: '';
  position: absolute;
  left: 3px;
  top: calc(50% - 18px);
  width: 2px;
  height: 36px;
  border-radius: 2px;
  background: var(--navigation-control-border, #857b94);
}
.sidebar-resizer:focus-visible {
  outline: 2px solid var(--navigation-accent, #bba3f5);
  outline-offset: -3px;
}
@media (hover: hover) {
  .sidebar-resizer:hover {
    background: var(--navigation-resizer-hover, #bba3f540);
  }
}
</style>
