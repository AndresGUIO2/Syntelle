<script setup lang="ts">
import { computed, onMounted } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import type { Workspace } from '../workspaces/useWorkspaces'
import ConsoleHeading from './ConsoleHeading.vue'
import { useConsoleResource } from './useConsoleResource'
import type { Integration, Publication, Usage } from './types'
const props = defineProps<{ workspace: Workspace | null; loading?: boolean; error?: string }>()
defineEmits<{ navigate: [event: MouseEvent]; connect: []; retry: [] }>()
const { t } = useLocaleStore()
const base = `/api/v1/workspaces/${encodeURIComponent(props.workspace?.id ?? '')}`
const publications = useConsoleResource<{ items: Publication[] }>(base + '/publications')
const integrations = useConsoleResource<{ items: Integration[] }>(base + '/integrations')
const usage = useConsoleResource<Usage>(base + '/usage')
const operational = props.workspace?.role !== 'billing'
const count = computed(
  () => publications.data.value?.items.filter((item) => !item.revoked_at).length,
)
const deployed = computed(
  () =>
    integrations.data.value?.items.filter((item) => item.enabled && item.published_revision > 0)
      .length,
)
onMounted(() => {
  if (!props.workspace) return
  if (operational) {
    void publications.refresh()
    void integrations.refresh()
  }
  void usage.refresh()
})
</script>
<template>
  <section class="console-page console-page--operational">
    <template v-if="!workspace">
      <ConsoleHeading
        title="Overview"
        description="Create a character. Give it a place on your website."
      />
      <p v-if="loading" role="status">{{ t('Loading your workspace…') }}</p>
      <div v-else-if="error" class="console-notice error" role="alert">
        <p>{{ t('Your workspace could not be loaded') }}</p>
        <p>{{ t(error) }}</p>
        <button class="console-button" @click="$emit('retry')">{{ t('Try again') }}</button>
      </div>
      <div class="welcome-paths">
        <section class="console-surface welcome-path">
          <AppIcon name="image" /><span class="path-label">{{ t('Your character') }}</span>
          <h2>{{ t('Start with a portrait') }}</h2>
          <p>
            {{
              t(
                'Upload a photo, name your character, then try its voice and movement. It stays in your personal collection.',
              )
            }}
          </p>
          <div class="welcome-actions">
            <a
              class="console-button primary"
              href="/characters/new"
              @click="$emit('navigate', $event)"
              >{{ t('Create character') }}<AppIcon name="plus"
            /></a>
            <a class="welcome-link" href="/characters" @click="$emit('navigate', $event)">{{
              t('Explore your characters')
            }}</a>
          </div>
        </section>
        <section class="console-surface welcome-path">
          <AppIcon name="globe" /><span class="path-label">{{ t('Your website') }}</span>
          <h2>{{ t('Connect a character') }}</h2>
          <p>
            {{
              t(
                'Choose a ready character and decide where it appears. Set up your workspace as part of the first connection.',
              )
            }}
          </p>
          <div class="welcome-actions">
            <button
              class="console-button"
              :disabled="loading || Boolean(error)"
              @click="$emit('connect')"
            >
              {{ t('Connect a website') }}<AppIcon name="external" />
            </button>
          </div>
        </section>
      </div>
    </template>
    <template v-else>
      <ConsoleHeading
        title="Overview"
        description="Your characters, ready for your website and applications."
        :workspace="workspace.name"
        ><a
          class="console-button primary"
          :href="operational ? '/console/integrations' : '/console/billing'"
          @click="$emit('navigate', $event)"
          ><AppIcon :name="operational ? 'code' : 'card'" />{{
            t(operational ? 'Set up an integration' : 'Manage billing')
          }}</a
        ></ConsoleHeading
      >
      <p v-if="workspace.suspended" class="console-notice error" role="alert">
        {{ t('This workspace is suspended.') }}
      </p>
      <p
        v-if="publications.error.value || integrations.error.value || usage.error.value"
        class="console-notice error"
        role="alert"
      >
        {{ t(publications.error.value || integrations.error.value || usage.error.value) }}
      </p>
      <dl class="console-grid overview-stats">
        <div v-if="operational" class="console-stat">
          <dt>{{ t('Workspace characters') }}</dt>
          <dd>{{ count ?? '—' }}</dd>
        </div>
        <div v-if="operational" class="console-stat">
          <dt>{{ t('Published integrations') }}</dt>
          <dd>{{ deployed ?? '—' }}</dd>
        </div>
        <div class="console-stat">
          <dt>{{ t('Available conversation time') }}</dt>
          <dd>
            {{
              usage.data.value
                ? Math.floor(
                    (usage.data.value.included_seconds_remaining +
                      usage.data.value.topup_seconds_remaining) /
                      60,
                  )
                : '—'
            }}
            <small>{{ t('minutes') }}</small>
          </dd>
        </div>
      </dl>
      <div v-if="operational" class="overview-content">
        <section class="console-surface setup-guide">
          <span class="console-pill">{{ t('Getting started') }}</span>
          <h2>{{ t('Bring your character to your product') }}</h2>
          <p class="muted">
            {{
              t(
                'Publish a character, choose where it can appear, then connect a widget or API key.',
              )
            }}
          </p>
          <ol class="setup-steps">
            <li>
              <span class="step-number">1</span>
              <div>
                <a href="/characters" @click="$emit('navigate', $event)">{{
                  t('Choose a character')
                }}</a>
                <p>{{ t('Use Syntelle or publish one of your own prepared characters.') }}</p>
              </div>
              <AppIcon v-if="count" name="check" />
            </li>
            <li>
              <span class="step-number">2</span>
              <div>
                <a href="/console/integrations" @click="$emit('navigate', $event)">{{
                  t('Connect your website or app')
                }}</a>
                <p>{{ t('Limit each integration to its character and approved websites.') }}</p>
              </div>
              <AppIcon v-if="deployed" name="check" />
            </li>
            <li>
              <span class="step-number">3</span>
              <div>
                <a href="/console/usage" @click="$emit('navigate', $event)">{{
                  t('Keep track of usage')
                }}</a>
                <p>{{ t('See conversation time and active sessions in this workspace.') }}</p>
              </div>
            </li>
          </ol>
        </section>
        <section class="overview-note">
          <AppIcon name="shield" />
          <h2>{{ t('A clear boundary for your project') }}</h2>
          <p class="muted">
            {{
              t(
                'Characters you publish here belong to this workspace. Personal drafts and conversations stay in your account.',
              )
            }}
          </p>
          <a class="console-button quiet" href="/console/team" @click="$emit('navigate', $event)"
            >{{ t('Manage team access') }}<AppIcon name="external"
          /></a>
        </section>
      </div>
    </template>
  </section>
</template>
<style scoped>
.welcome-paths {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}
.welcome-path {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  padding: clamp(24px, 3vw, 36px);
  min-width: 0;
}
.welcome-path > svg {
  width: 32px;
  height: 32px;
  color: var(--accent);
  margin-bottom: 28px;
}
.path-label {
  color: var(--muted);
  font-size: 13px;
  margin-bottom: 12px;
}
.welcome-path h2 {
  font-size: 25px;
}
.welcome-path p {
  color: var(--muted);
  margin: 0 0 28px;
  max-width: 38ch;
}
.welcome-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
  margin-top: auto;
}
.welcome-link {
  color: var(--muted);
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  text-underline-offset: 4px;
  font-size: 14px;
}
@media (max-width: 900px) {
  .welcome-paths {
    grid-template-columns: 1fr;
  }
  .welcome-path {
    padding: 28px;
  }
}
.overview-stats {
  margin: 0 0 40px;
  gap: 32px;
}
.overview-content {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) minmax(240px, 1fr);
  gap: 32px;
  align-items: start;
}
.setup-guide > h2 {
  margin-top: 20px;
  font-size: 24px;
}
.setup-guide > p {
  margin: 0 0 24px;
}
.setup-steps {
  list-style: none;
  padding: 0;
  margin: 0;
}
.setup-steps li {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 20px 0;
  border-top: 1px solid var(--divider);
}
.setup-steps li > svg {
  margin-left: auto;
  color: var(--accent);
}
.step-number {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border: 1px solid var(--divider);
  border-radius: 50%;
  color: var(--muted);
  font-size: 12px;
}
.setup-steps a {
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
}
.setup-steps p {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 13px;
}
.overview-note {
  padding: 28px 8px;
}
.overview-note > svg {
  width: 28px;
  height: 28px;
  color: var(--accent);
  margin-bottom: 24px;
}
.overview-note .console-button {
  padding-left: 0;
}
@media (max-width: 1100px) {
  .overview-content {
    grid-template-columns: 1fr;
    gap: 16px;
  }
}
</style>
