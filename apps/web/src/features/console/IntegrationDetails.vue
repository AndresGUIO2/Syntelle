<script setup lang="ts">
import { computed, nextTick, onMounted, onScopeDispose, shallowRef, useTemplateRef } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import { request, RequestError } from '../streaming/api'
import type { Workspace } from '../workspaces/useWorkspaces'
import IntegrationForm from './IntegrationForm.vue'
import { clearIntegrationEditDraft } from './useIntegrationJourney'
import { useConsoleResource } from './useConsoleResource'
import type { Integration, IntegrationKey, Publication, WidgetAppearance } from './types'
const props = defineProps<{
  workspace: Workspace
  accountId: string
  item: Integration
  publications: Publication[]
}>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useLocaleStore()
const base = `/api/v1/workspaces/${encodeURIComponent(props.workspace.id)}`
const keys = useConsoleResource<{ items: IntegrationKey[] }>(base + '/keys')
const manage = computed(
  () => ['owner', 'administrator'].includes(props.workspace.role) && !props.workspace.suspended,
)
const current = computed(() => props.item)
const activePublications = computed(() => props.publications.filter((item) => !item.revoked_at))
const publications = { data: computed(() => ({ items: props.publications })) }
const currentKeys = computed(
  () => keys.data.value?.items.filter((item) => item.integration_id === props.item.id) ?? [],
)
const editing = shallowRef<Integration>()
const draftOpen = shallowRef(false)
const busy = shallowRef(false)
const error = shallowRef('')
const copied = shallowRef('')
const secret = shallowRef('')
const keyName = shallowRef('')
const keyUncertain = shallowRef(false)
const formDialog = useTemplateRef<HTMLDialogElement>('formDialog')
const confirmDialog = useTemplateRef<HTMLDialogElement>('confirmDialog')
const pending = shallowRef<{ path: string; method: string; label: string } | null>(null)
const localInstallation = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname)
let disposed = false
let returnFocus: HTMLElement | null = null
const snippet = computed(
  () =>
    `<script src="${location.origin}/widget/v1.js" data-widget="${props.item.widget_id}" data-mode="${props.item.appearance?.placement ?? 'floating'}" async><` +
    '/script>',
)
const apiExample = computed(
  () =>
    `POST ${location.origin}/api/v1/streams\nAuthorization: Bearer YOUR_SERVER_API_KEY\nContent-Type: application/json\n\n${JSON.stringify({ integration_id: props.item.id }, null, 2)}`,
)
async function edit(item: Integration) {
  returnFocus = document.activeElement as HTMLElement
  editing.value = item
  error.value = ''
  draftOpen.value = true
  await nextTick()
  formDialog.value?.showModal()
}
function closeForm() {
  if (busy.value) return
  clearIntegrationEditDraft(
    `${props.accountId}:${props.workspace.id}:${props.item.id}:${props.item.revision}`,
  )
  draftOpen.value = false
  formDialog.value?.close()
  returnFocus?.focus()
}
async function save(input: {
  name: string
  publication_id: string
  allowed_origins: string[]
  capabilities: string[]
  appearance: WidgetAppearance
  daily_limit_seconds: number
}) {
  if (busy.value || !manage.value) return
  busy.value = true
  error.value = ''
  try {
    await request(base + '/integrations/' + encodeURIComponent(props.item.id), 'PATCH', input)
    if (disposed) return
    clearIntegrationEditDraft(
      `${props.accountId}:${props.workspace.id}:${props.item.id}:${props.item.revision}`,
    )
    emit('changed')
    busy.value = false
    closeForm()
  } catch (failure) {
    if (!disposed)
      error.value =
        failure instanceof Error ? failure.message : 'The integration could not be saved.'
  } finally {
    if (!disposed) busy.value = false
  }
}
function cancelConfirmation() {
  if (!busy.value) {
    confirmDialog.value?.close()
    pending.value = null
    returnFocus?.focus()
  }
}
function propose(path: string, method: string, label: string) {
  returnFocus = document.activeElement as HTMLElement
  error.value = ''
  pending.value = { path, method, label }
  confirmDialog.value?.showModal()
}
async function confirm() {
  if (!pending.value || busy.value || !manage.value) return
  busy.value = true
  error.value = ''
  try {
    await request(base + pending.value.path, pending.value.method)
    if (disposed) return
    secret.value = ''
    emit('changed')
    await keys.refresh()
    if (disposed) return
    confirmDialog.value?.close()
    pending.value = null
    returnFocus?.focus()
  } catch (failure) {
    if (!disposed)
      error.value =
        failure instanceof Error ? failure.message : 'The integration could not be updated.'
  } finally {
    if (!disposed) busy.value = false
  }
}
async function createKey() {
  if (busy.value || keyUncertain.value || !manage.value || !keyName.value.trim()) return
  busy.value = true
  error.value = ''
  secret.value = ''
  try {
    const result = await request<{ secret: string }>(base + '/keys', 'POST', {
      name: keyName.value.trim(),
      integration_id: props.item.id,
    })
    if (disposed) return
    secret.value = result.secret
    keyName.value = ''
    await keys.refresh()
  } catch (failure) {
    if (!disposed) {
      keyUncertain.value = !(failure instanceof RequestError) || failure.status >= 500
      error.value = keyUncertain.value
        ? 'The key request could not be confirmed. Refresh and revoke any unwanted key before creating another.'
        : failure instanceof Error
          ? failure.message
          : 'The credential could not be created.'
      await keys.refresh()
    }
  } finally {
    if (!disposed) busy.value = false
  }
}
async function copy(value: string, kind: string) {
  try {
    await navigator.clipboard.writeText(value)
    if (!disposed) copied.value = kind
  } catch {
    if (!disposed) error.value = 'Copy could not finish. Select and copy the text below.'
  }
}
onMounted(() => {
  if (manage.value) void keys.refresh()
})
onScopeDispose(() => {
  disposed = true
  secret.value = ''
  formDialog.value?.close()
  confirmDialog.value?.close()
})
</script>
<template>
  <section class="integration-detail">
    <header class="integration-heading">
      <div>
        <span
          class="console-pill"
          :class="{ active: current.enabled && current.published_revision > 0 }"
          >{{
            t(current.enabled && current.published_revision > 0 ? 'Published' : 'Draft or disabled')
          }}</span
        >
        <h2>{{ current.name }}</h2>
        <p class="muted">
          {{
            publications.data.value?.items.find((item) => item.id === current?.publication_id)
              ?.name || t('Character unavailable')
          }}
        </p>
      </div>
      <div v-if="manage" class="console-actions">
        <button class="console-button" @click="edit(current)">{{ t('Edit draft') }}</button
        ><button
          class="console-button primary"
          :disabled="busy || (current.enabled && current.published_revision === current.revision)"
          @click="
            propose(
              `/integrations/${encodeURIComponent(current.id)}/publish`,
              'POST',
              'Publish integration',
            )
          "
        >
          {{ t('Publish') }}
        </button>
      </div>
    </header>
    <p
      v-if="current.revision > current.published_revision && current.published_revision > 0"
      class="console-notice"
    >
      {{
        t(
          'Your saved changes are a draft. The website keeps its published version until you publish again.',
        )
      }}
    </p>
    <section class="integration-section">
      <h3>{{ t('Allowed websites') }}</h3>
      <ul class="origin-list">
        <li v-for="origin in current.allowed_origins" :key="origin">
          <AppIcon name="globe" />{{ origin }}
        </li>
      </ul>
      <p class="muted">
        {{
          t(
            'The public widget identifier is safe to share. Server API keys must stay on your backend.',
          )
        }}
      </p>
    </section>
    <section v-if="current.enabled && current.published_revision" class="integration-section">
      <div class="console-section-heading">
        <h3>{{ t('Install the widget') }}</h3>
        <button
          class="console-button quiet"
          :disabled="!current.enabled || !current.published_revision"
          @click="copy(snippet, 'widget')"
        >
          <AppIcon :name="copied === 'widget' ? 'check' : 'copy'" />{{
            t(copied === 'widget' ? 'Copied' : 'Copy snippet')
          }}
        </button>
      </div>
      <p class="muted">
        {{
          t(
            'Place this snippet on an allowed website. Visitors choose when to start voice and microphone access.',
          )
        }}
      </p>
      <code class="console-code">{{ snippet }}</code>
      <p v-if="localInstallation" class="console-notice">
        {{
          t(
            'This installation address works only on this computer. External websites need a publicly accessible HTTPS host before the widget can be tested there.',
          )
        }}
      </p>
      <p v-if="!current.enabled || !current.published_revision" class="muted">
        {{ t('Publish this integration before installing it.') }}
      </p>
      <p class="muted">
        {{ t('Playback also requires available workspace time and a connected render service.') }}
      </p>
    </section>
    <details v-if="manage" class="integration-section advanced-keys">
      <summary>{{ t('Developer options') }}</summary>
      <h3>{{ t('Server API keys') }}</h3>
      <p v-if="keys.error.value" class="console-notice error" role="alert">
        {{ t(keys.error.value) }}
      </p>
      <p class="muted">
        {{
          t(
            'Keys are restricted to this integration. Never include them in browser code or the widget snippet.',
          )
        }}
      </p>
      <form class="key-form" @submit.prevent="createKey">
        <label class="console-field"
          >{{ t('Key name')
          }}<input
            v-model="keyName"
            maxlength="80"
            required
            :disabled="busy"
            :placeholder="t('Production server')" /></label
        ><button
          class="console-button"
          :disabled="
            busy ||
            keyUncertain ||
            !current.enabled ||
            !current.published_revision ||
            !keyName.trim()
          "
        >
          {{ t('Create API key') }}
        </button>
      </form>
      <p v-if="error && !draftOpen && !pending" class="console-notice error" role="alert">
        {{ t(error) }}
      </p>
      <div v-if="secret" class="console-notice">
        <strong>{{ t('Copy this credential now. It will not be shown again.') }}</strong
        ><code class="console-code">{{ secret }}</code
        ><button class="console-button" @click="copy(secret, 'secret')">
          {{ t(copied === 'secret' ? 'Copied' : 'Copy API key') }}</button
        ><button class="console-button quiet" @click="secret = ''">{{ t('Dismiss') }}</button>
      </div>
      <ul class="key-list">
        <li v-for="key in currentKeys" :key="key.id">
          <div>
            <strong>{{ key.name }}</strong
            ><small>{{ t(key.revoked_at ? 'Revoked' : key.status) }}</small>
          </div>
          <button
            v-if="!key.revoked_at"
            class="console-button quiet danger"
            :disabled="busy"
            @click="propose(`/keys/${encodeURIComponent(key.id)}`, 'DELETE', 'Revoke API key')"
          >
            {{ t('Revoke') }}
          </button>
        </li>
      </ul>
      <details>
        <summary>{{ t('API connection details') }}</summary>
        <code class="console-code">{{ apiExample }}</code>
        <p class="muted">
          {{
            t(
              'Create the session from your server with its integration key. The response contains the authenticated stream connection.',
            )
          }}
        </p>
      </details>
    </details>
    <section v-if="manage && current.enabled" class="integration-section">
      <button
        class="console-button danger"
        :disabled="busy"
        @click="
          propose(
            `/integrations/${encodeURIComponent(current.id)}`,
            'DELETE',
            'Disable integration',
          )
        "
      >
        {{ t('Disable integration') }}
      </button>
    </section>

    <dialog
      ref="formDialog"
      class="console-dialog"
      :aria-label="t(editing ? 'Edit integration' : 'New integration')"
      @cancel.prevent="closeForm"
    >
      <template v-if="draftOpen"
        ><h2>{{ t(editing ? 'Edit integration' : 'New integration') }}</h2>
        <p>{{ t('Save a draft, review it, then publish it to your website.') }}</p>
        <IntegrationForm
          :item="editing"
          :draft-key="`${accountId}:${workspace.id}:${editing?.id}:${editing?.revision}`"
          :publications="activePublications"
          :busy="busy"
          :error="error"
          @save="save"
          @cancel="closeForm"
      /></template>
    </dialog>
    <dialog
      ref="confirmDialog"
      class="console-dialog"
      :aria-label="t(pending?.label || 'Confirm change')"
      @cancel.prevent="cancelConfirmation"
    >
      <h2>{{ t(pending?.label || 'Confirm change') }}</h2>
      <p>
        {{
          t(
            pending?.method === 'DELETE'
              ? 'Existing access through this integration or key will stop. You can create a new key later.'
              : 'This makes the saved character, appearance and website permissions available to the widget and API.',
          )
        }}
      </p>
      <p v-if="error" class="console-notice error" role="alert">{{ t(error) }}</p>
      <div class="console-actions">
        <button class="console-button primary" :disabled="busy" @click="confirm">
          {{ t(busy ? 'Saving…' : 'Confirm') }}</button
        ><button class="console-button quiet" :disabled="busy" @click="cancelConfirmation">
          {{ t('Cancel') }}
        </button>
      </div>
    </dialog>
  </section>
</template>
<style scoped>
.integration-detail {
  min-width: 0;
  padding: 28px 32px;
  background: var(--surface);
  border-radius: 12px;
}
.integration-heading {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: start;
  gap: 20px;
}
.integration-heading h2 {
  font-size: 24px;
  margin: 12px 0 4px;
  overflow-wrap: anywhere;
}
.integration-heading p {
  margin: 0;
}
.integration-section {
  padding-top: 28px;
  margin-top: 28px;
  border-top: 1px solid var(--divider);
}
.integration-section > p {
  max-width: 72ch;
}
.integration-section .console-code {
  max-width: 100%;
}
.origin-list {
  list-style: none;
  margin: 16px 0;
  padding: 0;
  display: grid;
  gap: 12px;
}
.origin-list li {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.origin-list svg {
  color: var(--muted);
  width: 16px;
  height: 16px;
}
.key-form {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  margin: 24px 0;
}
.key-form label {
  flex: 1;
  max-width: 440px;
  min-width: 0;
}
.key-list {
  list-style: none;
  padding: 0;
}
.key-list li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 0;
  border-bottom: 1px solid var(--divider);
  gap: 16px;
}
.key-list strong {
  font-size: 13px;
  overflow-wrap: anywhere;
}
.key-list small {
  display: block;
  color: var(--muted);
  font-size: 11px;
  margin-top: 5px;
}
summary {
  font-size: 13px;
  color: var(--muted);
  cursor: pointer;
  padding: 12px 0;
}
@media (max-width: 480px) {
  .integration-detail {
    padding: 24px 20px;
  }
  .integration-heading {
    gap: 16px;
  }
  .integration-heading h2 {
    font-size: 22px;
  }
  .integration-section {
    padding-top: 24px;
    margin-top: 24px;
  }
  .key-form {
    flex-direction: column;
    align-items: stretch;
  }
  .key-list li {
    flex-wrap: wrap;
  }
}
</style>
