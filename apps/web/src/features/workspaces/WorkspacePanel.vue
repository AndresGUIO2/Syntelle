<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onScopeDispose,
  shallowRef,
  useTemplateRef,
  watch,
} from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import SectionNavigation from '../navigation/SectionNavigation.vue'
import WorkspaceCreation from './WorkspaceCreation.vue'
import { useWorkspaces, type Section, type WorkspaceRole } from './useWorkspaces'
const { t } = useLocaleStore()
const props = withDefaults(
  defineProps<{
    conversationHref?: string
    workspaceId?: string
    embedded?: boolean
    initialSection?: Section
  }>(),
  { conversationHref: '/', workspaceId: undefined, initialSection: undefined },
)
defineEmits<{ navigate: [event: MouseEvent] }>()
const state = useWorkspaces()
const initialized = shallowRef(false)
const creationOpen = shallowRef(false)
const creating = shallowRef(false)
const creationError = shallowRef('')
const creationDialog = useTemplateRef<HTMLDialogElement>('creationDialog')
const createTrigger = useTemplateRef<HTMLButtonElement>('createTrigger')
const workspacePicker = useTemplateRef<HTMLSelectElement>('workspacePicker')
const email = shallowRef('')
const inviteRole = shallowRef<WorkspaceRole>('editor')
const keyName = shallowRef('')
const copyId = shallowRef('')
const shareableSettings = computed(() =>
  state.personal.value.filter((item) => item.settings.character === 'syntelle'),
)
const transferTo = shallowRef('')
const ownershipChallenge = shallowRef('')
const ownershipCode = shallowRef('')
const sections = computed<Section[]>(() => {
  const base: Section[] = ['overview', 'usage', 'billing']
  if (state.operate.value) base.splice(1, 0, 'characters')
  if (state.manage.value) base.splice(2, 0, 'integrations', 'team', 'activity')
  return base
})
const sectionLabels: Record<Section, string> = {
  overview: 'Overview',
  characters: 'Shared settings',
  integrations: 'Integrations',
  team: 'Team',
  activity: 'Activity',
  usage: 'Usage',
  billing: 'Billing',
}
const sectionItems = computed(() =>
  sections.value.map((value) => ({ value, label: t(sectionLabels[value]) })),
)
function selectSection(value: string) {
  if (sections.value.includes(value as Section)) state.section.value = value as Section
}
const pendingInvite = shallowRef(new URLSearchParams(location.search).get('invite') ?? '')
const pendingTransfer = shallowRef(new URLSearchParams(location.search).get('transfer') ?? '')
onMounted(async () => {
  await state.refresh()
  if (props.workspaceId) await state.select(props.workspaceId)
  if (props.initialSection) state.section.value = props.initialSection
  initialized.value = true
})
watch(
  [creationOpen, creationDialog],
  ([open, dialog]) => {
    if (open && dialog && !dialog.open) dialog.showModal()
    else if (!open && dialog?.open) dialog.close()
  },
  { flush: 'post' },
)
onScopeDispose(() => creationDialog.value?.close())
function openCreation() {
  creationError.value = ''
  creationOpen.value = true
}
function closeCreation() {
  if (creating.value) return
  creationOpen.value = false
  void nextTick(() => createTrigger.value?.focus())
}
async function create(name: string) {
  if (creating.value) return
  creating.value = true
  creationError.value = ''
  await state.create(name)
  creating.value = false
  if (state.error.value) creationError.value = state.error.value
  else {
    creationOpen.value = false
    await nextTick()
    workspacePicker.value?.focus()
  }
}
function invite() {
  if (email.value.trim()) void state.invite(email.value.trim(), inviteRole.value)
}
function copy() {
  if (copyId.value) void state.copyConfiguration(copyId.value)
}
async function requestOwnershipCode() {
  ownershipChallenge.value = await state.startOwnershipEmail()
}
function transfer() {
  if (ownershipChallenge.value && ownershipCode.value && transferTo.value)
    void state.transferOwnership(transferTo.value, ownershipChallenge.value, ownershipCode.value)
}
function acceptInvite() {
  const token = pendingInvite.value
  void state.acceptInvitation(token).then((accepted) => {
    if (accepted) pendingInvite.value = ''
  })
}
function acceptOwnership() {
  const token = pendingTransfer.value
  void state.acceptTransfer(token).then((accepted) => {
    if (accepted) pendingTransfer.value = ''
  })
}
function createCredential() {
  if (keyName.value.trim()) void state.createKey(keyName.value.trim())
}
</script>

<template>
  <section
    class="workspace-panel"
    :class="{ 'empty-state': initialized && !state.items.value.length && !embedded }"
    :aria-label="t('Workspaces')"
  >
    <header v-if="!embedded" class="panel-heading">
      <div>
        <h1>{{ t('Workspaces') }}</h1>
        <p>{{ t('Shared settings and team access, in one place.') }}</p>
      </div>
    </header>
    <p v-if="state.error.value && !creationError" class="message error" role="alert">
      {{ t(state.error.value) }}
    </p>
    <p v-if="!initialized" class="message" role="status">{{ t('Loading…') }}</p>
    <div v-if="pendingInvite" class="invitation">
      <h2>{{ t('Workspace invitation') }}</h2>
      <button type="button" @click="acceptInvite">
        {{ t('Accept invitation') }}
      </button>
    </div>
    <div v-if="pendingTransfer" class="invitation">
      <h2>{{ t('Ownership invitation') }}</h2>
      <button type="button" @click="acceptOwnership">
        {{ t('Accept ownership') }}
      </button>
    </div>
    <div v-if="initialized && !state.items.value.length" class="empty-workspace">
      <WorkspaceCreation :busy="creating" :error="creationError" @create="create" />
    </div>
    <template v-if="state.items.value.length">
      <div v-if="!embedded" class="workspace-toolbar">
        <div class="selector">
          <label for="active-workspace">{{ t('Active workspace') }}</label>
          <div class="workspace-choice">
            <select
              id="active-workspace"
              ref="workspacePicker"
              :value="state.selectedId.value"
              @change="state.select(($event.target as HTMLSelectElement).value)"
            >
              <option v-for="item in state.items.value" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select>
            <span v-if="state.selected.value" class="role">{{ t(state.selected.value.role) }}</span>
          </div>
        </div>
        <button
          ref="createTrigger"
          class="new-workspace-button"
          type="button"
          @click="openCreation"
        >
          <AppIcon name="plus" />{{ t('New workspace') }}
        </button>
      </div>
      <div v-if="state.selected.value" class="details" :class="{ embedded }">
        <SectionNavigation
          v-if="!embedded"
          :label="t('Workspace sections')"
          :items="sectionItems"
          :selected="state.section.value"
          @select="selectSection"
        />
        <section v-if="state.section.value === 'overview'" class="card">
          <h2>{{ state.selected.value.name }}</h2>
          <p v-if="state.selected.value.suspended" class="message error">
            {{ t('This workspace is suspended.') }}
          </p>
          <p>{{ t('Commercial access is not available yet.') }}</p>
          <p>{{ t('Your current personal conversations remain private.') }}</p>
        </section>
        <section v-else-if="state.section.value === 'usage'" class="card">
          <h2>{{ t('Usage') }}</h2>
          <p>{{ t('Paid usage is not active yet.') }}</p>
        </section>
        <section v-else-if="state.section.value === 'billing'" class="card">
          <h2>{{ t('Billing') }}</h2>
          <p>{{ t('Subscriptions and invoices will appear when payments are available.') }}</p>
        </section>
        <section
          v-else-if="state.section.value === 'characters' && state.operate.value"
          class="card"
        >
          <h2>{{ t('Shared settings') }}</h2>
          <p>{{ t('Copy one of your saved character settings into this workspace.') }}</p>
          <form class="row" @submit.prevent="copy">
            <select v-model="copyId" :aria-label="t('Personal saved setting')">
              <option value="">{{ t('Choose a saved setting') }}</option>
              <option v-for="item in shareableSettings" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select>
            <button type="submit" :disabled="!copyId">{{ t('Copy') }}</button>
          </form>
          <ul class="entries">
            <li v-for="item in state.configurations.value" :key="item.id">
              <span>{{ item.name }}</span
              ><button type="button" @click="state.removeConfiguration(item.id)">
                {{ t('Remove') }}
              </button>
            </li>
          </ul>
          <p v-if="!state.configurations.value.length">{{ t('No shared settings yet.') }}</p>
        </section>
        <section
          v-else-if="state.section.value === 'integrations' && state.manage.value"
          class="card"
        >
          <h2>{{ t('Integrations') }}</h2>
          <p>{{ t('Workspace credentials remain inactive until paid access is available.') }}</p>
          <form class="row" @submit.prevent="createCredential">
            <input
              v-model="keyName"
              maxlength="80"
              required
              :aria-label="t('Credential name')"
              :placeholder="t('Credential name')"
            /><button type="submit">{{ t('Create credential') }}</button>
          </form>
          <p v-if="state.secret.value" class="secret">
            {{ t('Copy this credential now. It will not be shown again.') }}
            <code>{{ state.secret.value }}</code>
          </p>
          <ul class="entries">
            <li v-for="key in state.keys.value" :key="key.id">
              <span>{{ key.name }} · {{ t(key.status) }}</span
              ><button v-if="!key.revoked_at" type="button" @click="state.revokeKey(key.id)">
                {{ t('Revoke') }}
              </button>
            </li>
          </ul>
        </section>
        <section v-else-if="state.section.value === 'team' && state.manage.value" class="card">
          <h2 v-if="!embedded">{{ t('Team') }}</h2>
          <ul class="entries members">
            <li v-for="person in state.members.value" :key="person.account_id" class="member-item">
              <div class="member-identity">
                <strong>{{ person.name || person.email }}</strong
                ><small v-if="person.name">{{ person.email }}</small>
              </div>
              <div class="member-actions">
                <select
                  v-if="state.selected.value.role === 'owner' && person.role !== 'owner'"
                  :value="person.role"
                  :aria-label="t('Role for {email}', { email: person.email })"
                  @change="
                    state.memberChange(
                      person.account_id,
                      ($event.target as HTMLSelectElement).value as WorkspaceRole,
                    )
                  "
                >
                  <option value="administrator">{{ t('administrator') }}</option>
                  <option value="editor">{{ t('editor') }}</option>
                  <option value="billing">{{ t('billing') }}</option></select
                ><span v-else class="role">{{ t(person.role) }}</span
                ><button
                  v-if="
                    person.role === 'editor' ||
                    (state.selected.value.role === 'owner' && person.role !== 'owner')
                  "
                  type="button"
                  @click="state.memberChange(person.account_id)"
                >
                  {{ t('Remove') }}
                </button>
              </div>
            </li>
          </ul>
          <form class="invite-form" @submit.prevent="invite">
            <label class="invite-email"
              >{{ t('Invite by email')
              }}<input
                v-model="email"
                type="email"
                required
                :aria-label="t('Invite by email')"
                :placeholder="t('Invite by email')" /></label
            ><label
              >{{ t('Role')
              }}<select v-model="inviteRole" :aria-label="t('Role')">
                <option value="editor">{{ t('editor') }}</option>
                <option v-if="state.selected.value.role === 'owner'" value="administrator">
                  {{ t('administrator') }}
                </option>
                <option v-if="state.selected.value.role === 'owner'" value="billing">
                  {{ t('billing') }}
                </option>
              </select></label
            ><button type="submit">{{ t('Create invitation') }}</button>
          </form>
          <p v-if="state.invitation.value" class="secret">
            {{ t('Share this link with the invited person.') }}
            <code>{{ state.invitation.value }}</code>
          </p>
          <div v-if="state.selected.value.role === 'owner'" class="transfer">
            <h3>{{ t('Transfer ownership') }}</h3>
            <p>{{ t('Choose an existing member. Email verification is required.') }}</p>
            <select v-model="transferTo" :aria-label="t('New owner')">
              <option value="">{{ t('Choose a member') }}</option>
              <option
                v-for="person in state.members.value.filter((person) => person.role !== 'owner')"
                :key="person.account_id"
                :value="person.account_id"
              >
                {{ person.email }}
              </option>
            </select>
            <button type="button" :disabled="!transferTo" @click="requestOwnershipCode">
              {{ t('Send verification code') }}
            </button>
            <div v-if="ownershipChallenge" class="row">
              <input
                v-model="ownershipCode"
                inputmode="numeric"
                maxlength="6"
                :aria-label="t('Email code')"
              /><button type="button" @click="transfer">{{ t('Prepare transfer') }}</button>
            </div>
            <p v-if="state.transfer.value" class="secret">
              {{ t('Share this link with the new owner.') }} <code>{{ state.transfer.value }}</code>
            </p>
          </div>
        </section>
        <section v-else-if="state.section.value === 'activity' && state.manage.value" class="card">
          <h2>{{ t('Activity') }}</h2>
          <ul class="entries">
            <li
              v-for="item in state.activity.value"
              :key="item.created_at + item.action + item.target_id"
            >
              <span
                >{{ t(item.action) }} · {{ item.actor_email || 'Syntelle' }} ·
                {{ new Date(item.created_at).toLocaleString()
                }}<small v-if="item.reason">{{ item.reason }}</small></span
              >
            </li>
          </ul>
        </section>
      </div>
    </template>
    <dialog
      ref="creationDialog"
      class="creation-dialog"
      :aria-label="t('Create workspace')"
      @cancel.prevent="closeCreation"
      @close="creationOpen = false"
      @click="$event.target === creationDialog && closeCreation()"
    >
      <WorkspaceCreation
        v-if="creationOpen"
        :busy="creating"
        :error="creationError"
        dismissible
        @create="create"
        @cancel="closeCreation"
      />
    </dialog>
  </section>
</template>

<style scoped>
.workspace-panel {
  display: grid;
  gap: 28px;
  width: 100%;
  max-width: 1184px;
  min-width: 0;
  margin: 0;
  color: var(--text);
}
.workspace-panel.empty-state {
  max-width: 520px;
}
.panel-heading {
  display: flex;
  justify-content: space-between;
  align-items: start;
  gap: 1rem;
}
.panel-heading h1 {
  font-size: 32px;
  font-weight: 650;
  letter-spacing: -0.8px;
  line-height: 1.25;
  margin: 0;
}
.panel-heading > div > p:last-child {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
}
.empty-workspace {
  width: min(100%, 520px);
  margin: 8px 0 32px;
  background: var(--surface);
  border: 1px solid transparent;
  border-radius: 18px;
  box-shadow:
    0 0 0 1px var(--divider),
    0 8px 28px rgb(0 0 0 / 0.04);
  box-sizing: border-box;
}
.creation-dialog {
  width: min(520px, calc(100% - 32px));
  max-height: calc(100dvh - 40px);
  padding: 0;
  border: 1px solid var(--divider);
  border-radius: 18px;
  background: var(--surface);
  color: var(--text);
  box-shadow: 0 24px 80px rgb(0 0 0 / 0.25);
  overflow: auto;
  overscroll-behavior: contain;
}
.creation-dialog::backdrop {
  background: rgb(8 6 12 / 0.55);
}
.invitation {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  border-radius: 12px;
  background: var(--subtle);
}
.invitation h2 {
  margin: 0;
  font-size: 16px;
}
.workspace-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding-bottom: 28px;
  border-bottom: 1px solid var(--divider);
}
.workspace-choice {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.new-workspace-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  white-space: nowrap;
}
.new-workspace-button :deep(svg) {
  width: 18px;
  height: 18px;
}
.selector {
  display: grid;
  gap: 9px;
  min-width: 0;
}
.selector label {
  font-size: 12px;
  color: var(--muted);
}
.selector select {
  width: 320px;
  max-width: 100%;
  font-weight: 600;
  background: var(--surface);
}
.role {
  color: var(--muted);
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 5px;
  background: var(--subtle);
}
.message {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
}
.message.error {
  color: var(--error);
}
@media (min-width: 1024px) {
  .card {
    min-height: 190px;
    border-left: 1px solid var(--divider);
  }
}
.details {
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr);
  align-items: start;
  gap: 32px;
}
.card {
  padding: 8px 0 24px 32px;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-content: start;
  gap: 16px;
  font-size: 14px;
  min-width: 0;
}
.card h2,
.card h3 {
  margin: 0;
  font-weight: 600;
}
.card h2 {
  font-size: 21px;
  letter-spacing: -0.4px;
}
.card h3 {
  font-size: 16px;
}
.card p {
  margin: 0;
  color: var(--muted);
  line-height: 1.6;
}
.row {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.7rem;
}
.row > input,
.row > select {
  flex: 1 1 220px;
  min-width: 0;
  max-width: min(520px, 100%);
}
input,
select {
  min-height: 44px;
  max-width: 100%;
  min-width: 0;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  background: var(--page);
  color: var(--text);
  font: inherit;
  font-size: 14px;
  box-sizing: border-box;
}
button {
  min-height: 44px;
  padding: 0.5rem 0.8rem;
  border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  border-radius: 10px;
  background: var(--page);
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 550;
  cursor: pointer;
}
@media (hover: hover) {
  button:hover {
    border-color: var(--accent);
  }
}
button:focus-visible,
input:focus-visible,
select:focus-visible,
a:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.entries {
  padding: 0;
  margin: 0;
  list-style: none;
  display: grid;
  gap: 0.5rem;
}
.entries li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 0;
  border-bottom: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  flex-wrap: wrap;
}
.entries li > span {
  flex: 1 1 180px;
  overflow-wrap: anywhere;
}
.entries .member-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 0;
}
.member-identity {
  display: grid;
  gap: 4px;
  flex: 1 1 180px;
  min-width: 0;
}
.member-identity strong {
  font-size: 14px;
  font-weight: 550;
  overflow-wrap: anywhere;
}
.member-identity small {
  color: var(--muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.member-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}
.member-actions button {
  border-color: transparent;
  color: var(--muted);
  background: transparent;
}
.invite-form {
  display: flex;
  align-items: end;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 8px;
}
.invite-form label {
  display: grid;
  gap: 8px;
  font-size: 12px;
  color: var(--muted);
  min-width: 0;
}
.invite-form .invite-email {
  flex: 1 1 260px;
}
.invite-form input {
  width: 100%;
}
.secret {
  overflow-wrap: anywhere;
}
.secret code {
  display: block;
  margin-top: 0.4rem;
  font-size: 0.8rem;
}
.message.error {
  color: var(--error);
}
.transfer {
  display: grid;
  gap: 0.6rem;
  border-top: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  padding-top: 24px;
  margin-top: 16px;
  width: 100%;
  max-width: 640px;
  justify-items: start;
}
.transfer > select,
.transfer > .row {
  width: 100%;
}
.transfer .row input {
  max-width: 180px;
}
@media (max-width: 600px) {
  .panel-heading {
    flex-direction: column;
  }
  .row input {
    flex: 1 1 180px;
  }
  .workspace-panel {
    gap: 24px;
  }
  .panel-heading h1 {
    font-size: 28px;
  }
  .empty-workspace {
    margin: 12px 0 24px;
  }
  .workspace-toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 18px;
    padding-bottom: 22px;
  }
  .workspace-choice select {
    min-width: 0;
    flex: 1;
    width: auto;
  }
  .new-workspace-button {
    align-self: flex-start;
  }
  .invitation {
    align-items: start;
    flex-direction: column;
  }
  .invite-form {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    width: 100%;
  }
  .invite-form button {
    justify-self: start;
  }
}
@media (max-width: 1023px) {
  .details {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
  .card {
    padding: 8px 0 24px;
  }
}
.details.embedded {
  grid-template-columns: minmax(0, 1fr);
  gap: 0;
}
.details.embedded .card {
  padding: 0;
  border: 0;
}
</style>
