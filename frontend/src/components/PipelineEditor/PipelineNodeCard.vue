<template>
  <Handle type="target" :position="Position.Top" />
  <div class="pn-card" :class="{ selected: props.selected }">
    <div class="pn-index">{{ data.index + 1 }}</div>
    <div class="pn-body">
      <div class="pn-name">{{ data.node.name }}</div>
      <div class="pn-type">{{ data.node.type }}</div>
    </div>
  </div>
  <Handle id="next" type="source" :position="Position.Bottom" />
  <Handle id="fallback" type="source" :position="Position.Right" />
</template>

<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import type { PipelineNode } from '@/types'

const props = defineProps<{
  data: { node: PipelineNode; index: number }
  selected?: boolean
}>()
</script>

<style scoped>
.pn-card {
  width: 200px;
  background: #fff;
  border: 1.5px solid #dce4ef;
  border-radius: 8px;
  padding: 10px 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  transition:
    border-color 0.15s,
    box-shadow 0.15s;
}

.pn-card.selected {
  border-color: #2dd4bf;
  box-shadow: 0 0 0 3px rgba(45, 212, 191, 0.2);
}

.pn-index {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #f0fdfb;
  border: 1px solid #2dd4bf;
  color: #0d9488;
  font-size: 12px;
  font-weight: 700;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.pn-name {
  font-weight: 600;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pn-type {
  font-size: 11px;
  color: #64748b;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
