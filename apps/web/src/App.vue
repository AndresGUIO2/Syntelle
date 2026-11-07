<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onMounted,
  shallowRef,
  useTemplateRef,
  watch,
  watchEffect,
} from 'vue'
import { useLocaleStore } from './stores/locale'
import AppNavigation from './features/navigation/AppNavigation.vue'
import AppIcon from './components/AppIcon.vue'
import PricingPage from './features/pricing/PricingPage.vue'
import CreateCharacterPage from './features/characters/CreateCharacterPage.vue'
import WorkspacePanel from './features/workspaces/WorkspacePanel.vue'
import StaffPanel from './features/workspaces/StaffPanel.vue'
import WorkspaceCreation from './features/workspaces/WorkspaceCreation.vue'
import ConsoleOverview from './features/console/ConsoleOverview.vue'
import ConsoleCharacters from './features/console/ConsoleCharacters.vue'
import ConsoleIntegrations from './features/console/ConsoleIntegrations.vue'
import ConsoleUsage from './features/console/ConsoleUsage.vue'
import ConsoleBilling from './features/console/ConsoleBilling.vue'
import ConsoleTeam from './features/console/ConsoleTeam.vue'
import ConsoleRouteState from './features/console/ConsoleRouteState.vue'
import { useConnectionIntent, type ConnectionSelection } from './features/console/connectionIntent'
import { resetCharacterJourney } from './features/characters/useCharacterJourney'
import { resetIntegrationJourney } from './features/console/useIntegrationJourney'
import { useConsoleWorkspace } from './features/console/useConsoleWorkspace'
import './features/console/console.css'
import AccountDialog from './features/streaming/components/AccountDialog.vue'
import ConversationHistoryPanel from './features/history/ConversationHistoryPanel.vue'
import { useConversationHistory, type HistoryItem } from './features/history/useConversationHistory'
import { useAccount } from './features/streaming/composables/useAccount'
import { useChatSettings } from './features/streaming/composables/useChatSettings'
import { request } from './features/streaming/api'
import { usePageNavigation, type Destination } from './composables/usePageNavigation'
import { defaultSettings, type SavedConfiguration } from './features/streaming/types'
import type { ConversationTurn } from './features/conversation/types'
type ChatHandle = {
  stopForAuth: () => Promise<void>
  prepareHistorySelection: () => Promise<void>
  load: (setup: SavedConfiguration) => Promise<void>
  previewHistory: (item: HistoryItem, turns: ConversationTurn[]) => Promise<void>
  updateHistoryTurns: (id: string, turns: ConversationTurn[]) => void
  newChat: () => Promise<void>
}
const locale = useLocaleStore()
const { t } = locale
const navigation = usePageNavigation()
const auth = useAccount()
const workspace = useConsoleWorkspace(() => auth.account.value?.id)
const connection = useConnectionIntent(() => auth.account.value?.id)
function connectCharacter(selection: ConnectionSelection = {}) {
  if (!connection.begin(selection)) return
  if (selection.workspaceId) workspace.select(selection.workspaceId)
  navigation.visit('/console/integrations')
}
async function createConnectionWorkspace(name: string) {
  const owner = auth.account.value?.id
  const created = await workspace.create(name)
  const result = {
    workspace: created ? workspace.selected.value : null,
    uncertain: workspace.uncertainCreation.value,
    error: workspace.error.value,
  }
  if (owner === auth.account.value?.id) workspace.error.value = ''
  return result
}
watch([connection.intent, workspace.items, workspace.loading], ([intent, items, loading]) => {
  if (!intent?.workspaceId || loading || workspace.error.value || !auth.account.value) return
  if (items.some((item) => item.id === intent.workspaceId)) workspace.select(intent.workspaceId)
  else
    connection.unavailable(
      'This workspace is no longer available. Choose a workspace and connect the character again.',
    )
})
const workspaceDialog = useTemplateRef<HTMLDialogElement>('workspaceDialog')
const workspaceCreationOpen = shallowRef(false)
let workspaceReturnFocus: HTMLElement | null = null
function openWorkspaceCreation() {
  workspaceReturnFocus = document.activeElement as HTMLElement
  workspace.error.value = ''
  workspaceCreationOpen.value = true
  void nextTick(() => workspaceDialog.value?.showModal())
}
function closeWorkspaceCreation() {
  if (workspace.creating.value) return
  workspaceCreationOpen.value = false
  workspaceDialog.value?.close()
  workspaceReturnFocus?.focus()
}
async function createWorkspace(name: string) {
  if (await workspace.create(name)) {
    closeWorkspaceCreation()
    if (navigation.chat.value) navigation.visit('/console')
  }
}
const routeScope = computed(
  () =>
    `${auth.account.value?.id ?? 'guest'}:${workspace.selectedId.value}:${workspace.selected.value?.role ?? ''}:${workspace.selected.value?.suspended ?? false}`,
)
const financialAccess = computed(() =>
  ['owner', 'billing'].includes(workspace.selected.value?.role ?? ''),
)
const workspaceRoute = computed(() => navigation.consolePage.value)
const savedHistory = useConversationHistory(() => auth.account.value?.id)
const { settings, selectCharacter } = useChatSettings()
const accountReady = shallowRef(false)
const accountOpen = shallowRef(false)
const historyOpen = shallowRef(false)
const activeConversationId = shallowRef<string | null>(null)
let historyIntent = 0
let applyingHistory = false
const navigationShell = useTemplateRef<{ closeNavigation: () => void }>('navigationShell')
const staffEligible = shallowRef(false)
const chatGeneration = shallowRef(0)
const chatWorkspace = useTemplateRef<ChatHandle>('chatWorkspace')
const loadChatModule = () => import('./features/streaming/StreamWorkspace.vue')
const streamWorkspace = defineAsyncComponent(loadChatModule)
const signedIn = computed(() => Boolean(auth.account.value))
const accountLabel = computed(() => auth.account.value?.name || auth.account.value?.email || '')
const flow = computed(() => navigation.authQuery.value.get('flow_id') ?? undefined)
watch(navigation.selectedCharacter, (value) => selectCharacter(value), { immediate: true })
async function beforeLogin() {
  try {
    if (workspaceRoute.value || navigation.characters.value)
      sessionStorage.setItem('syntelle.console.auth-return', location.pathname)
  } catch {
    /* Optional route restoration. */
  }
  await chatWorkspace.value?.stopForAuth()
}
function restoreConsoleReturn() {
  let target = ''
  try {
    target = sessionStorage.getItem('syntelle.console.auth-return') ?? ''
    sessionStorage.removeItem('syntelle.console.auth-return')
  } catch {
    /* Optional route restoration. */
  }
  if (
    [
      '/console',
      '/console/integrations',
      '/console/usage',
      '/console/billing',
      '/console/team',
      '/console/settings',
      '/characters',
      '/characters/new',
    ].includes(target)
  ) {
    navigation.visit(target as Destination, true)
    return true
  }
  return false
}
async function accountChanged() {
  await auth.refresh()
  accountOpen.value = false
  if (signedIn.value && restoreConsoleReturn()) return
  if (signedIn.value && navigation.authQuery.value.get('return') === 'characters')
    navigation.visit('/characters', true)
  else if (
    navigation.authQuery.value.has('auth') ||
    flow.value ||
    navigation.authQuery.value.has('auth_error')
  )
    navigation.visit(signedIn.value ? '/console' : navigation.conversationHref.value, true)
}
async function ensureChat(character: string) {
  const owner = auth.account.value?.id
  const generation = chatGeneration.value
  const destination: Destination =
    character === 'syntelle' ? '/' : (`/?character=${character}` as Destination)
  if (!navigation.chat.value || navigation.selectedCharacter.value !== character)
    navigation.visit(destination)
  await loadChatModule()
  await nextTick()
  const current = () =>
    navigation.chat.value &&
    navigation.selectedCharacter.value === character &&
    auth.account.value?.id === owner &&
    chatGeneration.value === generation
  if (!current()) return
  if (chatWorkspace.value) return chatWorkspace.value
  // Module resolution can precede the async wrapper's mount, notably in WebKit.
  // Wait for the actual chat handle, while allowing navigation/owner changes to cancel.
  return new Promise<ChatHandle | undefined>((resolve) => {
    const stop = watch(
      [
        chatWorkspace,
        navigation.chat,
        navigation.selectedCharacter,
        chatGeneration,
        () => auth.account.value?.id,
      ],
      () => {
        if (current() && !chatWorkspace.value) return
        stop()
        resolve(current() ? (chatWorkspace.value ?? undefined) : undefined)
      },
      { flush: 'post' },
    )
  })
}
async function loadSetup(setup: SavedConfiguration) {
  historyIntent++
  applyingHistory = false
  savedHistory.clearSelection()
  activeConversationId.value = null
  const chat = await ensureChat(setup.settings.character)
  if (chat) await chat.load(setup)
}
function cancelHistoryIntent() {
  historyOpen.value = false
  historyIntent++
  applyingHistory = false
  savedHistory.clearSelection()
}
async function openSavedConversation(item: HistoryItem) {
  const intent = ++historyIntent
  const owner = auth.account.value?.id
  const from = location.pathname + location.search
  applyingHistory = true
  try {
    await chatWorkspace.value?.prepareHistorySelection()
    if (
      intent !== historyIntent ||
      owner !== auth.account.value?.id ||
      from !== location.pathname + location.search
    )
      return
    const preview = await savedHistory.select(item)
    if (
      !preview?.item.settings ||
      intent !== historyIntent ||
      owner !== auth.account.value?.id ||
      from !== location.pathname + location.search
    )
      return
    navigationShell.value?.closeNavigation()
    historyOpen.value = false
    const chat = await ensureChat(preview.item.settings.character)
    if (chat && intent === historyIntent && savedHistory.selectedId.value === preview.item.id)
      await chat.previewHistory(preview.item, preview.turns)
  } finally {
    if (intent === historyIntent) {
      applyingHistory = false
      savedHistory.setSelection(activeConversationId.value)
    }
  }
}
function conversationChanged(id: string | null) {
  activeConversationId.value = id
  if (!applyingHistory) savedHistory.setSelection(id)
}
async function deletedConversation(id: string) {
  if (activeConversationId.value !== id) return
  activeConversationId.value = null
  savedHistory.clearSelection()
  if (chatWorkspace.value) await chatWorkspace.value.newChat()
  else chatGeneration.value++
}
async function loadOlderHistory() {
  const preview = await savedHistory.loadOlder()
  if (preview && preview.item.id === activeConversationId.value)
    chatWorkspace.value?.updateHistoryTurns(preview.item.id, preview.turns)
}
watch(
  () => auth.account.value?.id,
  async (id, previous, cleanup) => {
    staffEligible.value = false
    if (accountReady.value && id !== previous) {
      resetCharacterJourney()
      resetIntegrationJourney()
      connection.clear()
      workspaceDialog.value?.close()
      workspaceCreationOpen.value = false
      workspaceReturnFocus = null
      historyIntent++
      historyOpen.value = false
      applyingHistory = false
      activeConversationId.value = null
      chatGeneration.value++
      auth.forgetSecret()
      if (previous) {
        settings.value = {
          ...defaultSettings(),
          language: locale.speechLanguage,
          reduced_motion: matchMedia('(prefers-reduced-motion: reduce)').matches,
        }
        navigation.selectedCharacter.value = 'syntelle'
        if (navigation.chat.value) navigation.visit('/', true)
      }
    }
    if (!id) return
    let current = true
    cleanup(() => {
      current = false
    })
    try {
      await request('/api/v1/admin/eligibility')
      if (current) staffEligible.value = true
    } catch {
      /* staff is separately granted */
    }
  },
)
watch([accountReady, navigation.characters, signedIn], ([ready, characters, authenticated]) => {
  if (ready && characters && !authenticated) accountOpen.value = true
})
watch([accountReady, navigation.pricing, signedIn], ([ready, pricing, authenticated]) => {
  if (ready && pricing && authenticated)
    navigation.visit(financialAccess.value ? '/console/billing' : '/console', true)
})
watch(navigation.active, () => {
  if (auth.account.value) void workspace.refresh()
})
watch(navigation.chat, (chat, previous) => {
  if (!chat) historyOpen.value = false
  if (previous && !chat && signedIn.value) void savedHistory.refresh()
})
watch(
  () => navigation.authQuery.value.toString(),
  () => {
    if (
      navigation.authQuery.value.has('auth') ||
      flow.value ||
      navigation.authQuery.value.has('auth_error')
    )
      accountOpen.value = true
  },
  { immediate: true },
)
onMounted(async () => {
  await auth.refresh()
  if (signedIn.value && !restoreConsoleReturn() && location.pathname === '/' && !location.search)
    navigation.visit('/console', true)
  if (
    signedIn.value &&
    navigation.workspaces.value &&
    !navigation.authQuery.value.has('invite') &&
    !navigation.authQuery.value.has('transfer')
  )
    navigation.visit('/console/team', true)
  accountReady.value = true
  if (navigation.authQuery.value.has('auth_error'))
    auth.reportError('Sign-in could not finish. Try again or continue as a guest.')
  if (signedIn.value && navigation.authQuery.value.get('return') === 'characters') {
    accountOpen.value = false
    navigation.visit('/characters', true)
  }
})
watchEffect(() => {
  const title =
    navigation.active.value === 'chat'
      ? ''
      : t(
          {
            characters: 'Characters',
            workspaces: 'Workspaces',
            admin: 'Administration',
            pricing: 'Pricing',
            overview: 'Overview',
            integrations: 'Integrations',
            usage: 'Usage',
            billing: 'Billing',
            team: 'Team',
            settings: 'Workspace settings',
          }[navigation.active.value],
        )
  document.title = title ? `${title} · Syntelle` : 'Syntelle'
})
</script>
<template>
  <AppNavigation
    ref="navigationShell"
    :signed-in="signedIn"
    :account-label="accountLabel"
    :staff-eligible="staffEligible"
    :active="navigation.active.value"
    :conversation-href="navigation.conversationHref.value"
    :workspaces="workspace.items.value"
    :workspace-id="workspace.selectedId.value"
    @workspace="workspace.select"
    @create-workspace="openWorkspaceCreation"
    @navigate="navigation.follow"
    @account="accountOpen = true"
  >
    <a class="skip-link" href="#main-content">{{ t('Skip to content') }}</a>
    <main
      id="main-content"
      class="main-content"
      :class="{
        'conversation-content': navigation.chat.value,
        'console-content': !navigation.chat.value,
      }"
      tabindex="-1"
    >
      <div
        v-if="connection.error.value && !navigation.chat.value"
        class="console-notice error"
        role="alert"
      >
        {{ t(connection.error.value) }}
        <button class="console-button quiet" @click="connection.clear">{{ t('Dismiss') }}</button>
      </div>
      <KeepAlive :key="chatGeneration" :max="1">
        <component
          :is="streamWorkspace"
          v-if="navigation.chat.value && accountReady"
          :key="navigation.selectedCharacter.value"
          ref="chatWorkspace"
          v-model="settings"
          :auth="auth"
          :character-id="navigation.selectedCharacter.value"
          @navigate="navigation.follow"
          @conversation-changed="conversationChanged"
          @conversation-updated="savedHistory.refresh()"
          @new-conversation="cancelHistoryIntent"
          ><template v-if="signedIn" #conversation-actions>
            <button
              class="history-trigger"
              type="button"
              :aria-label="t('Conversation history')"
              :title="t('Conversation history')"
              aria-haspopup="dialog"
              :aria-expanded="historyOpen"
              @click="historyOpen = true"
            >
              <AppIcon name="history" /><span>{{ t('History') }}</span>
            </button> </template
          ><template #history-tools>
            <button
              v-if="
                savedHistory.olderBefore.value &&
                savedHistory.selectedId.value === activeConversationId
              "
              class="older-messages"
              type="button"
              :disabled="savedHistory.loadingOlder.value"
              @click="loadOlderHistory"
            >
              {{ t('Load earlier messages') }}
            </button>
          </template></component
        >
      </KeepAlive>
      <PricingPage
        v-if="navigation.pricing.value && accountReady && !signedIn"
        @navigate="navigation.follow"
      />
      <CreateCharacterPage
        v-else-if="navigation.characterCreation.value && accountReady && signedIn"
        :key="auth.account.value?.id"
        :owner-id="auth.account.value!.id"
        mode="create"
        :conversation-href="navigation.conversationHref.value"
        @navigate="navigation.follow"
        @connect="connectCharacter"
      />
      <ConsoleCharacters
        v-else-if="navigation.characters.value && signedIn"
        :key="routeScope"
        :owner-id="auth.account.value!.id"
        :workspace="workspace.selected.value"
        @navigate="navigation.follow"
        @connect="connectCharacter"
      />
      <ConsoleOverview
        v-else-if="
          navigation.consolePage.value && navigation.consoleSection.value === 'overview' && signedIn
        "
        :key="routeScope"
        :workspace="workspace.selected.value"
        :loading="workspace.loading.value"
        :error="workspace.error.value"
        @retry="workspace.refresh"
        @navigate="navigation.follow"
        @connect="connectCharacter()"
      />
      <ConsoleIntegrations
        v-else-if="
          navigation.consolePage.value &&
          navigation.consoleSection.value === 'integrations' &&
          signedIn
        "
        :key="auth.account.value!.id"
        :account-id="auth.account.value!.id"
        :workspace="workspace.selected.value"
        :workspace-loading="workspace.loading.value"
        :workspace-error="workspace.error.value"
        :create-workspace="createConnectionWorkspace"
        :intent="connection.intent.value"
        @clear-intent="connection.clear"
        @retry-workspace="workspace.refresh"
        @navigate="navigation.follow"
      />
      <ConsoleRouteState
        v-else-if="workspaceRoute && signedIn && !workspace.selected.value"
        :section="navigation.consoleSection.value"
        :loading="workspace.loading.value"
        :error="workspace.error.value"
        @retry="workspace.refresh"
        @navigate="navigation.follow"
      />
      <ConsoleUsage
        v-else-if="
          navigation.consolePage.value &&
          navigation.consoleSection.value === 'usage' &&
          signedIn &&
          workspace.selected.value
        "
        :key="routeScope"
        :workspace="workspace.selected.value"
      />
      <ConsoleBilling
        v-else-if="
          navigation.consolePage.value &&
          navigation.consoleSection.value === 'billing' &&
          signedIn &&
          workspace.selected.value
        "
        :key="routeScope"
        :workspace="workspace.selected.value"
      />
      <ConsoleTeam
        v-else-if="
          navigation.consolePage.value &&
          ['team', 'settings'].includes(navigation.consoleSection.value) &&
          signedIn &&
          workspace.selected.value
        "
        :key="routeScope + navigation.consoleSection.value"
        :workspace="workspace.selected.value"
        :settings="navigation.consoleSection.value === 'settings'"
        @navigate="navigation.follow"
      />
      <WorkspacePanel
        v-else-if="navigation.workspaces.value && signedIn"
        :key="auth.account.value?.id"
        :conversation-href="navigation.conversationHref.value"
        @navigate="navigation.follow"
      />
      <StaffPanel
        v-else-if="navigation.admin.value && signedIn"
        :key="auth.account.value?.id"
        :conversation-href="navigation.conversationHref.value"
        @navigate="navigation.follow"
      />
      <section v-else-if="workspaceRoute && signedIn" class="access-state">
        <h1>{{ t('This section is not available for your workspace role.') }}</h1>
        <a class="console-button" href="/console" @click="navigation.follow">{{ t('Overview') }}</a>
      </section>
      <section v-else-if="!accountReady" class="access-state" role="status">
        {{ t('Checking your account…') }}
      </section>
      <section v-else-if="!navigation.chat.value && !navigation.pricing.value" class="access-state">
        <h1>{{ t(navigation.characters.value ? 'Your characters' : 'Sign in to continue.') }}</h1>
        <p>{{ t('Sign in to access your personal space.') }}</p>
        <button class="button button-primary" type="button" @click="accountOpen = true">
          {{ t('Sign in') }}
        </button>
      </section>
    </main>
    <dialog
      ref="workspaceDialog"
      class="console-dialog workspace-create-dialog"
      :aria-label="t('Create workspace')"
      @cancel.prevent="closeWorkspaceCreation"
    >
      <WorkspaceCreation
        v-if="workspaceCreationOpen"
        :busy="workspace.creating.value"
        :error="workspace.error.value"
        dismissible
        @create="createWorkspace"
        @cancel="closeWorkspaceCreation"
      />
    </dialog>
    <AccountDialog
      v-model="accountOpen"
      :auth="auth"
      :settings="settings"
      :before-login="beforeLogin"
      :flow="flow"
      @load="loadSetup"
      @signed-in="accountChanged"
      @signed-out="accountChanged"
    />
    <ConversationHistoryPanel
      v-if="signedIn && navigation.chat.value"
      v-model="historyOpen"
      :history="savedHistory"
      @open="openSavedConversation"
      @deleted="deletedConversation"
    />
  </AppNavigation>
</template>
<style scoped>
.main-content {
  width: 100%;
  max-width: 1280px;
  height: 100%;
  min-width: 0;
  min-height: 0;
  margin: 0 auto;
  padding: 48px 40px;
  overflow-y: auto;
  overscroll-behavior: contain;
}
.older-messages {
  align-self: center;
  min-height: 44px;
  padding: 8px 16px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.older-messages:focus-visible {
  outline: 2px solid var(--accent);
}
.history-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 44px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  color: var(--text);
  background: transparent;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.history-trigger > svg {
  width: 20px;
  height: 20px;
}
.history-trigger:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
@media (hover: hover) {
  .history-trigger:hover {
    background: var(--subtle);
  }
}
@media (max-width: 1100px) {
  .history-trigger {
    width: 44px;
    padding: 0;
  }
  .history-trigger > span {
    display: none;
  }
}
.main-content:focus {
  outline: none;
}
.access-state {
  min-height: 60%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  text-align: center;
  color: var(--muted);
}
.access-state h1 {
  color: var(--text);
  margin: 0;
}
@media (max-width: 899px) {
  .main-content {
    padding: 32px 20px;
  }
}
@media (max-width: 480px) {
  .main-content {
    padding: 24px 16px;
  }
}
.main-content.conversation-content {
  max-width: 1520px;
  padding: 24px clamp(20px, 2.5vw, 40px);
}
.workspace-onboarding {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(360px, 520px);
  gap: 64px;
  align-items: center;
  min-height: 70%;
}
.onboarding-heading > p:first-child {
  color: var(--accent);
  font-size: 13px;
  font-weight: 600;
}
.onboarding-heading h1 {
  font-size: clamp(30px, 3vw, 44px);
  line-height: 1.15;
  font-weight: 650;
  letter-spacing: -1.4px;
  max-width: 560px;
  margin: 20px 0;
}
.onboarding-heading > p:last-child {
  color: var(--muted);
  font-size: 15px;
  line-height: 1.7;
  max-width: 460px;
}
.workspace-onboarding > :deep(.workspace-creation) {
  border-radius: 16px;
  background: var(--surface);
}
.workspace-create-dialog {
  padding: 0;
}
@media (max-width: 1100px) {
  .workspace-onboarding {
    grid-template-columns: 1fr;
    gap: 24px;
    max-width: 600px;
    margin: 0 auto;
  }
}
@media (max-width: 760px) {
  .main-content.conversation-content {
    padding: 16px;
  }
}
</style>
