<script setup lang="ts">
import { shallowRef } from 'vue'
import { getIntegrationEditDraft, normalizeWebsiteOrigins } from './useIntegrationJourney'
import { useLocaleStore } from '../../stores/locale'
import type { Integration, Publication, WidgetAppearance } from './types'
const props = defineProps<{
  item?: Integration
  draftKey: string
  publications: Publication[]
  busy: boolean
  error: string
}>()
const emit = defineEmits<{
  save: [
    input: {
      name: string
      publication_id: string
      allowed_origins: string[]
      capabilities: string[]
      daily_limit_seconds: number
      appearance: WidgetAppearance
    },
  ]
  cancel: []
}>()
const { t } = useLocaleStore()
const localError = shallowRef('')
const form = getIntegrationEditDraft(props.draftKey, {
  name: props.item?.name ?? '',
  publication: props.item?.publication_id ?? props.publications[0]?.id ?? '',
  origins: props.item?.allowed_origins.join('\n') ?? '',
  minutes: (props.item?.daily_limit_seconds ?? 3600) / 60,
  appearance: {
    ...(props.item?.appearance ?? {
      placement: 'floating',
      position: 'bottom-right',
      theme: 'system',
      language: 'en-US',
      launcher_label: 'Talk to us',
    }),
  } as WidgetAppearance,
})
function save() {
  if (props.busy) return
  localError.value = ''
  let origins: string[]
  try {
    origins = normalizeWebsiteOrigins(form.origins)
  } catch (failure) {
    localError.value = failure instanceof Error ? failure.message : 'Check the website address.'
    return
  }
  emit('save', {
    name: form.name.trim(),
    publication_id: form.publication,
    allowed_origins: origins,
    daily_limit_seconds: form.minutes * 60,
    capabilities: props.item?.capabilities ?? ['message', 'speak', 'interrupt', 'suspend'],
    appearance: form.appearance,
  })
}
</script>
<template>
  <form class="console-form" @submit.prevent="save">
    <label class="console-field"
      >{{ t('Integration name')
      }}<input
        v-model="form.name"
        maxlength="100"
        required
        :disabled="busy"
        :placeholder="t('Website or application name')" /></label
    ><label class="console-field"
      >{{ t('Workspace character')
      }}<select v-model="form.publication" required :disabled="busy">
        <option v-for="publication in publications" :key="publication.id" :value="publication.id">
          {{ publication.name }}
        </option>
      </select></label
    ><label class="console-field"
      >{{ t('Allowed websites')
      }}<textarea
        v-model="form.origins"
        rows="3"
        required
        :disabled="busy"
        placeholder="https://www.example.com"
      /><small>{{
        t(
          'One exact website origin per line, including https://. No wildcards or paths. Local development may use http://localhost.',
        )
      }}</small></label
    ><label class="console-field"
      >{{ t('Daily allowance in minutes')
      }}<input
        v-model.number="form.minutes"
        type="number"
        min="1"
        max="1440"
        step="1"
        required
        :disabled="busy"
    /></label>
    <div class="form-pair">
      <label class="console-field"
        >{{ t('Placement')
        }}<select v-model="form.appearance.placement" :disabled="busy">
          <option value="floating">{{ t('Floating button') }}</option>
          <option value="inline">{{ t('Inline on the page') }}</option>
        </select></label
      ><label class="console-field"
        >{{ t('Appearance')
        }}<select v-model="form.appearance.theme" :disabled="busy">
          <option value="system">{{ t('Follow device') }}</option>
          <option value="light">{{ t('Light') }}</option>
          <option value="dark">{{ t('Dark') }}</option>
        </select></label
      >
    </div>
    <div class="form-pair">
      <label class="console-field"
        >{{ t('Language')
        }}<select v-model="form.appearance.language" :disabled="busy">
          <option value="en-US">English</option>
          <option value="es-CO">Español</option>
        </select></label
      ><label v-if="form.appearance.placement === 'floating'" class="console-field"
        >{{ t('Button position')
        }}<select v-model="form.appearance.position" :disabled="busy">
          <option value="bottom-right">{{ t('Bottom right') }}</option>
          <option value="bottom-left">{{ t('Bottom left') }}</option>
        </select></label
      >
    </div>
    <label v-if="form.appearance.placement === 'floating'" class="console-field"
      >{{ t('Button label')
      }}<input v-model="form.appearance.launcher_label" maxlength="60" required :disabled="busy"
    /></label>
    <p v-if="error || localError" class="console-notice error" role="alert">
      {{ t(error || localError) }}
    </p>
    <div class="console-actions">
      <button class="console-button primary" :disabled="busy || !form.publication" type="submit">
        {{ t(busy ? 'Saving…' : 'Save draft') }}</button
      ><button class="console-button quiet" :disabled="busy" type="button" @click="emit('cancel')">
        {{ t('Cancel') }}
      </button>
    </div>
  </form>
</template>
<style scoped>
.console-form {
  width: 100%;
  min-width: 0;
  max-width: 640px;
}
.console-field {
  min-width: 0;
  font-size: 14px;
}
.console-field input,
.console-field select,
.console-field textarea {
  font-size: 15px;
  min-width: 0;
  width: 100%;
}
.form-pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
@media (max-width: 480px) {
  .form-pair {
    grid-template-columns: 1fr;
  }
}
</style>
