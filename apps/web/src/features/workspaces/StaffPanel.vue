<script setup lang="ts">
import { computed, onBeforeUnmount, shallowRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import { request } from '../streaming/api'
import SectionNavigation from '../navigation/SectionNavigation.vue'
const { t } = useLocaleStore()
withDefaults(defineProps<{ conversationHref?: string }>(), { conversationHref: '/' })
defineEmits<{ navigate: [event: MouseEvent] }>()
type StaffRole = 'administrator' | 'support'
type Customer = { id: string; name: string; suspended: boolean; created_at: string }
type Audit = {
  workspace_id: string | null
  actor_account_id: string | null
  action: string
  target_id: string
  reason: string
  created_at: string
}
const challengeId = shallowRef('')
const code = shallowRef('')
const token = shallowRef('')
const role = shallowRef<StaffRole | null>(null)
const section = shallowRef('customers')
const customers = shallowRef<Customer[]>([])
const audit = shallowRef<Audit[]>([])
const reason = shallowRef('')
const error = shallowRef('')
const busy = shallowRef(false)
const sections = computed(() =>
  role.value === 'administrator' ? ['customers', 'audit'] : ['customers'],
)
const sectionLabels: Record<string, string> = {
  customers: 'Customers',
  plans: 'Plans',
  'usage-costs': 'Usage and costs',
  'service-health': 'Service health',
  audit: 'Audit',
}
const sectionItems = computed(() =>
  sections.value.map((value) => ({ value, label: t(sectionLabels[value] ?? value) })),
)
function selectSection(value: string) {
  if (sections.value.includes(value)) section.value = value
}
const adminHeaders = computed(() => ({ 'X-Syntelle-Staff-Access': token.value }))
async function run(work: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await work()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'The request failed.'
  } finally {
    busy.value = false
  }
}
function sendCode() {
  void run(async () => {
    const result = await request<{ challenge_id: string }>('/api/v1/admin/challenge/start', 'POST')
    challengeId.value = result.challenge_id
  })
}
function verify() {
  void run(async () => {
    const result = await request<{ access_token: string }>(
      '/api/v1/admin/challenge/verify',
      'POST',
      { challenge_id: challengeId.value, code: code.value },
    )
    token.value = result.access_token
    code.value = ''
    const access = await request<{ role: StaffRole }>(
      '/api/v1/admin',
      'GET',
      undefined,
      adminHeaders.value,
    )
    role.value = access.role
    await load()
  })
}
async function load() {
  if (!token.value) return
  await run(async () => {
    const response = await request<{ items: Customer[] }>(
      '/api/v1/admin/customers',
      'GET',
      undefined,
      adminHeaders.value,
    )
    customers.value = response.items
    if (role.value === 'administrator') {
      const entries = await request<{ items: Audit[] }>(
        '/api/v1/admin/audit',
        'GET',
        undefined,
        adminHeaders.value,
      )
      audit.value = entries.items
    }
  })
}
function changeSuspension(customer: Customer) {
  if (!reason.value.trim()) return
  void run(async () => {
    await request(
      `/api/v1/admin/workspaces/${encodeURIComponent(customer.id)}/${customer.suspended ? 'resume' : 'suspend'}`,
      'POST',
      { reason: reason.value.trim() },
      adminHeaders.value,
    )
    reason.value = ''
    await load()
  })
}
function leave() {
  const current = token.value
  token.value = ''
  role.value = null
  challengeId.value = ''
  if (current)
    void request('/api/v1/admin/exit', 'POST', {}, { 'X-Syntelle-Staff-Access': current }).catch(
      () => undefined,
    )
}
onBeforeUnmount(leave)
</script>

<template>
  <section
    class="staff-panel"
    :class="{ 'verification-state': !token }"
    :aria-label="t('Administration')"
  >
    <header class="heading">
      <div>
        <p class="eyebrow">{{ t('Staff console') }}</p>
        <h1>{{ t('Administration') }}</h1>
      </div>
    </header>
    <p v-if="error" class="error" role="alert">{{ t(error) }}</p>
    <p v-if="busy" role="status">{{ t('Loading…') }}</p>
    <div v-if="!token" class="card verification-card">
      <h2>{{ t('Verify by email') }}</h2>
      <p>{{ t('An email code is required each time you enter this panel.') }}</p>
      <button
        v-if="!challengeId"
        class="console-button primary"
        type="button"
        :disabled="busy"
        @click="sendCode"
      >
        {{ t('Send verification code') }}
      </button>
      <form v-else class="verification-form" @submit.prevent="verify">
        <label for="staff-email-code">{{ t('Email code') }}</label>
        <div class="row">
          <input
            id="staff-email-code"
            v-model="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            required
            :aria-label="t('Email code')"
          /><button class="console-button primary" type="submit" :disabled="busy">
            {{ t('Verify') }}
          </button>
        </div>
      </form>
    </div>
    <template v-else>
      <div class="session-toolbar">
        <span class="staff-role">{{ t(role ?? '') }}</span
        ><button type="button" @click="leave">{{ t('Leave administration') }}</button>
      </div>
      <div class="details">
        <SectionNavigation
          :label="t('Administration sections')"
          :items="sectionItems"
          :selected="section"
          @select="selectSection"
        />
        <section v-if="section === 'customers'" class="card">
          <h2>{{ t('Customers') }}</h2>
          <div v-if="role === 'administrator'" class="reason-field">
            <label for="suspension-reason">{{ t('Reason for operational change') }}</label
            ><input id="suspension-reason" v-model="reason" maxlength="500" />
          </div>
          <ul class="entries">
            <li v-for="item in customers" :key="item.id">
              <span class="customer-identity"
                ><strong>{{ item.name }}</strong
                ><small>{{ item.suspended ? t('Suspended') : t('Active') }}</small></span
              ><button
                v-if="role === 'administrator'"
                type="button"
                :disabled="!reason.trim()"
                @click="changeSuspension(item)"
              >
                {{ item.suspended ? t('Resume') : t('Suspend') }}
              </button>
            </li>
          </ul>
          <p v-if="!customers.length">{{ t('No customer workspaces yet.') }}</p>
        </section>
        <section v-else-if="section === 'audit' && role === 'administrator'" class="card">
          <h2>{{ t('Audit') }}</h2>
          <ul class="entries">
            <li v-for="item in audit" :key="item.created_at + item.action + item.target_id">
              {{ t(item.action) }} · {{ new Date(item.created_at).toLocaleString() }}
            </li>
          </ul>
        </section>
      </div>
    </template>
  </section>
</template>

<style scoped>
.staff-panel {
  display: grid;
  gap: 24px;
  width: 100%;
  max-width: 1184px;
  min-width: 0;
  margin: 0;
  color: var(--text);
}
.staff-panel.verification-state {
  max-width: 640px;
}
.heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 1rem;
}
.heading h1 {
  margin: 0;
  font-size: 32px;
  line-height: 1.2;
  letter-spacing: -0.8px;
  font-weight: 650;
}
.heading > a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--muted);
  font-size: 13px;
  text-underline-offset: 4px;
}
.details {
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr);
  align-items: start;
  gap: 28px;
}
.eyebrow {
  margin: 0 0 12px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--muted);
  font-size: 0.72rem;
}
.card {
  background: var(--surface);
  border: 1px solid transparent;
  border-radius: 12px;
  padding: 24px;
  display: grid;
  gap: 0.8rem;
  min-width: 0;
}
.verification-card {
  width: 100%;
  padding: 28px;
  gap: 16px;
}
.verification-card > button {
  justify-self: start;
}
.verification-form,
.reason-field {
  display: grid;
  gap: 8px;
}
.verification-form label,
.reason-field label {
  font-size: 13px;
  font-weight: 550;
}
.verification-form input {
  width: 180px;
  min-width: 0;
  font-size: 20px;
  letter-spacing: 0.22em;
}
.reason-field {
  width: 100%;
  max-width: 640px;
  margin: 4px 0 12px;
}
.reason-field input {
  width: 100%;
  min-width: 0;
}
.session-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--divider);
}
.staff-role {
  padding: 5px 9px;
  font-size: 12px;
  color: var(--muted);
  background: var(--subtle);
  border-radius: 5px;
}
.card h2,
.card p {
  margin: 0;
}
.card p {
  color: var(--muted);
  line-height: 1.6;
}
.row,
.tabs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
}
button,
input {
  min-height: 44px;
  max-width: 100%;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  padding: 0.5rem 0.7rem;
  background: var(--page);
  color: var(--text);
  font: inherit;
  font-size: 14px;
}
button {
  cursor: pointer;
  flex-shrink: 0;
}
@media (hover: hover) {
  button:hover {
    border-color: var(--accent);
  }
}
button:focus-visible,
input:focus-visible,
a:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.entries {
  list-style: none;
  padding: 0;
  margin: 0;
}
.entries li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 16px 0;
  border-bottom: 1px solid var(--divider);
  overflow-wrap: anywhere;
}
.entries li:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}
.customer-identity {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.customer-identity strong {
  font-size: 14px;
  font-weight: 550;
  line-height: 1.5;
}
.customer-identity small {
  font-size: 12px;
  color: var(--muted);
}
.error {
  color: var(--error);
}
@media (max-width: 600px) {
  .heading {
    flex-direction: column;
  }
  .heading h1 {
    font-size: 28px;
  }
  .card {
    padding: 20px;
  }
  .session-toolbar {
    flex-wrap: wrap;
  }
  .entries li {
    align-items: flex-start;
    gap: 12px;
  }
}
@media (max-width: 1023px) {
  .details {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
}
</style>
