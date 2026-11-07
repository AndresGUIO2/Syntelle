<script setup lang="ts">
import { computed, onBeforeUnmount, shallowRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import { request } from '../streaming/api'
const { t } = useLocaleStore()
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
  role.value === 'administrator'
    ? ['customers', 'plans', 'usage-costs', 'service-health', 'audit']
    : ['customers', 'plans', 'usage-costs', 'service-health'],
)
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
  <section class="staff-panel" :aria-label="t('Administration')">
    <header class="heading">
      <div>
        <p class="eyebrow">Syntelle</p>
        <h1>{{ t('Administration') }}</h1>
      </div>
      <a href="/">{{ t('Back to conversation') }}</a>
    </header>
    <p v-if="error" class="error" role="alert">{{ t(error) }}</p>
    <p v-if="busy" role="status">{{ t('Loading…') }}</p>
    <div v-if="!token" class="card">
      <h2>{{ t('Verify by email') }}</h2>
      <p>{{ t('An email code is required each time you enter this panel.') }}</p>
      <button v-if="!challengeId" type="button" :disabled="busy" @click="sendCode">
        {{ t('Send verification code') }}
      </button>
      <form v-else class="row" @submit.prevent="verify">
        <input
          v-model="code"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          required
          :aria-label="t('Email code')"
        /><button type="submit" :disabled="busy">{{ t('Verify') }}</button>
      </form>
    </div>
    <template v-else>
      <div class="row">
        <span>{{ t(role ?? '') }}</span
        ><button type="button" @click="leave">{{ t('Leave administration') }}</button>
      </div>
      <nav class="tabs" :aria-label="t('Administration sections')">
        <button
          v-for="item in sections"
          :key="item"
          type="button"
          :aria-current="section === item ? 'page' : undefined"
          @click="section = item"
        >
          {{ t(item) }}
        </button>
      </nav>
      <section v-if="section === 'customers'" class="card">
        <h2>{{ t('Customers') }}</h2>
        <label v-if="role === 'administrator'" for="suspension-reason">{{
          t('Reason for operational change')
        }}</label
        ><input
          v-if="role === 'administrator'"
          id="suspension-reason"
          v-model="reason"
          maxlength="500"
        />
        <ul class="entries">
          <li v-for="item in customers" :key="item.id">
            <span>{{ item.name }} · {{ item.suspended ? t('Suspended') : t('Active') }}</span
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
      <section v-else-if="section === 'plans'" class="card">
        <h2>{{ t('Plans') }}</h2>
        <p>{{ t('Plan prices are not configured yet.') }}</p>
      </section>
      <section v-else-if="section === 'usage-costs'" class="card">
        <h2>{{ t('Usage and costs') }}</h2>
        <p>{{ t('Paid usage and measured costs are not available yet.') }}</p>
      </section>
      <section v-else-if="section === 'service-health'" class="card">
        <h2>{{ t('Service health') }}</h2>
        <p>{{ t('API available. Other services have not been verified here.') }}</p>
      </section>
      <section v-else-if="section === 'audit' && role === 'administrator'" class="card">
        <h2>{{ t('Audit') }}</h2>
        <ul class="entries">
          <li v-for="item in audit" :key="item.created_at + item.action + item.target_id">
            {{ t(item.action) }} · {{ new Date(item.created_at).toLocaleString() }}
          </li>
        </ul>
      </section>
    </template>
  </section>
</template>

<style scoped>
.staff-panel {
  display: grid;
  gap: 1rem;
  max-width: 1050px;
  margin: 0 auto;
  color: var(--text);
}
.heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 1rem;
}
.heading h1 {
  margin: 0.2rem 0;
  font-size: clamp(1.8rem, 3vw, 2.5rem);
}
.eyebrow {
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--muted);
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
.card p {
  margin: 0;
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
  min-height: 40px;
  border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
  border-radius: 10px;
  padding: 0.5rem 0.7rem;
  background: var(--page);
  color: var(--text);
}
button {
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
  padding: 0.6rem 0;
  border-bottom: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
}
.error {
  color: #a43562;
}
@media (max-width: 600px) {
  .heading {
    flex-direction: column;
  }
}
</style>
