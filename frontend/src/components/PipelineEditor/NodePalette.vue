<template>
  <div class="panel">
    <h3>节点库</h3>
    <div v-for="[category, items] in grouped" :key="category" class="palette-group">
      <div class="palette-category">{{ categoryLabel(category) }}</div>
      <el-space direction="vertical" fill style="width: 100%">
        <el-button
          v-for="item in items"
          :key="item.type"
          plain
          class="palette-button"
          @click="$emit('add', item)"
        >
          <el-icon><Plus /></el-icon>
          <span>{{ item.name }}</span>
        </el-button>
      </el-space>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import type { NodeMetadata } from '@/types'

const props = defineProps<{ nodeTypes: NodeMetadata[] }>()
defineEmits<{ add: [node: NodeMetadata] }>()

const CATEGORY_LABELS: Record<string, string> = {
  build: '构建',
  control: '控制',
  file: '文件',
  network: '网络',
  process: '进程',
}

const grouped = computed<Map<string, NodeMetadata[]>>(() => {
  const map = new Map<string, NodeMetadata[]>()
  for (const item of props.nodeTypes) {
    const category = item.category || 'default'
    if (!map.has(category)) map.set(category, [])
    map.get(category)!.push(item)
  }
  return map
})

function categoryLabel(category: string) {
  return CATEGORY_LABELS[category] ?? category
}
</script>

<style scoped>
h3 {
  margin: 0 0 12px;
  font-size: 15px;
}

.palette-group + .palette-group {
  margin-top: 14px;
}

.palette-category {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
}

.palette-button {
  width: 100%;
  justify-content: flex-start;
}
</style>
