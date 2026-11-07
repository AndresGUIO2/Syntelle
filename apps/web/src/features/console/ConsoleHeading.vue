<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'
defineProps<{ title: string; description: string; workspace?: string }>()
const { t } = useLocaleStore()
</script>
<template>
  <header class="console-heading">
    <div>
      <p v-if="workspace" class="console-breadcrumb">
        {{ workspace }}
      </p>
      <h1>{{ t(title) }}</h1>
      <p class="console-intro">{{ t(description) }}</p>
    </div>
    <div v-if="$slots.default" class="console-heading-actions"><slot /></div>
  </header>
</template>
