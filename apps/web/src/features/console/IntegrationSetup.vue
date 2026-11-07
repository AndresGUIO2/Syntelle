<script setup lang="ts">
import { computed, nextTick, useTemplateRef } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import type { PersonalCharacter } from '../characters/api'
import type { Publication } from './types'
import { normalizeWebsiteOrigins, type IntegrationJourney } from './useIntegrationJourney'

const draft = defineModel<IntegrationJourney>({ required: true })
const props = defineProps<{
  characters: PersonalCharacter[]
  publications: Publication[]
  loading: boolean
  workspaceName?: string
}>()
const emit = defineEmits<{
  createWorkspace: []
  save: []
  cancel: []
  refresh: []
  recover: [id: string]
}>()
const { t } = useLocaleStore()
const heading = useTemplateRef<HTMLElement>('heading')
const ready = computed(() => props.characters.filter((item) => item.status === 'ready'))
const matches = computed(() =>
  props.publications.filter(
    (item) => !item.revoked_at && item.character_id === draft.value.characterId,
  ),
)
const selectedPublication = computed(() =>
  props.publications.find((item) => item.id === draft.value.publicationId && !item.revoked_at),
)
const characterName = computed(
  () =>
    selectedPublication.value?.name ??
    (draft.value.characterId === 'syntelle'
      ? 'Syntelle'
      : (ready.value.find((item) => item.id === draft.value.characterId)?.name ??
        t('Character unavailable'))),
)
const canContinue = computed(() =>
  Boolean(
    selectedPublication.value ||
    draft.value.characterId === 'syntelle' ||
    ready.value.some((item) => item.id === draft.value.characterId),
  ),
)
const steps = ['character', 'website', 'review'] as const
const labels = {
  workspace: 'Name your workspace',
  character: 'Choose a character',
  website: 'Your website',
  review: 'Review your connection',
}
function go(step: IntegrationJourney['step']) {
  draft.value.step = step
  draft.value.error = ''
  void nextTick(() => heading.value?.focus())
}
function chooseCharacter() {
  draft.value.sharingApproved = false
  const compatible = matches.value
  draft.value.publicationId = compatible.length === 1 ? compatible[0]!.id : ''
}
function chooseCopy() {
  const item = selectedPublication.value
  if (item) draft.value.characterId = item.character_id
  if (draft.value.uncertain === 'publication' && item) draft.value.uncertain = ''
  draft.value.sharingApproved = false
}
function nextCharacter() {
  if (!canContinue.value || props.loading || draft.value.busy) return
  if (matches.value.length > 1 && !draft.value.publicationId) {
    draft.value.error = 'Choose which workspace copy to use.'
    return
  }
  go('website')
}
function review() {
  try {
    const origins = normalizeWebsiteOrigins(draft.value.origins)
    if (!draft.value.name.trim()) {
      draft.value.error = 'Give this connection a name.'
      return
    }
    if (
      !Number.isInteger(draft.value.minutes) ||
      draft.value.minutes < 1 ||
      draft.value.minutes > 1440
    ) {
      draft.value.error = 'Use a daily allowance between 1 and 1440 minutes.'
      return
    }
    draft.value.origins = origins.join('\n')
    go('review')
  } catch (error) {
    draft.value.error = error instanceof Error ? error.message : 'Check the website address.'
  }
}
</script>
<template>
  <section class="integration-setup" :aria-busy="draft.busy">
    <header class="setup-heading">
      <div>
        <p class="setup-eyebrow">{{ t('Connect a website') }}</p>
        <h2 ref="heading" tabindex="-1">{{ t(labels[draft.step]) }}</h2>
      </div>
      <button class="console-button quiet" :disabled="draft.busy" @click="emit('cancel')">
        {{ t('Cancel setup') }}
      </button>
    </header>
    <ol v-if="draft.workspaceId" class="setup-progress" :aria-label="t('Connection steps')">
      <li
        v-for="(step, index) in steps"
        :key="step"
        :aria-current="draft.step === step ? 'step' : undefined"
      >
        <span>{{ index + 1 }}</span
        >{{ t(labels[step]) }}
      </li>
    </ol>
    <div v-if="draft.error" class="console-notice error" role="alert">{{ t(draft.error) }}</div>
    <div v-if="draft.uncertain" class="console-notice" role="status">
      <p>
        {{
          t(
            'The last request could not be confirmed. Your saved work may already exist; it will not be submitted again automatically.',
          )
        }}
      </p>
      <button
        v-if="draft.uncertain !== 'workspace'"
        class="console-button"
        :disabled="draft.busy"
        @click="emit('refresh')"
      >
        {{ t('Check saved work') }}
      </button>
    </div>
    <form
      v-if="draft.step === 'workspace'"
      class="setup-form"
      @submit.prevent="emit('createWorkspace')"
    >
      <p class="setup-description">
        {{
          t(
            'A workspace keeps your website connections, shared access and usage together. You can invite people later.',
          )
        }}
      </p>
      <label class="console-field"
        >{{ t('Workspace name')
        }}<input
          v-model="draft.workspaceName"
          required
          maxlength="100"
          autocomplete="organization"
          :disabled="draft.busy || !!draft.uncertain"
          :placeholder="t('Your business or project name')"
      /></label>
      <p class="muted">
        {{
          t('Your personal characters stay private. You choose what to share in the next steps.')
        }}
      </p>
      <button
        class="console-button primary"
        :disabled="draft.busy || !!draft.uncertain || !draft.workspaceName.trim()"
      >
        {{ t(draft.busy ? 'Creating workspace…' : 'Create workspace and continue')
        }}<AppIcon name="external" />
      </button>
    </form>
    <form v-else-if="draft.step === 'character'" class="setup-form" @submit.prevent="nextCharacter">
      <p class="setup-description">
        {{ t('Choose who visitors will meet. One character can appear on several websites.') }}
      </p>
      <label class="console-field"
        >{{ t('Character')
        }}<select
          v-model="draft.characterId"
          :disabled="draft.busy || loading"
          @change="chooseCharacter"
        >
          <option value="syntelle">Syntelle · {{ t('Included character') }}</option>
          <option v-for="item in ready" :key="item.id" :value="item.id">{{ item.name }}</option>
          <option
            v-if="
              selectedPublication &&
              !ready.some((item) => item.id === selectedPublication?.character_id) &&
              selectedPublication.character_id !== 'syntelle'
            "
            :value="selectedPublication.character_id"
          >
            {{ selectedPublication.name }}
          </option>
        </select></label
      >
      <label v-if="matches.length || publications.length" class="console-field"
        >{{ t('Workspace copy')
        }}<select
          v-model="draft.publicationId"
          :disabled="draft.busy || loading"
          @change="chooseCopy"
        >
          <option value="">
            {{ t(matches.length > 1 ? 'Choose a workspace copy' : 'Create a copy when saving') }}
          </option>
          <option v-for="item in publications" :key="item.id" :value="item.id">
            {{ item.name }} · {{ new Date(item.created_at).toLocaleDateString() }}
          </option>
        </select></label
      >
      <div class="selected-character">
        <AppIcon name="image" />
        <div>
          <strong>{{ characterName }}</strong>
          <p>
            {{
              t(
                selectedPublication
                  ? 'This shared copy is already available in your workspace.'
                  : 'A shared copy will be created only after you approve it and save the draft.',
              )
            }}
          </p>
        </div>
      </div>
      <button
        class="console-button primary"
        :disabled="draft.busy || loading || !canContinue || !!draft.uncertain"
      >
        {{ t('Continue') }}
      </button>
    </form>
    <form v-else-if="draft.step === 'website'" class="setup-form" @submit.prevent="review">
      <label class="console-field"
        >{{ t('Connection name')
        }}<input
          v-model="draft.name"
          maxlength="100"
          required
          :disabled="draft.busy"
          :placeholder="t('For example, Studio website')"
      /></label>
      <div class="console-field">
        <label for="connection-website">{{ t('Website address') }}</label
        ><textarea
          id="connection-website"
          v-model="draft.origins"
          aria-describedby="connection-website-help"
          rows="2"
          required
          :disabled="draft.busy"
          placeholder="https://www.example.com"
        /><small id="connection-website-help">{{
          t('Include https:// and use one address per line. Do not add a page path.')
        }}</small>
      </div>
      <div class="setup-pair">
        <label class="console-field"
          >{{ t('Placement')
          }}<select v-model="draft.appearance.placement">
            <option value="floating">{{ t('Floating button') }}</option>
            <option value="inline">{{ t('Inline on the page') }}</option>
          </select></label
        ><label class="console-field"
          >{{ t('Language')
          }}<select v-model="draft.appearance.language">
            <option value="en-US">English</option>
            <option value="es-CO">Español</option>
          </select></label
        >
      </div>
      <div class="setup-pair">
        <label class="console-field"
          >{{ t('Appearance')
          }}<select v-model="draft.appearance.theme">
            <option value="system">{{ t('Follow device') }}</option>
            <option value="light">{{ t('Light') }}</option>
            <option value="dark">{{ t('Dark') }}</option>
          </select></label
        ><label v-if="draft.appearance.placement === 'floating'" class="console-field"
          >{{ t('Button position')
          }}<select v-model="draft.appearance.position">
            <option value="bottom-right">{{ t('Bottom right') }}</option>
            <option value="bottom-left">{{ t('Bottom left') }}</option>
          </select></label
        >
      </div>
      <label v-if="draft.appearance.placement === 'floating'" class="console-field"
        >{{ t('Button label')
        }}<input v-model="draft.appearance.launcher_label" maxlength="60" required
      /></label>
      <details class="setup-advanced">
        <summary>{{ t('Usage limit') }}</summary>
        <label class="console-field"
          >{{ t('Daily allowance in minutes')
          }}<input
            v-model.number="draft.minutes"
            type="number"
            min="1"
            max="1440"
            step="1"
            required
        /></label>
      </details>
      <div class="console-actions">
        <button class="console-button quiet" type="button" @click="go('character')">
          {{ t('Back') }}</button
        ><button class="console-button primary">{{ t('Review connection') }}</button>
      </div>
    </form>
    <div v-else class="setup-form">
      <dl class="connection-review">
        <div>
          <dt>{{ t('Workspace') }}</dt>
          <dd>{{ workspaceName }}</dd>
        </div>
        <div>
          <dt>{{ t('Character') }}</dt>
          <dd>{{ characterName }}</dd>
        </div>
        <div>
          <dt>{{ t('Website') }}</dt>
          <dd class="review-address">{{ draft.origins }}</dd>
        </div>
        <div>
          <dt>{{ t('Placement') }}</dt>
          <dd>
            {{
              t(
                draft.appearance.placement === 'floating'
                  ? 'Floating button'
                  : 'Inline on the page',
              )
            }}
          </dd>
        </div>
        <div>
          <dt>{{ t('Daily allowance in minutes') }}</dt>
          <dd>{{ draft.minutes }}</dd>
        </div>
      </dl>
      <label v-if="!selectedPublication" class="share-approval"
        ><input v-model="draft.sharingApproved" type="checkbox" :disabled="draft.busy" /><span>{{
          t(
            'Create a shared copy of this character in this workspace. My personal original stays private and unchanged.',
          )
        }}</span></label
      >
      <p class="muted">
        {{
          t(
            'Saving creates a draft. You will publish it separately before adding it to your website.',
          )
        }}
      </p>
      <div class="console-actions">
        <button class="console-button quiet" :disabled="draft.busy" @click="go('website')">
          {{ t('Back') }}</button
        ><button
          class="console-button primary"
          :disabled="
            draft.busy || !!draft.uncertain || (!selectedPublication && !draft.sharingApproved)
          "
          @click="emit('save')"
        >
          {{ t(draft.busy ? 'Saving…' : 'Save draft') }}
        </button>
      </div>
    </div>
  </section>
</template>
<style scoped>
.integration-setup {
  max-width: 760px;
  padding: 32px;
  border-radius: 16px;
  background: var(--surface);
  width: 100%;
  min-width: 0;
}
.setup-heading {
  display: flex;
  justify-content: space-between;
  align-items: start;
  gap: 20px;
  margin-bottom: 28px;
}
.setup-eyebrow {
  font-size: 12px;
  color: var(--accent);
  margin: 0 0 8px;
  font-weight: 600;
}
.setup-heading h2:focus {
  outline: none;
}
.setup-heading h2 {
  font-size: 22px;
  line-height: 1.3;
}
.setup-progress {
  display: flex;
  list-style: none;
  gap: 24px;
  margin: 0 0 32px;
  padding: 0 0 24px;
  border-bottom: 1px solid var(--divider);
}
.setup-progress li {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--muted);
}
.setup-progress span {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border: 1px solid var(--divider);
  border-radius: 50%;
  flex-shrink: 0;
}
.setup-progress [aria-current] {
  color: var(--text);
  font-weight: 600;
}
.setup-progress [aria-current] span {
  background: var(--accent);
  color: var(--on-accent);
  border-color: transparent;
}
.setup-form {
  display: grid;
  gap: 24px;
}
.setup-form .console-field {
  min-width: 0;
  font-size: 14px;
}
.setup-form input:not([type='checkbox']),
.setup-form select,
.setup-form textarea {
  font-size: 15px;
  width: 100%;
  min-width: 0;
}
.setup-description {
  margin: 0;
  color: var(--muted);
  max-width: 60ch;
}
.setup-form > .console-button {
  justify-self: start;
}
.setup-pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}
.selected-character {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 20px;
  background: var(--subtle);
  border-radius: 10px;
}
.selected-character > svg {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  color: var(--accent);
}
.selected-character p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--muted);
}
.setup-advanced summary {
  min-height: 44px;
  cursor: pointer;
  color: var(--muted);
}
.connection-review {
  margin: 0;
  display: grid;
  gap: 20px;
}
.connection-review > div {
  display: grid;
  grid-template-columns: 150px minmax(0, 1fr);
  gap: 16px;
}
.connection-review dt {
  color: var(--muted);
  font-size: 13px;
}
.connection-review dd {
  margin: 0;
  font-size: 14px;
  overflow-wrap: anywhere;
}
.review-address {
  white-space: pre-line;
}
.share-approval {
  display: flex;
  align-items: start;
  gap: 12px;
  padding: 18px;
  background: var(--subtle);
  border-radius: 10px;
  font-size: 14px;
  line-height: 1.6;
}
.share-approval input {
  width: 20px;
  height: 20px;
  margin: 2px 0 0;
  flex-shrink: 0;
  accent-color: var(--accent);
}
@media (max-width: 650px) {
  .integration-setup {
    padding: 24px 18px;
  }
  .setup-progress {
    gap: 10px;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .setup-progress li {
    align-items: start;
    font-size: 11px;
  }
  .setup-pair {
    grid-template-columns: 1fr;
  }
  .connection-review > div {
    grid-template-columns: 1fr;
    gap: 4px;
  }
  .setup-heading {
    flex-wrap: wrap;
    margin-bottom: 20px;
  }
  .setup-heading .console-button {
    margin-left: -12px;
  }
  .setup-form {
    gap: 20px;
  }
  .selected-character {
    padding: 16px;
    gap: 12px;
  }
  .share-approval {
    padding: 16px;
  }
}
</style>
