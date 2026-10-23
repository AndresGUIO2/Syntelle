<script setup lang="ts">
import CharacterStage from '../../features/character/components/CharacterStage.vue'
defineProps<{
  frame: HTMLCanvasElement | null
  background: string
  blur: number
  aspect: number
  focus?: { x: number; y: number; width: number; height: number }
}>()
</script>

<template>
  <CharacterStage
    :frame="frame"
    :background="background"
    :blur="blur"
    :aspect="aspect"
    :focus="focus"
  />
</template>
