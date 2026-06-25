<template>
  <div class="panel">
    <template v-if="node && metadata">
      <h3>节点配置</h3>
      <el-form label-position="top">
        <el-form-item label="Name">
          <el-input v-model="node.name" />
        </el-form-item>
        <el-form-item label="Timeout Seconds">
          <el-input-number v-model="node.timeoutSeconds" :min="0" :step="10" style="width: 100%" />
        </el-form-item>
        <el-form-item label="Retry Times">
          <el-input-number v-model="node.retryTimes" :min="0" :max="5" style="width: 100%" />
        </el-form-item>
        <el-form-item label="出错时继续 (Continue on Error)">
          <el-switch v-model="node.continueOnError" />
        </el-form-item>
        <el-form-item label="成功后跳转 (Next Node)">
          <el-select v-model="node.nextNodeId" clearable style="width: 100%">
            <el-option label="结束 Pipeline" value="" />
            <el-option
              v-for="item in targetOptions"
              :key="item.id"
              :label="`${item.name} (${item.id})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="失败后跳转 (Fallback Node)">
          <el-select v-model="node.fallbackNodeId" clearable style="width: 100%">
            <el-option label="失败并结束" value="" />
            <el-option
              v-for="item in targetOptions"
              :key="item.id"
              :label="`${item.name} (${item.id})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-divider />
        <el-form-item v-for="field in metadata.fields" :key="field.name" :label="field.label">
          <el-input
            v-if="field.type === 'input'"
            v-model="params[field.name]"
            :show-password="field.secret"
          />
          <el-input
            v-else-if="field.type === 'textarea'"
            v-model="params[field.name]"
            type="textarea"
            :rows="field.name === 'script' ? 8 : 4"
          />
          <el-input-number
            v-else-if="field.type === 'number'"
            v-model="params[field.name] as number"
            :min="0"
            style="width: 100%"
          />
          <el-select
            v-else-if="field.type === 'select'"
            v-model="params[field.name]"
            style="width: 100%"
          >
            <el-option
              v-for="option in field.options || []"
              :key="option"
              :label="option"
              :value="option"
            />
          </el-select>
          <el-switch v-else-if="field.type === 'switch'" v-model="params[field.name] as boolean" />
          <el-select
            v-else-if="field.type === 'credential'"
            v-model="params[field.name]"
            clearable
            style="width: 100%"
          >
            <el-option label="无需凭据" :value="0" />
            <el-option
              v-for="credential in credentials"
              :key="credential.id"
              :label="`${credential.name} (${credential.type})`"
              :value="credential.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
    </template>
    <el-empty v-else description="选择一个节点以编辑配置" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Credential, NodeMetadata, PipelineNode } from '@/types'

const props = defineProps<{
  node?: PipelineNode
  metadata?: NodeMetadata
  nodes?: PipelineNode[]
  credentials?: Credential[]
}>()

const params = computed(() => props.node?.params || {})
const targetOptions = computed(() =>
  (props.nodes || []).filter((item) => item.id !== props.node?.id),
)
</script>

<style scoped>
h3 {
  margin: 0 0 12px;
  font-size: 15px;
}
</style>
