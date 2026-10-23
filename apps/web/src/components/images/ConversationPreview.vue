<script setup lang="ts">
import AppIcon from '../AppIcon.vue'
</script>

<template>
  <footer class="conversation-preview" aria-labelledby="conversation-preview-label">
    <div class="composer">
      <label class="visually-hidden" for="message-preview">Message</label>
      <textarea
        id="message-preview"
        class="message-input"
        placeholder="Conversation is not available yet"
        rows="2"
        disabled
        aria-describedby="conversation-preview-label"
      />
      <div class="composer-controls">
        <span class="composer-hint">Voice and text conversation</span>
        <div class="composer-actions">
          <button class="composer-button" type="button" aria-label="Use microphone" disabled>
            <AppIcon name="mic" />
          </button>
          <button
            class="composer-button send-button"
            type="button"
            aria-label="Send message"
            disabled
          >
            <AppIcon name="send" />
          </button>
        </div>
      </div>
    </div>
    <p id="conversation-preview-label" class="availability-note">
      Conversation is coming in a later step.
    </p>
  </footer>
</template>

<style scoped>
.conversation-preview {
  padding: 16px 20px 14px;
  border-top: 1px solid var(--divider);
  background: var(--surface);
}
.composer {
  padding: 12px 12px 8px;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  background: var(--page);
}
.message-input {
  display: block;
  resize: none;
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 14px;
  line-height: 1.5;
  -webkit-text-fill-color: var(--muted);
  opacity: 1;
}
.message-input::placeholder {
  color: var(--muted);
  opacity: 1;
}
.composer-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.composer-hint {
  color: var(--muted);
  font-size: 12px;
}
.composer-actions {
  display: flex;
  gap: 4px;
}
.composer-button {
  width: 44px;
  min-height: 44px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 6px;
  color: var(--muted);
  background: transparent;
}
.send-button {
  background: var(--subtle);
}
.availability-note {
  margin: 8px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--muted);
}
</style>
