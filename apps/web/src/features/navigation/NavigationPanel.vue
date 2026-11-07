<script setup lang="ts">
import { computed, useId } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import BrandSignature from '../../components/BrandSignature.vue'
import LanguageSelector from '../../components/LanguageSelector.vue'
import ThemeToggle from '../../components/ThemeToggle.vue'
import { useLocaleStore } from '../../stores/locale'
import type { NavigationSection } from '../../composables/usePageNavigation'
import type { Workspace } from '../workspaces/useWorkspaces'
import NavigationItem from './NavigationItem.vue'

const props = defineProps<{
  signedIn: boolean
  accountLabel: string
  staffEligible: boolean
  active: NavigationSection
  conversationHref: string
  workspaces?: Workspace[]
  workspaceId?: string
  compact?: boolean
  mobile?: boolean
}>()
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  account: []
  toggle: []
  close: []
  workspace: [id: string]
  createWorkspace: []
}>()
const { t } = useLocaleStore()
const workspacePickerId = useId()
const accountInitial = computed(() => props.accountLabel.trim().slice(0, 1).toLocaleUpperCase())
const selected = computed(() => props.workspaces?.find((item) => item.id === props.workspaceId))
const financial = computed(
  () => !selected.value || ['owner', 'billing'].includes(selected.value.role),
)
const operational = computed(() => !selected.value || selected.value.role !== 'billing')
const manage = computed(
  () => !!selected.value && ['owner', 'administrator'].includes(selected.value.role),
)
const personalLinks = computed(() => [
  ...(props.signedIn
    ? [{ key: 'overview', href: '/console', label: 'Overview', icon: 'grid' as const }]
    : []),
  { key: 'characters', href: '/characters', label: 'Characters', icon: 'image' as const },
])
const workspaceLinks = computed(() => [
  ...(operational.value
    ? [
        {
          key: 'integrations',
          href: '/console/integrations',
          label: 'Integrations',
          icon: 'code' as const,
        },
      ]
    : []),
  { key: 'usage', href: '/console/usage', label: 'Usage', icon: 'chart' as const },
  ...(financial.value
    ? [{ key: 'billing', href: '/console/billing', label: 'Billing', icon: 'card' as const }]
    : []),
])
</script>

<template>
  <div class="navigation-panel" :class="{ compact }">
    <div class="panel-top">
      <BrandSignature
        :href="signedIn ? '/console' : conversationHref"
        :variant="compact ? 'compact' : 'expanded'"
        @navigate="emit('navigate', $event)"
      />
      <button
        class="quiet-control collapse-control"
        type="button"
        :aria-label="
          t(mobile ? 'Close navigation' : compact ? 'Expand navigation' : 'Collapse navigation')
        "
        :title="
          t(mobile ? 'Close navigation' : compact ? 'Expand navigation' : 'Collapse navigation')
        "
        @click="mobile ? emit('close') : emit('toggle')"
      >
        <AppIcon :name="mobile ? 'close' : 'sidebar'" />
      </button>
    </div>

    <template v-if="active !== 'admin'">
      <nav class="primary-navigation" :aria-label="t('Main navigation')">
        <NavigationItem
          v-for="item in personalLinks"
          :key="item.key"
          :label="t(item.label)"
          :href="item.href"
          :icon="item.icon"
          :active="active === item.key"
          :compact="compact"
          @click="emit('navigate', $event)"
        />
      </nav>
      <section v-if="signedIn" class="workspace-navigation" :aria-label="t('Workspace')">
        <div v-if="!compact" class="workspace-context">
          <label :for="workspaces?.length ? workspacePickerId : undefined" class="group-label">{{
            t('Workspace')
          }}</label>
          <div v-if="workspaces?.length" class="workspace-switch-row">
            <select
              :id="workspacePickerId"
              :value="workspaceId"
              @change="emit('workspace', ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="item in workspaces" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select>
            <button
              type="button"
              class="quiet-control"
              :aria-label="t('New workspace')"
              :title="t('New workspace')"
              @click="emit('createWorkspace')"
            >
              <AppIcon name="plus" />
            </button>
          </div>
          <p v-else class="no-workspace"><AppIcon name="workspace" />{{ t('No workspace yet') }}</p>
        </div>
        <NavigationItem
          v-else-if="workspaces?.length"
          :label="selected?.name || t('Choose workspace')"
          icon="workspace"
          compact
          @click="emit('toggle')"
        />
        <nav class="workspace-links" :aria-label="t('Workspace navigation')">
          <NavigationItem
            v-for="item in workspaceLinks"
            :key="item.key"
            :label="t(item.label)"
            :href="item.href"
            :icon="item.icon"
            :active="active === item.key"
            :compact="compact"
            @click="emit('navigate', $event)"
          />
        </nav>
        <nav v-if="selected" class="utility-navigation" :aria-label="t('Workspace tools')">
          <NavigationItem
            v-if="manage"
            :label="t('Team')"
            href="/console/team"
            icon="users"
            :active="active === 'team'"
            :compact="compact"
            @click="emit('navigate', $event)"
          />
          <NavigationItem
            :label="t('Workspace settings')"
            href="/console/settings"
            icon="adjust"
            :active="active === 'settings'"
            :compact="compact"
            @click="emit('navigate', $event)"
          />
        </nav>
      </section>
      <nav class="test-navigation" :aria-label="t('Character testing')">
        <NavigationItem
          :label="t('Test character')"
          :href="conversationHref"
          icon="play"
          :active="active === 'chat'"
          :compact="compact"
          @click="emit('navigate', $event)"
        />
      </nav>
    </template>
    <template v-else>
      <p v-if="!compact" class="group-label staff-label">{{ t('Staff console') }}</p>
      <nav class="primary-navigation" :aria-label="t('Main navigation')">
        <NavigationItem
          :label="t('Administration')"
          href="/?view=admin"
          icon="shield"
          active
          :compact="compact"
          @click="emit('navigate', $event)"
        />
        <NavigationItem
          :label="t('Customer console')"
          href="/console"
          icon="grid"
          :compact="compact"
          @click="emit('navigate', $event)"
        />
      </nav>
    </template>

    <div class="panel-bottom">
      <nav class="secondary-navigation" :aria-label="t('More from Syntelle')">
        <NavigationItem
          v-if="!signedIn"
          :label="t('Pricing')"
          href="/pricing"
          icon="tag"
          :active="active === 'pricing'"
          :compact="compact"
          @click="emit('navigate', $event)"
        />
        <NavigationItem
          v-if="signedIn && staffEligible && active !== 'admin'"
          :label="t('Staff console')"
          href="/?view=admin"
          icon="shield"
          :compact="compact"
          @click="emit('navigate', $event)"
        />
      </nav>
      <div class="preferences" :aria-label="t('Preferences')">
        <LanguageSelector class="navigation-language" /><ThemeToggle />
      </div>
      <NavigationItem
        class="account-button"
        :label="signedIn && accountLabel ? `${t('Your account')}: ${accountLabel}` : t('Sign in')"
        :compact="compact"
        popup="dialog"
        @click="emit('account')"
      >
        <template #icon
          ><span class="account-avatar" aria-hidden="true"
            ><template v-if="signedIn && accountInitial">{{ accountInitial }}</template
            ><AppIcon v-else name="user" /></span
        ></template>
        <span class="account-copy"
          ><strong>{{ t(signedIn ? 'Your account' : 'Sign in') }}</strong
          ><small>{{ signedIn && accountLabel ? accountLabel : t('Make it yours') }}</small></span
        >
      </NavigationItem>
    </div>
  </div>
</template>

<style scoped>
.navigation-panel {
  --page: #0d1016;
  --surface: #0d1016;
  --surface-raised: var(--navigation-tooltip, #232733);
  --subtle: var(--navigation-tooltip, #232733);
  --text: var(--navigation-text, #f3f0f7);
  --muted: var(--navigation-muted, #b7bfce);
  --accent: var(--navigation-accent, #bba3f5);
  --on-accent: var(--navigation-on-accent, #21172f);
  --divider: var(--navigation-edge, #2c303b);
  --control-border: var(--navigation-control-border, #857b94);
  color-scheme: dark;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 100%;
  padding: 24px 16px 16px;
  background: #0d1016;
  color: var(--text);
}
.panel-top {
  flex-shrink: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 6px;
  padding: 0 4px;
}
.quiet-control {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
.quiet-control svg {
  width: 20px;
  height: 20px;
}
.collapse-control {
  margin-top: -4px;
  margin-right: -8px;
}
.quiet-control:focus-visible,
.workspace-switch-row select:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
.primary-navigation,
.workspace-links,
.utility-navigation,
.secondary-navigation,
.test-navigation {
  display: grid;
  gap: 4px;
  flex-shrink: 0;
}
.primary-navigation {
  margin-top: 30px;
}
.workspace-navigation {
  flex-shrink: 0;
  margin-top: 28px;
  padding-top: 22px;
  border-top: 1px solid var(--divider);
}
.workspace-context {
  margin-bottom: 14px;
}
.group-label {
  display: block;
  padding: 0 12px;
  margin: 0 0 10px;
  color: var(--muted);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}
.workspace-switch-row {
  display: flex;
  align-items: center;
  gap: 4px;
}
.workspace-switch-row select {
  width: 100%;
  min-width: 0;
  height: 44px;
  padding: 0 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  color: var(--text);
  background: var(--navigation-hover, #1c202b);
  font: inherit;
  font-size: 13px;
}
.no-workspace {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 0 12px;
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.no-workspace svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}
.utility-navigation {
  margin-top: 10px;
}
.utility-navigation :deep(.nav-item) {
  font-size: 13px;
}
.test-navigation {
  margin-top: 28px;
}
.test-navigation :deep(.nav-item:not([aria-current])) {
  color: var(--navigation-muted, #bdb6c9);
}
.panel-bottom {
  margin-top: auto;
  padding-top: 32px;
  flex-shrink: 0;
}
.preferences {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  padding: 12px 0 8px;
  margin-top: 10px;
  border-top: 1px solid var(--divider);
}
.navigation-language :deep(.language-menu) {
  top: auto;
  bottom: calc(100% + 6px);
  right: auto;
  left: 0;
}
.account-button {
  min-height: 56px;
  padding: 8px;
}
.account-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--navigation-selected, #393047);
  color: var(--navigation-accent, #d5c4fb);
  font-size: 14px;
  font-weight: 650;
}
.account-copy {
  display: grid;
  gap: 4px;
}
.account-copy strong {
  font-size: 13px;
  color: var(--text);
  font-weight: 600;
}
.account-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  color: var(--muted);
}
.staff-label {
  margin-top: 30px;
}
.staff-label + .primary-navigation {
  margin-top: 0;
}
.compact {
  padding: 22px 12px 12px;
}
.compact .panel-top {
  flex-direction: column;
  align-items: center;
  padding: 0;
  gap: 16px;
}
.compact .collapse-control {
  margin: 0;
}
.compact .primary-navigation {
  margin-top: 22px;
}
.compact .workspace-navigation {
  margin-top: 18px;
  padding-top: 18px;
}
.compact .workspace-links {
  margin-top: 4px;
}
.compact .test-navigation {
  margin-top: 20px;
}
.compact .panel-bottom {
  padding-top: 20px;
}
.compact .preferences {
  flex-direction: column;
  padding-top: 12px;
}
.compact .navigation-language :deep(.language-trigger) {
  width: 44px;
  padding: 10px;
}
.compact .navigation-language :deep(.language-trigger > span),
.compact .navigation-language :deep(.language-chevron) {
  display: none;
}
.compact .navigation-language :deep(.language-globe) {
  width: 20px;
  height: 20px;
}
.compact .navigation-language :deep(.language-menu) {
  position: fixed;
  left: 76px;
  bottom: 76px;
  top: auto;
  z-index: 100;
}
.compact .account-button {
  padding: 5px;
  min-height: 44px;
}
@media (hover: hover) {
  .quiet-control:hover {
    color: var(--text);
    background: var(--subtle);
  }
}
@media (prefers-reduced-motion: no-preference) {
  .quiet-control {
    transition:
      background-color 150ms ease-out,
      color 150ms ease-out;
  }
}
</style>
