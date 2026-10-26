<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import { onScopeDispose, shallowRef, useTemplateRef, watch } from 'vue'
import type { useAccount } from '../composables/useAccount'
import type { SavedConfiguration, StreamSettings } from '../types'
const { t, errorMessage } = useLocaleStore()
const open = defineModel<boolean>({ required: true })
const props = defineProps<{
  auth: ReturnType<typeof useAccount>
  settings: StreamSettings
  beforeLogin: () => Promise<void>
  flow?: string
}>()
const emit = defineEmits<{
  load: [configuration: SavedConfiguration]
  signedIn: []
  signedOut: []
  resumeStream: []
}>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const email = shallowRef('')
const code = shallowRef('')
const challenge = shallowRef('')
const confirmLink = shallowRef(false)
const requiresConfirmation = shallowRef(false)
const provider = shallowRef('')
const tab = shallowRef<'saved' | 'keys'>('saved')
const setupName = shallowRef(t('My Syntelle'))
const keyName = shallowRef(t('My integration'))
const sent = shallowRef(false)
const cooldown = shallowRef(0)
let timer: ReturnType<typeof setInterval> | undefined
let previousFocus: HTMLElement | null = null
watch([open, dialog], ([visible, element]) => {
  if (!element) return
  if (visible && !element.open) {
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    element.showModal()
    props.auth.clearError()
    if (props.auth.account.value) void props.auth.loadLibrary()
  } else if (!visible && element.open) element.close()
})
function closed() {
  open.value = false
  props.auth.forgetSecret()
  code.value = ''
  previousFocus?.focus()
}
async function sendCode() {
  const result = await props.auth.sendCode(email.value, props.flow)
  if (!result) return
  challenge.value = result.challenge_id
  requiresConfirmation.value = result.requires_confirmation ?? false
  provider.value = result.provider ?? ''
  sent.value = true
  code.value = ''
  cooldown.value = result.resend_after
  clearInterval(timer)
  timer = setInterval(() => {
    cooldown.value = Math.max(0, cooldown.value - 1)
    if (!cooldown.value) clearInterval(timer)
  }, 1000)
}
async function verify() {
  await props.beforeLogin()
  if (await props.auth.verify(challenge.value, code.value, confirmLink.value)) {
    code.value = challenge.value = ''
    sent.value = false
    confirmLink.value = false
    requiresConfirmation.value = false
    emit('signedIn')
    if (props.flow) history.replaceState(null, '', '/?view=stream')
  } else emit('resumeStream')
}
async function signOut() {
  await props.beforeLogin()
  await props.auth.logout()
  if (!props.auth.account.value) {
    sent.value = false
    code.value = challenge.value = ''
    confirmLink.value = false
    requiresConfirmation.value = false
    emit('signedOut')
  } else emit('resumeStream')
}
async function copySecret() {
  try {
    await navigator.clipboard.writeText(props.auth.secret.value)
  } catch {
    props.auth.reportError('Select and copy the key from the field below.')
  }
}
function loadConfiguration(configuration: SavedConfiguration) {
  emit('load', configuration)
  open.value = false
}
onScopeDispose(() => {
  clearInterval(timer)
  dialog.value?.close()
})
</script>

<template>
  <dialog
    ref="dialog"
    class="account-dialog"
    aria-labelledby="account-title"
    @close="closed"
    @click="$event.target === dialog && (open = false)"
  >
    <div class="account-content">
      <header class="dialog-header">
        <h2 id="account-title">
          {{ t(auth.account.value ? 'Your account' : 'Welcome to Syntelle') }}
        </h2>
        <button class="close" type="button" :aria-label="t('Close account')" @click="open = false">
          ×
        </button>
      </header>
      <template v-if="!auth.account.value">
        <p class="intro">{{ t('Save your setup and connect Syntelle to your applications.') }}</p>
        <p v-if="props.flow && auth.status.value?.email.configured" class="note">
          {{ t('Verify your email to finish connecting this sign-in method.') }}
        </p>
        <form
          v-if="auth.status.value?.email.configured && !sent"
          class="email-form"
          @submit.prevent="sendCode"
        >
          <label class="field">
            {{ t('Email address') }}
            <input
              v-model.trim="email"
              type="email"
              autocomplete="email"
              required
              maxlength="254"
              :disabled="auth.loading.value"
          /></label>
          <button class="button button-primary" type="submit" :disabled="auth.loading.value">
            {{ t(auth.loading.value ? 'Sending…' : 'Send sign-in code') }}
          </button>
        </form>
        <form
          v-else-if="auth.status.value?.email.configured"
          class="email-form"
          @submit.prevent="verify"
        >
          <p class="note">
            {{ t('Enter the six-digit code sent to') }} <strong>{{ email }}</strong>
            {{ t('. It expires in ten minutes.') }}
          </p>
          <label class="field">
            {{ t('Sign-in code') }}
            <input
              v-model="code"
              class="code-input"
              inputmode="numeric"
              autocomplete="one-time-code"
              pattern="[0-9]{6}"
              maxlength="6"
              required
          /></label>
          <label v-if="requiresConfirmation" class="link-confirm"
            ><input v-model="confirmLink" type="checkbox" required />
            {{
              t('Connect {provider} to my existing account.', {
                provider: provider || t('this sign-in method'),
              })
            }}</label
          >
          <button
            class="button button-primary"
            type="submit"
            :disabled="auth.loading.value || (requiresConfirmation && !confirmLink)"
          >
            {{ t(auth.loading.value ? 'Verifying…' : 'Continue') }}
          </button>
          <button
            class="text-action"
            type="button"
            :disabled="cooldown > 0 || auth.loading.value"
            @click="sendCode"
          >
            {{
              cooldown > 0 ? t('Resend in {seconds}s', { seconds: cooldown }) : t('Send a new code')
            }}
          </button>
        </form>
        <p v-if="auth.status.value && !auth.status.value.email.configured" class="note">
          {{ t('Email sign-in is temporarily unavailable. You can continue as a guest.') }}
        </p>
        <button class="text-action guest" type="button" @click="open = false">
          {{ t('Continue as a guest') }}
        </button>
      </template>
      <template v-else>
        <p class="account-email">{{ auth.account.value.email }}</p>
        <div class="tabs" role="group" :aria-label="t('Account sections')">
          <button type="button" :aria-pressed="tab === 'saved'" @click="tab = 'saved'">
            {{ t('Saved setups') }}
          </button>
          <button type="button" disabled :aria-label="`${t('Developer access')}: ${t('Soon')}`">
            <span>{{ t('Developer access') }}</span>
            <span class="soon-badge" aria-hidden="true">{{ t('Soon') }}</span>
          </button>
        </div>
        <section v-if="tab === 'saved'" :aria-label="t('Saved Syntelle setups')">
          <form class="save-form" @submit.prevent="auth.save(setupName, settings)">
            <label class="field">
              {{ t('Setup name') }}
              <input v-model.trim="setupName" maxlength="80" required />
            </label>
            <button class="button button-primary" :disabled="auth.loading.value">
              {{ t('Save') }}
            </button>
          </form>
          <p v-if="!auth.configurations.value.length" class="note">
            {{
              t(
                'Saved instructions and appearance settings will be available when you sign in on another device.',
              )
            }}
          </p>
          <ul class="resource-list">
            <li v-for="item in auth.configurations.value" :key="item.id">
              <strong>{{ item.name }}</strong>
              <div class="resource-actions">
                <button type="button" class="text-action" @click="loadConfiguration(item)">
                  {{ t('Use this setup') }}
                </button>
                <button
                  type="button"
                  class="text-action"
                  :disabled="auth.loading.value"
                  @click="auth.update(item.id, item.name, settings)"
                >
                  {{ t('Replace with current setup') }}
                </button>
                <button
                  type="button"
                  class="text-action delete-action"
                  :disabled="auth.loading.value"
                  @click="auth.remove(item.id)"
                >
                  {{ t('Delete saved setup') }}
                </button>
              </div>
            </li>
          </ul>
        </section>
        <section v-else :aria-label="t('Integration keys')">
          <p class="note">
            {{
              t(
                "Use these keys on your application's server to start streams and send messages. Keep them out of browser code.",
              )
            }}
          </p>
          <form class="save-form" @submit.prevent="auth.createKey(keyName)">
            <label class="field">
              {{ t('Key name') }} <input v-model.trim="keyName" maxlength="80" required /></label
            ><button class="button button-primary" :disabled="auth.loading.value">
              {{ t('Create key') }}
            </button>
          </form>
          <div v-if="auth.secret.value" class="secret">
            <label class="field">
              {{ t('Copy this key now. It is shown only once.') }}
              <input
                :value="auth.secret.value"
                readonly
                autocomplete="off"
                spellcheck="false"
                :aria-label="t('New integration key')" /></label
            ><button class="button" type="button" @click="copySecret">{{ t('Copy key') }}</button>
          </div>
          <ul class="resource-list">
            <li v-for="item in auth.keys.value" :key="item.id">
              <div>
                <strong>{{ item.name }}</strong
                ><small>{{ item.id }}</small>
              </div>
              <button
                class="text-action"
                type="button"
                :disabled="auth.loading.value"
                @click="auth.revokeKey(item.id)"
              >
                {{ t('Revoke') }}
              </button>
            </li>
          </ul>
        </section>
      </template>
      <p v-if="auth.error.value" class="error" role="alert">{{ errorMessage(auth.error.value) }}</p>
      <footer class="dialog-footer">
        <button
          v-if="auth.account.value"
          class="text-action sign-out"
          type="button"
          :disabled="auth.loading.value"
          @click="signOut"
        >
          {{ t('Sign out') }}
        </button>
        <a href="/privacy.html" target="_blank" rel="noopener noreferrer">{{
          t('Privacy Policy')
        }}</a>
      </footer>
    </div>
  </dialog>
</template>

<style scoped>
.account-dialog {
  width: min(480px, calc(100% - 32px));
  max-height: calc(100dvh - 32px);
  padding: 0;
  color: var(--text);
  background: var(--surface);
  border: 1px solid var(--divider);
  border-radius: 14px;
  box-shadow: 0 24px 80px #00000040;
  overflow-y: auto;
  overscroll-behavior: contain;
}
.account-dialog::backdrop {
  background: #08060db3;
}
.account-content {
  padding: 24px;
}
.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}
h2 {
  font-size: 24px;
  line-height: 1.2;
  font-weight: 600;
  letter-spacing: -0.6px;
  margin: 0;
}
.close {
  width: 44px;
  height: 44px;
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 28px;
  border-radius: 10px;
}
.intro,
.note,
.account-email {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}
.account-email {
  margin: 0 0 22px;
  overflow-wrap: anywhere;
}
.intro {
  margin: 4px 0 16px;
}
.providers,
.email-form {
  display: grid;
  gap: 10px;
}
.providers .button {
  width: 100%;
}
.providers-soon {
  margin: 2px 0 0;
  text-align: center;
  font-weight: 600;
}
.button:disabled {
  opacity: 0.55;
}
.separator {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 16px 0;
  color: var(--muted);
  font-size: 12px;
}
.separator::before,
.separator::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--divider);
}
.field {
  display: grid;
  gap: 8px;
  font-size: 13px;
}
input {
  width: 100%;
  min-height: 44px;
  background: var(--page);
  color: var(--text);
  border: 1px solid var(--control-border);
  padding: 10px 12px;
  border-radius: 8px;
}
.code-input {
  letter-spacing: 0.35em;
  font-size: 22px;
}
.text-action {
  min-height: 44px;
  border: 0;
  background: transparent;
  color: var(--accent);
  padding: 8px 6px;
  font-size: 13px;
}
.guest {
  display: block;
  margin: 10px auto 0;
}
.dialog-footer a {
  color: var(--accent);
  text-underline-offset: 3px;
  font-size: 12px;
}
.tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  margin: 0 0 20px;
  padding: 4px;
  border: 1px solid var(--divider);
  border-radius: 11px;
  background: var(--subtle);
}
.tabs button {
  min-height: 44px;
  border: 0;
  border-radius: 7px;
  color: var(--muted);
  background: transparent;
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.tabs button[aria-pressed='true'] {
  color: var(--text);
  background: var(--surface);
  box-shadow: 0 1px 3px #0000001a;
}
.tabs button:disabled {
  opacity: 0.78;
  cursor: not-allowed;
}
.soon-badge {
  padding: 2px 7px;
  border: 1px solid var(--divider);
  border-radius: 999px;
  color: var(--muted);
  background: var(--surface);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  line-height: 1.4;
  text-transform: uppercase;
}
.save-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: flex-end;
  gap: 12px;
  margin: 0;
  padding: 16px;
  border: 1px solid var(--divider);
  border-radius: 12px;
  background: var(--subtle);
}
.save-form .field {
  min-width: 0;
}
.save-form .button-primary {
  min-width: 84px;
  border-radius: 8px;
}
.resource-list {
  list-style: none;
  padding: 0;
  margin: 12px 0 0;
}
.resource-list li {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 14px 4px;
  border-bottom: 1px solid var(--divider);
  font-size: 13px;
}
.resource-list li:last-child {
  border-bottom: 0;
}
.resource-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.resource-actions .text-action {
  min-height: 36px;
  padding: 6px 10px;
  border: 1px solid var(--divider);
  border-radius: 8px;
  color: var(--text);
  background: var(--surface);
  font-size: 12px;
  font-weight: 500;
}
.resource-actions .delete-action {
  color: var(--error);
}
.resource-list small {
  display: block;
  font-size: 11px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.secret {
  display: grid;
  gap: 10px;
  padding: 14px;
  background: var(--subtle);
  border-radius: 8px;
}
.secret input {
  font-family: monospace;
  font-size: 11px;
}
.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px solid var(--divider);
}
.sign-out {
  min-height: 40px;
  padding: 8px 12px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  color: var(--text);
  background: var(--surface);
  font-weight: 600;
}
.link-confirm {
  display: flex;
  gap: 8px;
  font-size: 13px;
  align-items: flex-start;
}
.link-confirm input {
  width: 16px;
  min-height: 16px;
  margin: 4px 0 0;
  flex-shrink: 0;
}
.error {
  color: var(--error);
  font-size: 13px;
  line-height: 1.5;
}
@media (max-width: 450px) {
  .account-content {
    padding: 20px 18px;
  }
  h2 {
    font-size: 23px;
  }
  .save-form {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
    padding: 12px;
  }
  .tabs {
    grid-template-columns: 1fr;
  }
  .tabs button {
    justify-content: space-between;
    padding-inline: 12px;
  }
}
@media (hover: hover) {
  .tabs button:not(:disabled):hover {
    background: var(--surface);
  }
  .close:hover,
  .sign-out:hover {
    background: var(--subtle);
  }
  .resource-actions .text-action:hover {
    background: var(--subtle);
  }
}
</style>
