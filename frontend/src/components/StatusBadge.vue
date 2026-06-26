<template>
  <el-tag :type="type" :size="size" effect="light" round>{{ labelMap[status] ?? status }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Status } from '@/types'

const props = defineProps<{ status: Status | string; size?: 'large' | 'default' | 'small' }>()

const labelMap: Record<string, string> = {
  pending: '等待',
  running: '运行中',
  success: '成功',
  failed: '失败',
  timeout: '超时',
  canceled: '已取消',
  skipped: '跳过',
}

const type = computed(() => {
  if (props.status === 'success') return 'success'
  if (props.status === 'failed' || props.status === 'timeout' || props.status === 'canceled') return 'danger'
  if (props.status === 'running') return 'warning'
  if (props.status === 'skipped') return 'info'
  return 'info'
})
</script>
