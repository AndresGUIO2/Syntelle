<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import { request } from '../streaming/api'
import type { Workspace } from '../workspaces/useWorkspaces'
import type { Publication } from '../console/types'
import { useConsoleResource } from '../console/useConsoleResource'
import type { CharacterConnection } from './useCharacterJourney'
const props = defineProps<{ workspace: Workspace }>()
const emit = defineEmits<{ connect: [selection: CharacterConnection] }>()
const { t } = useLocaleStore()
const base = `/api/v1/workspaces/${encodeURIComponent(props.workspace.id)}/publications`
const state = useConsoleResource<{ items: Publication[] }>(base)
const items = computed(() => state.data.value?.items.filter((item) => !item.revoked_at) ?? [])
const manage = computed(
  () => ['owner', 'administrator'].includes(props.workspace.role) && !props.workspace.suspended,
)
const confirming = shallowRef('')
const busy = shallowRef(false)
const error = shallowRef('')
let disposed = false
async function remove(item: Publication) {
  if (busy.value || !manage.value) return
  busy.value = true
  error.value = ''
  try {
    await request(`${base}/${encodeURIComponent(item.id)}`, 'DELETE')
    if (disposed) return
    confirming.value = ''
    await state.refresh()
  } catch (failure) {
    if (!disposed)
      error.value =
        failure instanceof Error ? failure.message : 'The character could not be removed.'
  } finally {
    if (!disposed) busy.value = false
  }
}
onMounted(state.refresh)
onScopeDispose(() => {
  disposed = true
})
</script>
<template>
  <details class="shared-collection">
    <summary>
      <AppIcon name="workspace" /><span
        >{{ t('Shared in workspace') }}<small>{{ workspace.name }}</small></span
      ><span class="shared-count">{{ items.length }}</span
      ><AppIcon name="chevron-down" />
    </summary>
    <div class="shared-content">
      <p class="shared-note">
        {{
          t(
            'These workspace-owned copies stay available independently of your personal collection.',
          )
        }}
      </p>
      <p v-if="state.error.value || error" class="console-notice error" role="alert">
        {{ t(state.error.value || error)
        }}<button class="console-button quiet" @click="state.refresh">{{ t('Try again') }}</button>
      </p>
      <p v-else-if="state.loading.value && !state.data.value" role="status">{{ t('Loading…') }}</p>
      <p v-else-if="!items.length" class="shared-note">
        {{
          t(
            'No shared characters yet. Choose a personal character when you connect your first website.',
          )
        }}
      </p>
      <ul v-else class="shared-list">
        <li v-for="item in items" :key="item.id">
          <div class="shared-row">
            <span class="shared-avatar" aria-hidden="true">{{
              item.name.slice(0, 1).toLocaleUpperCase()
            }}</span
            ><strong>{{ item.name }}</strong>
            <div class="shared-actions">
              <button
                v-if="manage"
                class="console-button"
                @click="
                  emit('connect', {
                    characterId: item.character_id,
                    publicationId: item.id,
                    workspaceId: workspace.id,
                  })
                "
              >
                {{ t('Connect to website') }}</button
              ><button v-if="manage" class="console-button quiet" @click="confirming = item.id">
                {{ t('Remove') }}
              </button>
            </div>
          </div>
          <div v-if="confirming === item.id" class="shared-confirmation">
            <p>
              {{
                t(
                  'Removing this shared copy stops integrations that use it. Your personal original stays private and unchanged.',
                )
              }}
            </p>
            <div class="shared-actions">
              <button class="console-button quiet" :disabled="busy" @click="confirming = ''">
                {{ t('Cancel') }}</button
              ><button class="console-button danger" :disabled="busy" @click="remove(item)">
                {{ t('Remove workspace character') }}
              </button>
            </div>
          </div>
        </li>
      </ul>
    </div>
  </details>
</template>
<style scoped>
.shared-collection {
  max-width: 1040px;
  margin-top: 48px;
  padding-top: 28px;
  border-top: 1px solid var(--divider);
}
.shared-collection summary > span:first-of-type {
  min-width: 0;
  overflow-wrap: anywhere;
}
.shared-collection summary {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 52px;
  cursor: pointer;
  list-style: none;
  font-size: 16px;
  font-weight: 600;
}
.shared-collection summary::-webkit-details-marker {
  display: none;
}
.shared-collection summary > svg {
  color: var(--muted);
  flex-shrink: 0;
}
.shared-collection summary > svg:last-child {
  margin-left: auto;
}
.shared-collection[open] summary > svg:last-child {
  rotate: 180deg;
}
.shared-collection summary:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}
.shared-collection small {
  display: block;
  font-size: 12px;
  color: var(--muted);
  font-weight: 400;
  margin-top: 4px;
}
.shared-count {
  color: var(--muted);
  font-size: 13px;
}
.shared-content {
  margin-top: 24px;
}
.shared-note {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
}
.shared-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.shared-list li {
  padding: 18px 0;
  border-bottom: 1px solid var(--divider);
}
.shared-row {
  display: flex;
  align-items: center;
  gap: 14px;
}
.shared-row strong {
  min-width: 0;
  flex: 1;
  font-size: 14px;
  overflow-wrap: anywhere;
}
.shared-avatar {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 42px;
  height: 48px;
  background: var(--subtle);
  color: var(--accent);
  border-radius: 8px;
  font-weight: 600;
}
.shared-actions {
  margin-left: auto;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.shared-confirmation {
  margin: 18px 0 0 56px;
}
.shared-confirmation p {
  color: var(--muted);
  font-size: 13px;
}
@media (max-width: 600px) {
  .shared-row {
    flex-wrap: wrap;
  }
  .shared-actions {
    margin-left: 0;
    width: 100%;
  }
  .shared-confirmation {
    margin-left: 0;
  }
}
</style>
