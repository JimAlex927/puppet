<template>
  <Handle type="target" :position="Position.Top" class="vf-handle vf-handle--in" />

  <div class="cn" :class="[`cn--${data.category}`, { 'cn--selected': selected, [`cn--${data.status}`]: !!data.status }]">
    <div class="cn-stripe" :style="{ background: color }" />
    <div class="cn-body">
      <div class="cn-icon" :style="{ background: color + '22', color }">
        <el-icon :size="14"><component :is="icon" /></el-icon>
      </div>
      <div class="cn-text">
        <div class="cn-name">{{ data.pipelineNode.name }}</div>
        <div class="cn-type">{{ data.pipelineNode.type }}</div>
      </div>
      <div v-if="data.status" class="cn-badge" :class="`cn-badge--${data.status}`">
        <el-icon v-if="data.status === 'running'" class="is-loading" :size="13"><Loading /></el-icon>
        <el-icon v-else-if="data.status === 'success'" :size="13"><Select /></el-icon>
        <el-icon v-else-if="data.status === 'failed'" :size="13"><CloseBold /></el-icon>
        <el-icon v-else-if="data.status === 'timeout'" :size="13"><Timer /></el-icon>
        <el-icon v-else :size="13"><Clock /></el-icon>
      </div>
    </div>
    <div v-if="data.status && data.durationMs" class="cn-dur">
      {{ fmtDuration(data.durationMs) }}
    </div>
  </div>

  <!-- Output handles — shown in editor mode only -->
  <template v-if="!data.status">
    <Handle id="next" type="source" :position="Position.Bottom" class="vf-handle vf-handle--next" />
    <div class="cn-handle-label cn-handle-label--bottom">成功</div>
    <Handle id="fallback" type="source" :position="Position.Right" class="vf-handle vf-handle--fallback" />
    <div class="cn-handle-label cn-handle-label--right">失败</div>
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position, type NodeProps } from '@vue-flow/core'
import {
  Clock,
  CloseBold,
  Connection,
  Document,
  Loading,
  Monitor,
  Operation,
  Select,
  Timer,
} from '@element-plus/icons-vue'
import type { PipelineNode, Status } from '@/types'
import { fmtDuration } from '@/utils/format'

type CanvasNodeData = {
  pipelineNode: PipelineNode
  category: string
  status?: Status
  durationMs?: number
}

const props = defineProps<NodeProps<CanvasNodeData>>()

const CATEGORY: Record<string, { color: string; icon: unknown }> = {
  process: { color: '#2dd4bf', icon: Monitor },
  script:  { color: '#f59e0b', icon: Document },
  http:    { color: '#6366f1', icon: Connection },
  sleep:   { color: '#94a3b8', icon: Clock },
  default: { color: '#64748b', icon: Operation },
}

const color = computed(() => CATEGORY[props.data?.category]?.color ?? CATEGORY.default.color)
const icon  = computed(() => CATEGORY[props.data?.category]?.icon  ?? CATEGORY.default.icon)
</script>

<style scoped>
.cn {
  width: 210px;
  background: #252633;
  border: 1.5px solid #3a3b4e;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  position: relative;
}

.cn--selected {
  border-color: #2dd4bf;
  box-shadow: 0 0 0 3px rgba(45, 212, 191, 0.18);
}

.cn--running {
  border-color: #3b82f6;
  animation: pulse 1.6s ease-in-out infinite;
}

.cn--success { border-color: #22c55e; }
.cn--failed  { border-color: #ef4444; }
.cn--timeout { border-color: #f97316; }
.cn--skipped { opacity: 0.5; }

@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.4); }
  50%       { box-shadow: 0 0 0 8px rgba(59, 130, 246, 0); }
}

.cn-stripe {
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
}

.cn-body {
  display: flex;
  align-items: center;
  padding: 10px 10px 10px 14px;
  gap: 9px;
}

.cn-icon {
  width: 28px; height: 28px;
  border-radius: 6px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.cn-name {
  font-size: 13px;
  font-weight: 600;
  color: #e2e8f0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 120px;
}

.cn-type {
  font-size: 11px;
  color: #8892a4;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cn-badge {
  margin-left: auto;
  flex-shrink: 0;
}

.cn-badge--success { color: #22c55e; }
.cn-badge--failed  { color: #ef4444; }
.cn-badge--running { color: #3b82f6; }
.cn-badge--timeout { color: #f97316; }
.cn-badge--pending, .cn-badge--canceled, .cn-badge--skipped { color: #64748b; }

.cn-dur {
  font-size: 10px;
  color: #64748b;
  padding: 0 14px 6px;
  margin-top: -4px;
}

/* Input handle — top, small grey dot */
:deep(.vf-handle--in) {
  width: 10px;
  height: 10px;
  border: 2px solid #1a1b23;
  border-radius: 50%;
  background: #64748b;
  cursor: crosshair;
}

/* Output handles — always-visible colored dots */
:deep(.vf-handle--next),
:deep(.vf-handle--fallback) {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2.5px solid #1a1b23;
  cursor: crosshair;
  transition: transform 0.15s, box-shadow 0.15s;
  /* label badge below/beside the dot */
  position: relative;
}

:deep(.vf-handle--next) {
  background: #2dd4bf;
  box-shadow: 0 0 0 2px rgba(45,212,191,0.3);
}

:deep(.vf-handle--fallback) {
  background: #f87171;
  box-shadow: 0 0 0 2px rgba(248,113,113,0.3);
}

:deep(.vf-handle--next:hover) {
  transform: scale(1.35);
  box-shadow: 0 0 0 4px rgba(45,212,191,0.4);
}

:deep(.vf-handle--fallback:hover) {
  transform: scale(1.35);
  box-shadow: 0 0 0 4px rgba(248,113,113,0.4);
}

/* Label badges beside the handles */
.cn-handle-label {
  position: absolute;
  font-size: 10px;
  font-weight: 600;
  pointer-events: none;
  white-space: nowrap;
  background: #1e1f2e;
  border: 1px solid #3a3b4e;
  border-radius: 3px;
  padding: 1px 5px;
  line-height: 1.5;
}

.cn-handle-label--bottom {
  bottom: -28px;
  left: 50%;
  transform: translateX(-50%);
  color: #2dd4bf;
}

.cn-handle-label--right {
  right: -36px;
  top: 50%;
  transform: translateY(-50%);
  color: #f87171;
}
</style>
