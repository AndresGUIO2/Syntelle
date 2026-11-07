<script setup lang="ts">
import type { Workspace } from '../workspaces/useWorkspaces'
import WorkspacePanel from '../workspaces/WorkspacePanel.vue'
import ConsoleHeading from './ConsoleHeading.vue'
import { useLocaleStore } from '../../stores/locale'
defineProps<{ workspace: Workspace; settings?: boolean }>()
defineEmits<{ navigate: [event: MouseEvent] }>()
const { t } = useLocaleStore()
</script>
<template>
  <section class="console-page console-page--operational">
    <ConsoleHeading
      :title="settings ? 'Workspace settings' : 'Team'"
      :description="
        settings
          ? 'Workspace identity, shared setups and operational activity.'
          : 'Choose who can create, integrate and manage billing.'
      "
      :workspace="workspace.name"
    /><template v-if="settings"
      ><dl class="workspace-facts">
        <div>
          <dt>{{ t('Workspace name') }}</dt>
          <dd>{{ workspace.name }}</dd>
        </div>
        <div>
          <dt>{{ t('Your role') }}</dt>
          <dd>{{ t(workspace.role) }}</dd>
        </div>
        <div>
          <dt>{{ t('Status') }}</dt>
          <dd>{{ t(workspace.suspended ? 'Suspended' : 'Active') }}</dd>
        </div>
      </dl>
      <p class="muted">
        {{
          t(
            'Workspace characters and integrations are shared. Personal character drafts, conversations and account settings remain private.',
          )
        }}
      </p>
      <WorkspacePanel
        v-if="workspace.role !== 'billing'"
        :key="workspace.id"
        class="console-section"
        embedded
        :workspace-id="workspace.id"
        initial-section="characters"
        @navigate="$emit('navigate', $event)" /></template
    ><WorkspacePanel
      v-else-if="['owner', 'administrator'].includes(workspace.role)"
      :key="workspace.id"
      embedded
      :workspace-id="workspace.id"
      initial-section="team"
      @navigate="$emit('navigate', $event)"
    />
    <p v-else class="console-notice">
      {{ t('Only owners and administrators can manage team access.') }}
    </p>
  </section>
</template>
<style scoped>
.workspace-facts {
  display: grid;
  gap: 24px;
  grid-template-columns: minmax(0, 2fr) repeat(2, minmax(0, 1fr));
  margin: 0 0 28px;
  padding-bottom: 28px;
  border-bottom: 1px solid var(--divider);
}
.workspace-facts dt {
  color: var(--muted);
  font-size: 12px;
  margin-bottom: 8px;
}
.workspace-facts dd {
  margin: 0;
  font-size: 16px;
  overflow-wrap: anywhere;
}
@media (max-width: 600px) {
  .workspace-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .workspace-facts > div:first-child {
    grid-column: 1 / -1;
  }
}
</style>
