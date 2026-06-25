<template>
  <el-tag :type="type" effect="light" round>{{ status }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Status } from '@/types'

const props = defineProps<{ status: Status | string }>()

const type = computed(() => {
  if (props.status === 'success') return 'success'
  if (props.status === 'failed' || props.status === 'timeout' || props.status === 'canceled') return 'danger'
  if (props.status === 'running') return 'warning'
  if (props.status === 'skipped') return 'info'
  return 'info'
})
</script>
