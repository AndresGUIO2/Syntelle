<script setup lang="ts">
import { computed } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import type { ConsoleSection } from '../../composables/usePageNavigation'
import ConsoleHeading from './ConsoleHeading.vue'
import AppIcon from '../../components/AppIcon.vue'
const props = defineProps<{ section: ConsoleSection; loading?: boolean; error?: string }>()
defineEmits<{ retry: []; navigate: [event: MouseEvent] }>()
const { t } = useLocaleStore()
const copy = computed(
  () =>
    ({
      overview: ['Overview', 'Your characters and website connections, in one place.'],
      integrations: ['Integrations', 'Connect a character to your website.'],
      usage: ['Usage', 'Conversation time and activity for your website connections.'],
      billing: ['Billing', 'Manage the plan and balance for your workspace.'],
      team: ['Team', 'Invite people to manage your website connections.'],
      settings: ['Workspace settings', 'Manage your workspace and shared access.'],
    })[props.section],
)
</script>
<template>
  <section class="console-page console-page--focused">
    <ConsoleHeading :title="copy[0]!" :description="copy[1]!" />
    <div class="route-state console-surface">
      <p v-if="loading" role="status">{{ t('Loading your workspace…') }}</p>
      <template v-else-if="error">
        <AppIcon name="workspace" />
        <h2>{{ t('Your workspace could not be loaded') }}</h2>
        <p role="alert">{{ t(error) }}</p>
        <button class="console-button" @click="$emit('retry')">{{ t('Try again') }}</button>
      </template>
      <template v-else>
        <AppIcon name="workspace" />
        <h2>{{ t('Available when you connect a website') }}</h2>
        <p>
          {{
            t(
              'Your first website connection sets up a workspace. You can create and test characters before that.',
            )
          }}
        </p>
        <a
          class="console-button primary"
          href="/console/integrations"
          @click="$emit('navigate', $event)"
          >{{ t('Connect a website') }}</a
        >
      </template>
    </div>
  </section>
</template>
<style scoped>
.route-state {
  max-width: 680px;
  padding: clamp(24px, 3vw, 36px);
}
.route-state > svg {
  width: 28px;
  height: 28px;
  color: var(--accent);
  margin-bottom: 20px;
}
.route-state p {
  color: var(--muted);
  margin: 12px 0 24px;
}
.route-state p[role='alert'] {
  color: var(--route-error, var(--muted));
}
</style>
