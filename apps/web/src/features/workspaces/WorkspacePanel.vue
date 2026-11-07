<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import { useWorkspaces, type Section, type WorkspaceRole } from './useWorkspaces'
const { t } = useLocaleStore()
const state = useWorkspaces()
const newName = shallowRef('')
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
const pendingInvite = shallowRef(new URLSearchParams(location.search).get('invite') ?? '')
const pendingTransfer = shallowRef(new URLSearchParams(location.search).get('transfer') ?? '')
onMounted(() => {
  void state.refresh()
})
function create() {
  if (newName.value.trim()) void state.create(newName.value.trim())
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
  <section class="workspace-panel" :aria-label="t('Workspaces')">
    <header class="panel-heading">
      <div>
        <p class="eyebrow">Syntelle</p>
        <h1>{{ t('Workspaces') }}</h1>
        <p>{{ t('Manage your team and shared character settings.') }}</p>
      </div>
      <a href="/">{{ t('Back to conversation') }}</a>
    </header>
    <p v-if="state.error.value" class="message error" role="alert">{{ t(state.error.value) }}</p>
    <p v-if="state.loading.value" class="message" role="status">{{ t('Loading…') }}</p>
    <div v-if="pendingInvite" class="card">
      <h2>{{ t('Workspace invitation') }}</h2>
      <button type="button" @click="acceptInvite">
        {{ t('Accept invitation') }}
      </button>
    </div>
    <div v-if="pendingTransfer" class="card">
      <h2>{{ t('Ownership invitation') }}</h2>
      <button type="button" @click="acceptOwnership">
        {{ t('Accept ownership') }}
      </button>
    </div>
    <form class="card new-workspace" @submit.prevent="create">
      <label for="workspace-name">{{ t('Create a workspace') }}</label>
      <input
        id="workspace-name"
        v-model="newName"
        maxlength="100"
        required
        :placeholder="t('Team or business name')"
      />
      <button type="submit" :disabled="state.loading.value">{{ t('Create') }}</button>
    </form>
    <template v-if="state.items.value.length">
      <div class="card selector">
        <label for="active-workspace">{{ t('Active workspace') }}</label>
        <select
          id="active-workspace"
          :value="state.selectedId.value"
          @change="state.select(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="item in state.items.value" :key="item.id" :value="item.id">
            {{ item.name }}
          </option>
        </select>
        <span v-if="state.selected.value" class="role">{{ t(state.selected.value.role) }}</span>
      </div>
      <div v-if="state.selected.value" class="details">
        <nav class="tabs" :aria-label="t('Workspace sections')">
          <button
            v-for="item in sections"
            :key="item"
            type="button"
            :aria-current="state.section.value === item ? 'page' : undefined"
            @click="state.section.value = item"
          >
            {{ t(item === 'characters' ? 'Shared settings' : item) }}
          </button>
        </nav>
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
          <h2>{{ t('Team') }}</h2>
          <ul class="entries">
            <li v-for="person in state.members.value" :key="person.account_id">
              <span>{{ person.email }} · {{ t(person.role) }}</span
              ><select
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
            </li>
          </ul>
          <form class="row" @submit.prevent="invite">
            <input
              v-model="email"
              type="email"
              required
              :aria-label="t('Invite by email')"
              :placeholder="t('Invite by email')"
            /><select v-model="inviteRole" :aria-label="t('Role')">
              <option value="editor">{{ t('editor') }}</option>
              <option v-if="state.selected.value.role === 'owner'" value="administrator">
                {{ t('administrator') }}
              </option>
              <option v-if="state.selected.value.role === 'owner'" value="billing">
                {{ t('billing') }}
              </option></select
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
  </section>
</template>

<style scoped>
.workspace-panel {
  display: grid;
  gap: 1rem;
  max-width: 1050px;
  margin: 0 auto;
  color: var(--text);
}
.panel-heading {
  display: flex;
  justify-content: space-between;
  align-items: start;
  gap: 1rem;
}
.panel-heading h1 {
  font-size: clamp(1.8rem, 3vw, 2.5rem);
  margin: 0.2rem 0;
}
.eyebrow {
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.12em;
  font-size: 0.72rem;
}
.card {
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  border-radius: 18px;
  padding: 1.25rem;
  display: grid;
  gap: 0.8rem;
}
.card h2,
.card h3 {
  margin: 0;
}
.card p {
  margin: 0;
}
.new-workspace,
.selector,
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.7rem;
}
input,
select {
  min-height: 40px;
  padding: 0.55rem 0.7rem;
  border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  border-radius: 10px;
  background: var(--page);
  color: var(--text);
}
button {
  min-height: 38px;
  padding: 0.5rem 0.8rem;
  border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  border-radius: 10px;
  background: var(--page);
  color: var(--text);
  cursor: pointer;
}
button:hover,
button[aria-current='page'] {
  border-color: var(--accent);
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-bottom: 1rem;
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
  color: #a43562;
}
.transfer {
  display: grid;
  gap: 0.6rem;
  border-top: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  padding-top: 1rem;
}
@media (max-width: 600px) {
  .panel-heading {
    flex-direction: column;
  }
  .new-workspace input,
  .row input {
    flex: 1 1 180px;
  }
}
</style>
