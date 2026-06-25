<template>
  <div class="panel">
    <h3>节点库</h3>
    <el-space direction="vertical" fill style="width: 100%">
      <el-button
        v-for="item in nodeTypes"
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
</template>

<script setup lang="ts">
import { Plus } from '@element-plus/icons-vue'
import type { NodeMetadata } from '@/types'

defineProps<{ nodeTypes: NodeMetadata[] }>()
defineEmits<{ add: [node: NodeMetadata] }>()
</script>

<style scoped>
h3 {
  margin: 0 0 12px;
  font-size: 15px;
}

.palette-button {
  width: 100%;
  justify-content: flex-start;
}
</style>
