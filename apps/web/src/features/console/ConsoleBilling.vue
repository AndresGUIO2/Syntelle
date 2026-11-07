<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef, useTemplateRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import { request } from '../streaming/api'
import type { Workspace } from '../workspaces/useWorkspaces'
import ConsoleHeading from './ConsoleHeading.vue'
import { useConsoleResource } from './useConsoleResource'
import { usePaddleCheckout } from './usePaddleCheckout'
import type { Billing } from './types'
const props = defineProps<{ workspace: Workspace }>()
const { t, locale } = useLocaleStore()
const base = `/api/v1/workspaces/${encodeURIComponent(props.workspace.id)}/billing`
const state = useConsoleResource<Billing>(base)
const busy = shallowRef(false)
const notice = shallowRef('')
const actionError = shallowRef('')
const checkout = shallowRef('')
const checkoutLoading = shallowRef(false)
const payment = usePaddleCheckout((event) => {
  if (event === 'error') actionError.value = 'The secure checkout could not complete this action.'
  else {
    notice.value =
      event === 'completed'
        ? 'Checkout finished. Waiting for server confirmation.'
        : 'Checkout closed. No access changes are applied until payment is confirmed.'
    void state.refresh()
  }
})
async function openCheckout() {
  if (!checkout.value || checkoutLoading.value || !state.data.value?.client_token) return
  checkoutLoading.value = true
  actionError.value = ''
  try {
    await payment.open(checkout.value, state.data.value.client_token, useLocaleStore().locale)
  } catch (failure) {
    if (!disposed)
      actionError.value =
        failure instanceof Error ? failure.message : 'The secure checkout is unavailable.'
  } finally {
    if (!disposed) checkoutLoading.value = false
  }
}
const pending = shallowRef<{ path: string; body: Record<string, string>; label: string } | null>(
  null,
)
const confirmation = useTemplateRef<HTMLDialogElement>('confirmation')
const entitled = computed(() => ['owner', 'billing'].includes(props.workspace.role))
let disposed = false
let lastAction: { fingerprint: string; key: string } | undefined
const money = (amount: number, currency: string) =>
  new Intl.NumberFormat(locale, { style: 'currency', currency }).format(amount / 100)
function cancelConfirmation() {
  if (busy.value) return
  confirmation.value?.close()
  pending.value = null
}
function propose(path: string, body: Record<string, string>, label: string) {
  pending.value = { path, body, label }
  confirmation.value?.showModal()
}
async function confirm() {
  if (!pending.value || busy.value || !state.data.value?.can_manage || !state.data.value.configured)
    return
  const action = pending.value
  const fingerprint = JSON.stringify(action)
  if (lastAction?.fingerprint !== fingerprint)
    lastAction = { fingerprint, key: crypto.randomUUID() }
  busy.value = true
  actionError.value = ''
  checkout.value = ''
  notice.value = ''
  try {
    const result = await request<{
      transaction_id?: string
      environment?: string
      status?: string
      effective_at?: string
    }>(base + action.path, 'POST', action.body, { 'Idempotency-Key': lastAction.key })
    if (disposed) return
    if (result.transaction_id) {
      if (result.environment !== 'sandbox')
        throw new Error('The test payment configuration is not valid.')
      checkout.value = result.transaction_id
      notice.value = 'Your secure payment page is ready. Continue to complete this action.'
    } else
      notice.value =
        result.status === 'scheduled'
          ? 'The change is scheduled for the end of the current billing period.'
          : 'The change is awaiting payment confirmation.'
    lastAction = undefined
    confirmation.value?.close()
    pending.value = null
    await state.refresh()
    if (checkout.value && !disposed) await openCheckout()
  } catch (failure) {
    if (!disposed)
      actionError.value = failure instanceof Error ? failure.message : 'The billing request failed.'
  } finally {
    if (!disposed) busy.value = false
  }
}
onMounted(() => {
  if (entitled.value) void state.refresh()
})
onScopeDispose(() => {
  disposed = true
  confirmation.value?.close()
})
</script>
<template>
  <section class="console-page console-page--operational billing-page">
    <ConsoleHeading
      title="Billing"
      description="Manage your workspace subscription and prepaid conversation time."
      :workspace="workspace.name"
      ><span class="console-pill">{{ t('Test environment') }}</span></ConsoleHeading
    >
    <p v-if="!entitled" class="console-notice">
      {{ t('Only the workspace owner and billing members can manage billing.') }}
    </p>
    <p v-if="state.error.value" class="console-notice error" role="alert">
      {{ t(state.error.value) }}
    </p>
    <p v-if="state.loading.value && !state.data.value" role="status">{{ t('Loading…') }}</p>
    <template v-if="state.data.value"
      ><p class="console-notice">
        {{
          t(
            'Paddle sandbox uses test payments. These plans and amounts are for integration testing, not approved launch pricing.',
          )
        }}
      </p>
      <p v-if="!state.data.value.configured" class="console-notice" role="status">
        <strong>{{ t('Payments are not connected yet.') }}</strong
        ><br />{{
          t(
            'A workspace owner can review the test catalog here. Checkout becomes available when the test payment service is configured.',
          )
        }}
      </p>
      <p v-if="state.data.value.review_required" class="console-notice error">
        {{
          t('Billing needs review. New paid sessions are paused until the balance is reconciled.')
        }}
      </p>
      <p v-if="notice" class="console-notice" role="status">
        {{ t(notice) }}
        <button
          v-if="checkout"
          class="console-button primary"
          :disabled="checkoutLoading"
          @click="openCheckout"
        >
          {{ t(checkoutLoading ? 'Opening secure checkout…' : 'Continue to secure checkout') }}
        </button>
      </p>
      <section v-if="state.data.value.subscription" class="console-surface subscription-summary">
        <div>
          <span class="console-pill">{{ t(state.data.value.subscription.status) }}</span>
          <h2>
            {{
              state.data.value.catalog.plans.find(
                (plan) => plan.id === state.data.value?.subscription?.plan,
              )?.name || state.data.value.subscription.plan
            }}
          </h2>
          <p class="muted">
            {{ t('Current period ends') }}
            {{
              new Date(state.data.value.subscription.current_period_end).toLocaleDateString(locale)
            }}
          </p>
          <p v-if="state.data.value.subscription.cancel_at_period_end" class="muted">
            {{ t('Cancellation is scheduled. Access continues until the end of the paid period.') }}
          </p>
          <p v-if="state.data.value.subscription.pending_plan" class="muted">
            {{ t('Next period plan') }}: {{ state.data.value.subscription.pending_plan }}
          </p>
        </div>
        <div class="console-actions">
          <button
            class="console-button"
            :disabled="busy || !state.data.value.configured"
            @click="propose('/payment-method', {}, 'Update payment method')"
          >
            {{ t('Update payment method') }}</button
          ><button
            v-if="!state.data.value.subscription.cancel_at_period_end"
            class="console-button quiet"
            :disabled="busy || !state.data.value.configured"
            @click="propose('/cancel', {}, 'Cancel subscription')"
          >
            {{ t('Cancel subscription') }}
          </button>
        </div>
      </section>
      <section class="console-section">
        <h2>{{ t('Test plans') }}</h2>
        <div class="plan-grid">
          <article
            v-for="plan in state.data.value.catalog.plans"
            :key="plan.id"
            class="console-surface plan"
          >
            <h3>{{ plan.name }}</h3>
            <p class="plan-price">
              {{ money(plan.amount, plan.currency) }} <small>/ {{ t('month') }}</small>
            </p>
            <p class="muted">
              {{ plan.included_minutes }} {{ t('included minutes') }}<br />{{ plan.concurrency }}
              {{ t('simultaneous sessions') }}
            </p>
            <button
              class="console-button"
              :disabled="
                busy ||
                !state.data.value.configured ||
                !state.data.value.can_manage ||
                state.data.value.subscription?.plan === plan.id
              "
              @click="
                propose(
                  state.data.value.subscription ? '/subscription' : '/checkout',
                  { plan: plan.id },
                  state.data.value.subscription ? 'Change plan' : 'Start test subscription',
                )
              "
            >
              {{
                t(
                  state.data.value.subscription?.plan === plan.id
                    ? 'Current plan'
                    : state.data.value.subscription
                      ? 'Change plan'
                      : 'Start test subscription',
                )
              }}
            </button>
          </article>
        </div>
        <p class="muted">
          {{
            t(
              'Upgrades require payment confirmation. Downgrades apply at the next billing period. No automatic overages.',
            )
          }}
        </p>
      </section>
      <section class="console-section">
        <h2>{{ t('Add conversation time') }}</h2>
        <p class="muted">
          {{
            t(
              'Top-ups are one-time purchases. Purchased time is retained and used after included time.',
            )
          }}
        </p>
        <div v-for="topup in state.data.value.catalog.topups" :key="topup.id" class="topup-row">
          <div>
            <strong>{{ topup.minutes }} {{ t('minutes') }}</strong>
            <p class="muted">
              {{ money(topup.amount, topup.currency) }} · {{ t('one-time test payment') }}
            </p>
          </div>
          <button
            class="console-button"
            :disabled="busy || !state.data.value.configured || !state.data.value.can_manage"
            @click="propose('/topup', { topup: topup.id }, 'Buy test top-up')"
          >
            {{ t('Buy test top-up') }}
          </button>
        </div>
      </section></template
    >
    <dialog
      ref="confirmation"
      class="console-dialog"
      :aria-label="t(pending?.label || 'Confirm billing change')"
      @cancel.prevent="cancelConfirmation"
    >
      <h2>{{ t(pending?.label || 'Confirm billing change') }}</h2>
      <p>
        {{
          t(
            pending?.path === '/cancel'
              ? 'Your subscription will end at the current period boundary. This does not delete your workspace.'
              : pending?.path === '/payment-method'
                ? 'For an overdue subscription, checkout also collects the unpaid renewal. No change occurs until you complete the payment page.'
                : 'Review this test billing action before continuing. Confirmed payments are applied only to this workspace.',
          )
        }}
      </p>
      <p v-if="actionError" class="console-notice error" role="alert">{{ t(actionError) }}</p>
      <div class="console-actions">
        <button class="console-button primary" :disabled="busy" @click="confirm">
          {{ t(busy ? 'Preparing…' : 'Confirm') }}</button
        ><button class="console-button quiet" :disabled="busy" @click="cancelConfirmation">
          {{ t('Cancel') }}
        </button>
      </div>
    </dialog>
  </section>
</template>
<style scoped>
.billing-page { --console-page-width: 960px; }
.subscription-summary {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  align-items: center;
}
.subscription-summary h2 {
  margin-top: 16px;
}
.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 260px), 1fr));
  gap: 20px;
  margin-top: 24px;
}
.plan {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
}
.plan-price {
  font-size: 30px;
  font-weight: 600;
  letter-spacing: -0.8px;
  margin: 20px 0;
}
.plan-price small {
  font-size: 13px;
  color: var(--muted);
  font-weight: 400;
}
.plan .console-button {
  width: 100%;
  margin-top: auto;
}
.topup-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  padding: 20px 0;
  border-bottom: 1px solid var(--divider);
}
.topup-row p {
  margin: 4px 0 0;
}
@media (max-width: 760px) {
  .subscription-summary {
    align-items: stretch;
    flex-direction: column;
  }
  .topup-row {
    flex-wrap: wrap;
    gap: 12px;
  }
}
</style>
