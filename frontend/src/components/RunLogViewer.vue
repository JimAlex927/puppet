<template>
  <div ref="viewer" class="log-viewer">
    <div
      v-for="log in logs"
      :key="log.id || `${log.sequence}-${log.content}`"
      :class="logClasses(log)"
    >{{ prefix(log) }}{{ log.content }}</div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import type { RunLog } from '@/types'

const props = defineProps<{ logs: RunLog[] }>()
const viewer = ref<HTMLElement>()

function prefix(log: RunLog) {
  return `[${String(log.sequence).padStart(4, '0')}] ${log.stream.padEnd(6)} `
}

function logClasses(log: RunLog) {
  const content = log.content.toLowerCase()
  const classes = [`log-${log.stream}`]
  if (
    content.includes('[node:failed]') ||
    content.includes('[task:end] status=failed') ||
    content.includes('[task:end] status=timeout') ||
    content.includes(' error=') ||
    content.includes(' failed:')
  ) {
    classes.push('log-failed')
  } else if (
    content.includes('[node:start]') ||
    content.includes('[node:end]') ||
    content.includes('[task:start]') ||
    content.includes('[task:end] status=success') ||
    content.includes(' succeeded')
  ) {
    classes.push('log-success')
  }
  return classes
}

watch(
  () => props.logs.length,
  async () => {
    await nextTick()
    if (viewer.value) viewer.value.scrollTop = viewer.value.scrollHeight
  },
)
</script>
