<script setup lang="ts">
import { onMounted } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import type { Workspace } from '../workspaces/useWorkspaces'
import ConsoleHeading from './ConsoleHeading.vue'
import { useConsoleResource } from './useConsoleResource'
import type { Usage } from './types'
const props = defineProps<{ workspace: Workspace }>()
const { t, locale } = useLocaleStore()
const state = useConsoleResource<Usage>(
  `/api/v1/workspaces/${encodeURIComponent(props.workspace.id)}/usage`,
)
const minutes = (seconds: number) =>
  new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(seconds / 60)
const date = (value: string) => (value ? new Date(value).toLocaleDateString(locale) : '—')
onMounted(state.refresh)
</script>
<template>
  <section class="console-page console-page--operational">
    <ConsoleHeading
      title="Usage"
      description="Conversation time and session capacity for this workspace."
      :workspace="workspace.name"
      ><button class="console-button" :disabled="state.loading.value" @click="state.refresh">
        {{ t('Refresh') }}
      </button></ConsoleHeading
    >
    <p v-if="state.error.value" class="console-notice error" role="alert">
      {{ t(state.error.value) }}
    </p>
    <p v-if="state.loading.value && !state.data.value" role="status">{{ t('Loading…') }}</p>
    <template v-if="state.data.value"
      ><dl class="console-grid">
        <div class="console-stat">
          <dt>{{ t('Used this period') }}</dt>
          <dd>
            {{ minutes(state.data.value.used_seconds) }} <small>{{ t('minutes') }}</small>
          </dd>
        </div>
        <div class="console-stat">
          <dt>{{ t('Included time remaining') }}</dt>
          <dd>
            {{ minutes(state.data.value.included_seconds_remaining) }}
            <small>{{ t('minutes') }}</small>
          </dd>
        </div>
        <div class="console-stat">
          <dt>{{ t('Top-up time remaining') }}</dt>
          <dd>
            {{ minutes(state.data.value.topup_seconds_remaining) }}
            <small>{{ t('minutes') }}</small>
          </dd>
        </div>
      </dl>
      <section class="console-section">
        <div class="console-section-heading">
          <h2>{{ t('Session capacity') }}</h2>
          <span class="console-pill"
            >{{ state.data.value.active_sessions }} / {{ state.data.value.concurrency_limit }}
            {{ t('active sessions') }}</span
          >
        </div>
        <p class="muted">
          {{
            t(
              'Active conversation time includes silence. Usage stops when the session ends or is suspended.',
            )
          }}
        </p>
        <p class="muted">
          {{ t('Reserved for current sessions') }}: {{ minutes(state.data.value.reserved_seconds) }}
          {{ t('minutes') }} · {{ t('Maximum session length') }}:
          {{ minutes(state.data.value.max_session_seconds) }} {{ t('minutes') }}
        </p>
        <p v-if="state.data.value.period_start" class="muted">
          {{ date(state.data.value.period_start) }} – {{ date(state.data.value.period_end) }}
        </p>
      </section>
      <section class="console-section">
        <h2>{{ t('By integration') }}</h2>
        <div v-if="state.data.value.by_integration?.length" class="console-table-wrap">
          <table class="console-table">
            <thead>
              <tr>
                <th>{{ t('Integration') }}</th>
                <th>{{ t('Conversation time') }}</th>
                <th>{{ t('Sessions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in state.data.value.by_integration" :key="item.integration_id">
                <td>{{ item.integration_id }}</td>
                <td>{{ minutes(item.seconds) }} {{ t('minutes') }}</td>
                <td>{{ item.sessions }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="muted">
          {{ t('No metered sessions yet. Usage will appear after an integration is used.') }}
        </p>
      </section></template
    >
  </section>
</template>
