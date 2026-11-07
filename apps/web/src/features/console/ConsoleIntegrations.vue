<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import { request, RequestError } from '../streaming/api'
import { defaultSettings } from '../streaming/types'
import type { PersonalCharacter } from '../characters/api'
import type { Workspace } from '../workspaces/useWorkspaces'
import ConsoleHeading from './ConsoleHeading.vue'
import IntegrationDetails from './IntegrationDetails.vue'
import IntegrationSetup from './IntegrationSetup.vue'
import type { ConnectionIntent } from './connectionIntent'
import type { Integration, Publication } from './types'
import {
  bindIntegrationWorkspace,
  discardIntegrationJourney,
  getIntegrationJourney,
  normalizeWebsiteOrigins,
  type WorkspaceCreationResult,
} from './useIntegrationJourney'

const props = withDefaults(
  defineProps<{
    workspace: Workspace | null
    accountId: string
    intent?: ConnectionIntent | null
    createWorkspace: (name: string) => Promise<WorkspaceCreationResult>
    workspaceLoading?: boolean
    workspaceError?: string
  }>(),
  { intent: null, workspaceLoading: false, workspaceError: '' },
)
const emit = defineEmits<{ navigate: [event: MouseEvent]; clearIntent: []; retryWorkspace: [] }>()
const { t } = useLocaleStore()
const items = shallowRef<Integration[]>([])
const publications = shallowRef<Publication[]>([])
const characters = shallowRef<PersonalCharacter[]>([])
const loading = shallowRef(false)
const error = shallowRef('')
const selectedId = shallowRef('')
const draft = shallowRef(getIntegrationJourney(props.accountId, props.workspace?.id))
const manage = computed(
  () => !props.workspace || ['owner', 'administrator'].includes(props.workspace.role),
)
const operational = computed(() => !props.workspace || props.workspace.role !== 'billing')
const current = computed(() => items.value.find((item) => item.id === selectedId.value) ?? null)
const availablePublications = computed(() => publications.value.filter((item) => !item.revoked_at))
const recoverable = computed(() =>
  draft.value.uncertain === 'integration'
    ? items.value.filter(
        (item) =>
          item.name === draft.value.name.trim() &&
          item.publication_id === draft.value.publicationId,
      )
    : [],
)
let epoch = 0
let disposed = false
const message = (failure: unknown) =>
  failure instanceof Error
    ? failure.message
    : 'This request could not be completed. Please try again.'
const ambiguous = (failure: unknown) => !(failure instanceof RequestError) || failure.status >= 500
function base(id = props.workspace?.id) {
  return `/api/v1/workspaces/${encodeURIComponent(id ?? '')}`
}
async function refresh() {
  const revision = ++epoch,
    owner = props.accountId,
    wid = props.workspace?.id
  loading.value = true
  error.value = ''
  try {
    const [personal, shared, connections] = await Promise.all([
      request<{ items: PersonalCharacter[] }>('/api/v1/characters'),
      wid && operational.value
        ? request<{ items: Publication[] }>(base(wid) + '/publications')
        : Promise.resolve({ items: [] }),
      wid && operational.value
        ? request<{ items: Integration[] }>(base(wid) + '/integrations')
        : Promise.resolve({ items: [] }),
    ])
    if (disposed || revision !== epoch || owner !== props.accountId || wid !== props.workspace?.id)
      return
    characters.value = personal.items
    publications.value = shared.items
    items.value = connections.items
    if (selectedId.value && !connections.items.some((item) => item.id === selectedId.value))
      selectedId.value = ''
    consumeIntent()
  } catch (failure) {
    if (!disposed && revision === epoch) error.value = message(failure)
  } finally {
    if (!disposed && revision === epoch) loading.value = false
  }
}
function consumeIntent() {
  const intent = props.intent
  if (
    props.workspaceLoading ||
    props.workspaceError ||
    !intent ||
    intent.accountId !== props.accountId ||
    (intent.workspaceId && intent.workspaceId !== props.workspace?.id)
  )
    return
  if (!manage.value || props.workspace?.suspended) {
    error.value = props.workspace?.suspended
      ? 'This workspace is suspended.'
      : 'Your workspace role does not allow creating website connections.'
    emit('clearIntent')
    return
  }
  const selected = intent.publicationId
    ? availablePublications.value.find((item) => item.id === intent.publicationId)
    : undefined
  if (intent.publicationId && !selected) {
    error.value = 'This shared character is no longer available. Choose another character.'
    emit('clearIntent')
    return
  }
  const characterId = selected?.character_id ?? intent.characterId ?? 'syntelle'
  if (
    !selected &&
    characterId !== 'syntelle' &&
    !characters.value.some((item) => item.id === characterId && item.status === 'ready')
  ) {
    error.value = 'This character is not ready or is no longer available.'
    emit('clearIntent')
    return
  }
  if (!draft.value.busy) {
    draft.value.characterId = characterId
    const matches = availablePublications.value.filter((item) => item.character_id === characterId)
    draft.value.publicationId = selected?.id ?? (matches.length === 1 ? matches[0]!.id : '')
    draft.value.sharingApproved = false
    start()
    emit('clearIntent')
  }
}
function start() {
  if (
    !manage.value ||
    props.workspace?.suspended ||
    props.workspaceLoading ||
    props.workspaceError ||
    error.value
  )
    return
  draft.value.open = true
  if (!props.workspace) draft.value.step = 'workspace'
  else if (draft.value.step === 'workspace') draft.value.step = 'character'
  if (!draft.value.publicationId && draft.value.step === 'character') {
    const matches = availablePublications.value.filter(
      (item) => item.character_id === draft.value.characterId,
    )
    if (matches.length === 1) draft.value.publicationId = matches[0]!.id
  }
}
function cancel() {
  if (draft.value.busy) return
  discardIntegrationJourney(draft.value)
  draft.value = getIntegrationJourney(props.accountId, props.workspace?.id)
  emit('clearIntent')
}
async function submitWorkspace() {
  const state = draft.value
  if (state.busy || state.uncertain || !state.workspaceName.trim() || props.workspace) return
  state.busy = true
  state.creatingWorkspace = true
  state.error = ''
  try {
    const result = await props.createWorkspace(state.workspaceName.trim())
    const workspace = result.workspace
    if (!workspace) {
      state.uncertain = result.uncertain ? 'workspace' : ''
      state.error =
        result.error ||
        'Workspace creation could not be confirmed. Check your workspaces before starting another setup.'
      return
    }
    if (!bindIntegrationWorkspace(state, workspace.id)) return
    state.step = 'character'
    state.uncertain = ''
    const selectedWorkspace = currentWorkspace()
    if (selectedWorkspace?.id !== workspace.id)
      draft.value = getIntegrationJourney(props.accountId, selectedWorkspace?.id)
    if (!disposed) await refresh()
  } catch (failure) {
    state.uncertain = ambiguous(failure) ? 'workspace' : ''
    state.error = message(failure)
  } finally {
    state.busy = false
    state.creatingWorkspace = false
  }
}
function currentWorkspace(): Workspace | null {
  return props.workspace
}
function useSelectedWorkspace() {
  const workspace = props.workspace,
    state = draft.value
  if (
    !workspace ||
    state.busy ||
    state.uncertain !== 'workspace' ||
    !manage.value ||
    workspace.suspended
  )
    return
  if (!bindIntegrationWorkspace(state, workspace.id)) return
  state.step = 'character'
  state.uncertain = ''
  state.error = ''
}
function finish(item: Integration) {
  selectedId.value = item.id
  if (!items.value.some((existing) => existing.id === item.id)) items.value = [...items.value, item]
  discardIntegrationJourney(draft.value)
  draft.value = getIntegrationJourney(props.accountId, props.workspace?.id)
}
async function save() {
  const state = draft.value,
    workspace = props.workspace
  if (state.busy || state.uncertain || !workspace || !manage.value || workspace.suspended) return
  if (state.workspaceId !== workspace.id || state.accountId !== props.accountId) return
  state.busy = true
  state.error = ''
  let operation: 'publication' | 'integration' = 'publication'
  let mutationStarted = false
  try {
    const origins = normalizeWebsiteOrigins(state.origins)
    if (!state.name.trim() || state.name.trim().length > 100)
      throw new Error('Give this connection a name.')
    if (!Number.isInteger(state.minutes) || state.minutes < 1 || state.minutes > 1440)
      throw new Error('Use a daily allowance between 1 and 1440 minutes.')
    // Refresh the selected identities before any sharing or integration mutation.
    const shared = await request<{ items: Publication[] }>(base(workspace.id) + '/publications')
    if (disposed || props.workspace?.id !== workspace.id) return
    publications.value = shared.items
    let publication = shared.items.find(
      (item) => item.id === state.publicationId && !item.revoked_at,
    )
    if (state.publicationId && !publication) {
      state.error = 'This shared character is no longer available. Choose another character.'
      state.step = 'character'
      return
    }
    if (!publication) {
      if (!state.sharingApproved) return
      const personal = await request<{ items: PersonalCharacter[] }>('/api/v1/characters')
      if (disposed || props.workspace?.id !== workspace.id) return
      characters.value = personal.items
      const character = personal.items.find(
        (item) => item.id === state.characterId && item.status === 'ready',
      )
      if (state.characterId !== 'syntelle' && !character) {
        state.error = 'This character is not ready or is no longer available.'
        state.step = 'character'
        return
      }
      mutationStarted = true
      publication = await request<Publication>(base(workspace.id) + '/publications', 'POST', {
        creation_id: state.characterId,
        name: character?.name ?? 'Syntelle',
        settings: { ...defaultSettings(), character: state.characterId },
      })
      // Preserve the successful copy even when the user navigates away before the second request.
      state.publicationId = publication.id
      if (disposed || props.workspace?.id !== workspace.id) return
      publications.value = [...publications.value, publication]
    }
    operation = 'integration'
    mutationStarted = true
    if (!manage.value || props.workspace?.suspended) return
    const result = await request<Integration>(base(workspace.id) + '/integrations', 'POST', {
      name: state.name.trim(),
      publication_id: publication.id,
      allowed_origins: origins,
      capabilities: ['message', 'speak', 'interrupt', 'suspend'],
      daily_limit_seconds: state.minutes * 60,
      appearance: { ...state.appearance },
    })
    if (disposed || props.workspace?.id !== workspace.id) {
      state.open = false
      return
    }
    finish(result)
    await refresh()
  } catch (failure) {
    state.error = message(failure)
    // Validation errors before a mutation do not create an uncertain server action.
    if (mutationStarted && ambiguous(failure)) state.uncertain = operation
    if (!disposed && props.workspace?.id === workspace.id) await refresh()
  } finally {
    state.busy = false
  }
}
function recover(id: string) {
  const item = recoverable.value.find((item) => item.id === id)
  if (item) finish(item)
}
function website(item: Integration) {
  return item.allowed_origins[0] ?? t('Website not set')
}
function characterName(item: Integration) {
  return (
    publications.value.find((value) => value.id === item.publication_id)?.name ??
    t('Character unavailable')
  )
}
function status(item: Integration) {
  return item.enabled && item.published_revision
    ? item.revision > item.published_revision
      ? 'Unpublished changes'
      : 'Published'
    : 'Draft or disabled'
}
watch(
  () =>
    [
      props.accountId,
      props.workspace?.id,
      props.workspace?.role,
      props.workspace?.suspended,
    ] as const,
  () => {
    const state = draft.value
    // Selection can change while a create request is pending. Only its returned ID,
    // or an explicit recovery choice, may bind the in-progress journey.
    if (!(
      state.accountId === props.accountId &&
      (state.creatingWorkspace || state.uncertain === 'workspace')
    ))
      draft.value = getIntegrationJourney(props.accountId, props.workspace?.id)
    items.value = []
    publications.value = []
    selectedId.value = ''
    error.value = ''
    void refresh()
  },
  { immediate: true },
)
watch(
  () => [props.intent, props.workspaceLoading, props.workspaceError],
  () => {
    if (!loading.value && !error.value) consumeIntent()
  },
)
onScopeDispose(() => {
  disposed = true
  epoch++
})
</script>
<template>
  <section
    class="console-page integrations-page"
    :class="{ 'integrations-page--setup': draft.open }"
  >
    <ConsoleHeading
      title="Integrations"
      description="Choose where your characters meet your visitors."
      :workspace="workspace?.name"
    >
      <button
        v-if="manage && !draft.open"
        class="console-button primary"
        :disabled="
          workspaceLoading || !!workspaceError || loading || !!error || workspace?.suspended
        "
        @click="start"
      >
        <AppIcon name="plus" />{{ t('Connect a website') }}
      </button>
    </ConsoleHeading>
    <p v-if="workspaceLoading" role="status">{{ t('Loading your workspace…') }}</p>
    <div v-else-if="workspaceError" class="console-notice error" role="alert">
      <p>{{ t(workspaceError) }}</p>
      <button class="console-button" @click="emit('retryWorkspace')">{{ t('Try again') }}</button>
    </div>
    <p v-else-if="!operational" class="console-notice">
      {{ t('Your workspace role does not include integration access.') }}
    </p>
    <p v-else-if="workspace?.suspended" class="console-notice error">
      {{ t('This workspace is suspended.') }}
    </p>
    <template v-else>
      <div v-if="error" class="console-notice error" role="alert">
        <p>{{ t(error) }}</p>
        <button class="console-button" :disabled="loading" @click="refresh">
          {{ t('Try again') }}
        </button>
      </div>
      <p v-if="loading && !items.length && !draft.open" role="status">{{ t('Loading…') }}</p>
      <template v-if="draft.open && manage">
        <IntegrationSetup
          v-model="draft"
          :characters="characters"
          :publications="availablePublications"
          :workspace-name="workspace?.name"
          :loading="loading"
          @create-workspace="submitWorkspace"
          @save="save"
          @cancel="cancel"
          @refresh="refresh"
        />
        <div v-if="draft.uncertain === 'workspace'" class="setup-recovery">
          <button class="console-button" @click="emit('retryWorkspace')">
            {{ t('Check workspaces') }}</button
          ><template v-if="workspace"
            ><p>
              {{
                t(
                  'Review the selected workspace before continuing. No new workspace will be created.',
                )
              }}
            </p>
            <button class="console-button primary" @click="useSelectedWorkspace">
              {{ t('Use selected workspace') }}: {{ workspace.name }}
            </button></template
          >
        </div>
        <div v-if="draft.uncertain === 'publication'" class="setup-recovery">
          <button class="console-button" @click="draft.step = 'character'">
            {{ t('Choose an existing workspace copy') }}
          </button>
        </div>
        <div v-if="recoverable.length" class="setup-recovery">
          <h3>{{ t('Matching saved connections') }}</h3>
          <button
            v-for="item in recoverable"
            :key="item.id"
            class="console-button"
            @click="recover(item.id)"
          >
            {{ t('Open saved connection') }}: {{ item.name }}
          </button>
        </div>
      </template>
      <template v-else-if="!loading && !error">
        <div v-if="!items.length" class="website-empty">
          <div class="website-illustration" aria-hidden="true">
            <div class="browser-dots"><i /><i /><i /></div>
            <div class="browser-body">
              <AppIcon name="globe" /><span /><span />
              <div class="browser-launcher"><AppIcon name="message" /></div>
            </div>
          </div>
          <div>
            <p class="empty-eyebrow">{{ t('Your websites and apps') }}</p>
            <h2>{{ t('Give visitors someone to talk to') }}</h2>
            <p>
              {{
                t(
                  'Add a character to your website with a floating button or an embedded player. Choose the character, review the connection, then publish.',
                )
              }}
            </p>
            <button v-if="manage" class="console-button primary" @click="start">
              {{ t('Connect your first website') }}<AppIcon name="external" />
            </button>
            <p v-else class="muted">
              {{ t('Ask a workspace owner or administrator to connect a website.') }}
            </p>
          </div>
        </div>
        <div v-else class="connections-content">
          <ul class="website-list" :aria-label="t('Website connections')">
            <li v-for="item in items" :key="item.id">
              <button
                :aria-current="current?.id === item.id ? 'true' : undefined"
                @click="selectedId = current?.id === item.id ? '' : item.id"
              >
                <span class="website-icon"><AppIcon name="globe" /></span
                ><span class="website-title"
                  ><strong>{{ item.name }}</strong
                  ><small>{{ website(item) }}</small></span
                ><span class="website-character">{{ characterName(item) }}</span
                ><span
                  class="console-pill"
                  :class="{ active: item.enabled && item.published_revision }"
                  >{{ t(status(item)) }}</span
                ><AppIcon name="external" />
              </button>
            </li>
          </ul>
          <IntegrationDetails
            v-if="current && workspace"
            :key="workspace.id + ':' + current.id"
            :workspace="workspace"
            :account-id="accountId"
            :item="current"
            :publications="availablePublications"
            @changed="refresh"
          />
          <p v-else class="connection-hint">
            {{ t('Select a website to review its settings and installation.') }}
          </p>
        </div>
      </template>
    </template>
  </section>
</template>
<style scoped>
.console-page.integrations-page {
  --console-page-width: 1120px;
}
.console-page.integrations-page--setup {
  --console-page-width: 760px;
}
.website-empty {
  display: grid;
  grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);
  align-items: center;
  gap: 48px;
  padding: 40px;
  background: var(--surface);
  border-radius: 16px;
}
.website-empty h2 {
  font-size: 24px;
  line-height: 1.25;
  max-width: 440px;
}
.website-empty p:not(.empty-eyebrow) {
  color: var(--muted);
  max-width: 480px;
  margin: 12px 0 24px;
}
.empty-eyebrow {
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
  margin: 0 0 12px;
}
.website-illustration {
  width: 100%;
  border: 1px solid var(--control-border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--page);
}
.browser-dots {
  display: flex;
  gap: 5px;
  padding: 13px;
  border-bottom: 1px solid var(--divider);
}
.browser-dots i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--muted);
}
.browser-body {
  padding: 24px;
  position: relative;
  height: 170px;
}
.browser-body > svg {
  width: 30px;
  height: 30px;
  color: var(--muted);
  margin-bottom: 20px;
}
.browser-body > span {
  display: block;
  width: 68%;
  height: 7px;
  border-radius: 4px;
  background: var(--divider);
  margin-bottom: 10px;
}
.browser-body > span + span {
  width: 45%;
}
.browser-launcher {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: var(--accent);
  color: var(--on-accent);
  position: absolute;
  bottom: 20px;
  right: 20px;
}
.website-list {
  list-style: none;
  margin: 0 0 28px;
  padding: 0;
  background: var(--surface);
  border-radius: 12px;
  overflow: hidden;
}
.website-list li + li {
  border-top: 1px solid var(--divider);
}
.website-list button {
  width: 100%;
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) minmax(100px, 160px) auto 16px;
  align-items: center;
  gap: 18px;
  padding: 20px 24px;
  background: transparent;
  border: 0;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.website-list button[aria-current] {
  background: var(--subtle);
  box-shadow: inset 3px 0 var(--accent);
}
.website-list button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -3px;
}
.website-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  background: var(--subtle);
  border-radius: 10px;
  flex-shrink: 0;
  color: var(--accent);
}
.website-title {
  min-width: 0;
  flex: 1;
}
.website-title strong {
  font-size: 15px;
  display: block;
  overflow-wrap: anywhere;
}
.website-title small {
  display: block;
  color: var(--muted);
  font-size: 12px;
  margin-top: 6px;
  overflow-wrap: anywhere;
}
.website-character {
  font-size: 13px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.website-list button > svg {
  color: var(--muted);
  width: 16px;
  flex-shrink: 0;
}
.connection-hint {
  font-size: 14px;
  color: var(--muted);
  text-align: center;
  padding: 28px;
}
.setup-recovery {
  margin-top: 24px;
  display: grid;
  gap: 12px;
  max-width: 760px;
}
@media (hover: hover) {
  .website-list button:hover {
    background: var(--subtle);
  }
}
@media (max-width: 800px) {
  .website-empty {
    grid-template-columns: minmax(140px, 190px) minmax(0, 1fr);
    gap: 24px;
    padding: 28px;
  }
  .browser-body {
    height: 150px;
  }
  .website-character {
    display: none;
  }
  .website-list button {
    grid-template-columns: 44px minmax(0, 1fr) auto 16px;
  }
}
@media (max-width: 600px) {
  .website-list button {
    grid-template-columns: 40px minmax(0, 1fr) 16px;
    gap: 12px 14px;
    padding: 18px;
  }
  .website-icon {
    width: 40px;
    height: 40px;
    align-self: start;
  }
  .website-title {
    grid-column: 2;
  }
  .website-list .console-pill {
    grid-column: 2;
    justify-self: start;
  }
  .website-list button > svg {
    grid-column: 3;
    grid-row: 1;
    align-self: center;
  }
  .website-empty {
    grid-template-columns: 1fr;
    padding: 24px;
  }
  .website-illustration {
    max-width: 180px;
  }
  .browser-body {
    height: 110px;
    padding: 18px;
  }
  .browser-body > svg {
    width: 24px;
    height: 24px;
    margin-bottom: 14px;
  }
  .browser-launcher {
    width: 32px;
    height: 32px;
    right: 14px;
    bottom: 14px;
  }
  .website-empty h2 {
    font-size: 22px;
  }
  .website-empty .console-button {
    width: 100%;
  }
}
</style>
