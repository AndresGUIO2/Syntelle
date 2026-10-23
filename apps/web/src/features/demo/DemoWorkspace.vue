<script setup lang="ts">
import { computed, shallowRef, useTemplateRef, watch } from 'vue'
import ConversationIcon from '../conversation/components/ConversationIcon.vue'
import { useBackgroundPreference } from '../character/composables/useBackgroundPreference'
import type { useCharacterMotion } from '../character/composables/useCharacterMotion'
import {
  demoBackgrounds,
  demoBackgroundUrl,
  demoCharacters,
  loadDemoCharacter,
  type DemoCharacterId,
  type PreparedDemoCharacter,
} from './catalog'
import { useDemoPreferences } from './composables/useDemoPreferences'
import { useWorkspaceViewport } from './composables/useWorkspaceViewport'
import DemoCharacterStage from './components/DemoCharacterStage.vue'
import CustomizationPanel from './components/CustomizationPanel.vue'
import ChatPanel from '../conversation/components/ChatPanel.vue'
import { useConversation } from '../conversation/composables/useConversation'

const props = defineProps<{ idle?: 'attention' | 'previous'; expressions?: boolean }>()
const smile = shallowRef(0),
  brows = shallowRef(0)
const selected = shallowRef<DemoCharacterId>('syntelle')
const prepared = shallowRef<PreparedDemoCharacter | null>(null)
const session = shallowRef<ReturnType<typeof useCharacterMotion> | null>(null)
const preparationError = shallowRef('')
const language = shallowRef('en-US')
const customization = shallowRef(false)
const customizeButton = useTemplateRef<HTMLButtonElement>('customizeButton')
const workspace = useTemplateRef<HTMLElement>('workspace')
const { viewportStyle, keyboardOpen } = useWorkspaceViewport(workspace)
const { blinking, movement, automaticExpressions, voices } = useDemoPreferences()
const { background, blur, selectedBackground } = useBackgroundPreference(
  demoBackgrounds,
  demoBackgroundUrl,
)
const voice = computed({
  get: () =>
    voices.value[selected.value] ?? demoCharacters.find((c) => c.id === selected.value)!.voice,
  set: (value: string) => {
    voices.value = { ...voices.value, [selected.value]: value }
  },
})
const characterError = computed(() => Boolean(preparationError.value || session.value?.error.value))
const speech = computed(() => session.value?.speech)
const chat = useConversation({ character: selected, language, voice, playback: speech })
const characterName = computed(() => demoCharacters.find((c) => c.id === selected.value)!.name)
watch(
  selected,
  async (id, _previous, onCleanup) => {
    let obsolete = false
    onCleanup(() => {
      obsolete = true
    })
    session.value?.speech.stop()
    session.value = null
    prepared.value = null
    preparationError.value = ''
    try {
      const next = await loadDemoCharacter(id, props.idle ?? 'attention', props.expressions ?? true)
      if (!obsolete) prepared.value = next
    } catch {
      if (!obsolete) preparationError.value = 'This character could not load.'
    }
  },
  { immediate: true, flush: 'sync' },
)
function attachSession(value: ReturnType<typeof useCharacterMotion> | null) {
  session.value = value
}
function closeCustomizationOnEscape(event: KeyboardEvent) {
  if (
    !customization.value ||
    event.key !== 'Escape' ||
    event.defaultPrevented ||
    event.target instanceof HTMLSelectElement
  )
    return
  event.preventDefault()
  customization.value = false
}
</script>

<template>
  <div
    ref="workspace"
    class="demo-workspace"
    :class="{ 'keyboard-open': keyboardOpen }"
    :style="viewportStyle"
    @keydown="closeCustomizationOnEscape"
  >
    <div class="workspace-panels">
      <section
        class="stage-panel"
        :aria-label="`${demoCharacters.find((c) => c.id === selected)!.name} character preview`"
        :aria-busy="!session?.ready.value && !characterError"
      >
        <DemoCharacterStage
          v-if="prepared"
          :key="prepared.id"
          :prepared="prepared"
          :text="''"
          :language="language"
          :voice="voice"
          :blinking="blinking"
          :movement="movement"
          :smile="smile"
          :brows="brows"
          :automatic-expressions="automaticExpressions"
          :background="selectedBackground"
          :blur="blur"
          @session="attachSession"
        />
        <img
          v-else
          class="loading-background"
          :src="selectedBackground"
          alt=""
          :style="{ filter: `blur(${blur}px)` }"
        />
        <p v-if="characterError" class="stage-message" role="alert">
          This character couldn’t load. Reload this page to try again.
        </p>
        <p v-else-if="!session?.ready.value" class="stage-message" role="status">
          Loading character…
        </p>
      </section>
      <ChatPanel
        class="editor-panel"
        :turns="chat.turns.value"
        :partial="chat.partial.value"
        :character="characterName"
        :configured="chat.configured.value"
        :checking="chat.checking.value"
        :ready="session?.ready.value ?? false"
        :pending="chat.pending.value"
        :speaking="speech?.active.value ?? false"
        :microphone="chat.microphone.value"
        :input-level="chat.inputLevel.value"
        :starting-microphone="chat.startingMicrophone.value"
        :recognition-available="chat.recognitionAvailable.value"
        :status="chat.status.value"
        :error="chat.error.value"
        @microphone="chat.toggleMicrophone()"
        @stop="chat.cancelReply()"
        @new-chat="chat.newChat()"
      >
        <template #actions>
          <button
            ref="customizeButton"
            class="workspace-action workspace-action-icon customize-button"
            type="button"
            aria-label="Customize"
            title="Customize"
            aria-haspopup="dialog"
            :aria-expanded="customization"
            @click="customization = true"
          >
            <ConversationIcon name="adjust" />
          </button>
        </template>
      </ChatPanel>
    </div>
    <CustomizationPanel
      v-model:open="customization"
      v-model:background="background"
      v-model:blur="blur"
      v-model:voice="voice"
      v-model:blinking="blinking"
      v-model:movement="movement"
      v-model:smile="smile"
      v-model:brows="brows"
      v-model:automatic-expressions="automaticExpressions"
      v-model:instructions="chat.instructions.value"
      :expressions-available="session?.expressions.available ?? false"
      :smile-available="session?.expressions.smileAvailable ?? false"
      :voice-locked="
        Boolean(
          speech?.active.value ||
          speech?.loading.value ||
          chat.pending.value ||
          chat.microphone.value,
        )
      "
      :return-focus="customizeButton"
      :reduced-motion="session?.clock.reducedMotion.value ?? false"
    />
  </div>
</template>

<style scoped>
.demo-workspace {
  display: grid;
  grid-template-rows: minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}
.workspace-panels {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 3fr);
  grid-template-rows: minmax(0, 1fr);
  gap: 32px;
  min-height: 0;
  height: 100%;
}
.stage-panel {
  position: relative;
  isolation: isolate;
  min-height: 0;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border: 1px solid var(--divider);
  border-radius: 16px;
  background: var(--subtle);
  box-shadow: var(--panel-shadow);
}
.stage-panel::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  border-radius: inherit;
  box-shadow: inset 0 0 0 1px #ffffff14;
  background: linear-gradient(
    100deg,
    var(--stage-edge),
    transparent 16%,
    transparent 84%,
    var(--stage-edge)
  );
}
.loading-background {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.stage-message {
  position: absolute;
  left: 16px;
  right: 16px;
  bottom: 16px;
  margin: 0;
  padding: 12px 16px;
  border-radius: 6px;
  background: var(--surface);
  color: var(--muted);
  font-size: 13px;
}
.editor-panel {
  border: 1px solid transparent;
  border-radius: 16px;
  background:
    linear-gradient(var(--conversation-surface), var(--conversation-surface)) padding-box,
    linear-gradient(
        140deg,
        var(--panel-edge),
        var(--divider) 40%,
        var(--divider) 80%,
        var(--panel-edge)
      )
      border-box;
  box-shadow: var(--panel-shadow);
  min-width: 0;
}
@media (max-width: 1000px) {
  .workspace-panels {
    height: 100%;
    gap: 24px;
  }
}
@media (max-width: 899px) {
  .demo-workspace {
    height: var(--demo-viewport-height, 100%);
  }
  .workspace-panels {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: clamp(160px, 26svh, 240px) minmax(0, 1fr);
    gap: 16px;
  }
  .stage-panel {
    height: 100%;
    aspect-ratio: auto;
  }
  .keyboard-open {
    gap: 0;
  }
  .keyboard-open .stage-panel {
    display: none;
  }
  .keyboard-open .workspace-panels {
    grid-template-rows: minmax(0, 1fr);
  }
}
@media (max-height: 650px) and (max-width: 899px) {
  .workspace-panels {
    grid-template-rows: clamp(160px, 26svh, 240px) minmax(0, 1fr);
  }
}
@media (max-height: 600px) and (min-width: 900px) {
  .demo-workspace {
    min-height: 480px;
  }
}
</style>
