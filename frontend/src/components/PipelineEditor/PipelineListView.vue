<template>
  <div class="panel">
    <el-tabs v-model="activeTab">
      <!-- ── Nodes Tab ──────────────────────────────────────────────── -->
      <el-tab-pane label="节点" name="nodes">
        <div class="pipeline-list">
          <div
            v-for="(node, index) in pipeline.nodes"
            :key="node.id"
            class="pipeline-node"
            :class="{ active: selectedNodeId === node.id }"
            @click="emit('update:selectedNodeId', node.id)"
          >
            <div class="node-title">
              <div>
                <strong>{{ index + 1 }}. {{ node.name }}</strong>
                <div class="muted">{{ node.type }} · {{ node.id }}</div>
                <div class="muted">
                  成功 → {{ node.nextNodeId || 'end' }}
                  <span v-if="node.fallbackNodeId"> · 失败 → {{ node.fallbackNodeId }}</span>
                  <span v-if="node.continueOnError"> · 错误继续</span>
                </div>
              </div>
              <el-space @click.stop>
                <el-button
                  :icon="ArrowUp"
                  circle
                  size="small"
                  :disabled="index === 0"
                  @click="emit('move', node.id, -1)"
                />
                <el-button
                  :icon="ArrowDown"
                  circle
                  size="small"
                  :disabled="index === pipeline.nodes.length - 1"
                  @click="emit('move', node.id, 1)"
                />
                <el-button
                  :icon="Delete"
                  circle
                  size="small"
                  type="danger"
                  @click="emit('remove', node.id)"
                />
              </el-space>
            </div>
          </div>
        </div>
        <el-empty v-if="!pipeline.nodes.length" description="从左侧添加节点" />
      </el-tab-pane>

      <!-- ── Inputs Tab ─────────────────────────────────────────────── -->
      <el-tab-pane name="inputs">
        <template #label>
          运行参数
          <el-badge
            v-if="pipeline.inputs.length"
            :value="pipeline.inputs.length"
            style="margin-left: 4px"
          />
        </template>

        <div style="margin-bottom: 12px">
          <el-button size="small" :icon="Plus" @click="emit('addInput')">添加参数</el-button>
        </div>

        <div class="pipeline-list">
          <div v-for="(input, index) in pipeline.inputs" :key="index" class="pipeline-node">
            <div class="input-row">
              <el-input v-model="input.name" placeholder="变量名" size="small" style="width: 120px" />
              <el-input v-model="input.label" placeholder="显示标签" size="small" style="width: 120px" />
              <el-select
                v-model="input.type"
                size="small"
                style="width: 100px"
                @change="onTypeChange(input)"
              >
                <el-option label="文本" value="string" />
                <el-option label="下拉" value="select" />
                <el-option label="开关" value="boolean" />
                <el-option label="数字" value="number" />
                <el-option label="文件" value="file" />
              </el-select>
              <el-checkbox v-model="input.required" size="small">必填</el-checkbox>
              <el-input
                v-if="input.type !== 'boolean' && input.type !== 'file'"
                v-model="input.default as string"
                placeholder="默认值"
                size="small"
                style="width: 110px"
              />
              <el-switch v-else-if="input.type === 'boolean'" v-model="input.default as boolean" size="small" />
              <el-checkbox v-else v-model="input.multiple" size="small">多文件</el-checkbox>
              <el-button
                :icon="Delete"
                circle
                size="small"
                type="danger"
                @click="pipeline.inputs.splice(index, 1)"
              />
            </div>

            <template v-if="input.type === 'select'">
              <div class="input-source-row">
                <span class="muted" style="font-size: 12px; margin-right: 8px">数据来源</span>
                <el-select
                  :model-value="getSourceType(input)"
                  size="small"
                  style="width: 160px"
                  @update:model-value="setSourceType(input, $event as string)"
                >
                  <el-option label="静态选项" value="static" />
                  <el-option v-for="t in sourceTypes" :key="t.type" :label="t.name" :value="t.type" />
                </el-select>
              </div>

              <div v-if="getSourceType(input) === 'static'" class="input-source-config">
                <el-input
                  v-model="input.optionsText"
                  placeholder="选项列表，逗号分隔，如：main, dev, release"
                  size="small"
                />
              </div>
              <div v-else class="input-source-config">
                <el-form label-position="left" label-width="90px" size="small">
                  <el-form-item
                    v-for="field in getSourceMeta(input)?.fields ?? []"
                    :key="field.name"
                    :label="field.label"
                    style="margin-bottom: 8px"
                  >
                    <el-input
                      v-if="field.type === 'input'"
                      v-model="(input.source!.params as any)[field.name]"
                    />
                    <el-input
                      v-else-if="field.type === 'textarea'"
                      v-model="(input.source!.params as any)[field.name]"
                      type="textarea"
                      :rows="5"
                      placeholder="标准输出每行作为一个选项"
                    />
                    <el-input-number
                      v-else-if="field.type === 'number'"
                      v-model="(input.source!.params as any)[field.name]"
                      style="width: 100%"
                    />
                    <el-select
                      v-else-if="field.type === 'select'"
                      v-model="(input.source!.params as any)[field.name]"
                      style="width: 100%"
                    >
                      <el-option
                        v-for="opt in field.options ?? []"
                        :key="opt"
                        :label="opt"
                        :value="opt"
                      />
                    </el-select>
                    <el-select
                      v-else-if="field.type === 'credential'"
                      v-model="(input.source!.params as any)[field.name]"
                      clearable
                      style="width: 100%"
                    >
                      <el-option label="无需凭据" :value="0" />
                      <el-option
                        v-for="cred in credentials"
                        :key="cred.id"
                        :label="`${cred.name} (${cred.type})`"
                        :value="cred.id"
                      />
                    </el-select>
                  </el-form-item>
                </el-form>
              </div>
            </template>
          </div>
        </div>

        <el-empty v-if="!pipeline.inputs.length" description="点击「添加参数」配置运行时参数" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ArrowDown, ArrowUp, Delete, Plus } from '@element-plus/icons-vue'
import type { Credential, InputSource, NodeMetadata, PipelineDefinition, PipelineInput } from '@/types'

const props = defineProps<{
  pipeline: PipelineDefinition
  sourceTypes: NodeMetadata[]
  credentials: Credential[]
  selectedNodeId: string | null
}>()

const emit = defineEmits<{
  'update:selectedNodeId': [id: string]
  remove: [id: string]
  move: [id: string, offset: -1 | 1]
  addInput: []
}>()

const activeTab = ref('nodes')

function getSourceType(input: PipelineInput): string {
  return input.source?.type || 'static'
}

function getSourceMeta(input: PipelineInput): NodeMetadata | undefined {
  return props.sourceTypes.find((t) => t.type === input.source?.type)
}

function setSourceType(input: PipelineInput, type: string) {
  if (type === 'static') {
    input.source = undefined
    return
  }
  const meta = props.sourceTypes.find((t) => t.type === type)
  const params: Record<string, unknown> = {}
  for (const field of meta?.fields ?? []) {
    params[field.name] = field.default ?? defaultFieldValue(field.type)
  }
  input.source = { type, params } as InputSource
  input.optionsText = ''
}

function onTypeChange(input: PipelineInput) {
  if (input.type !== 'select') {
    input.source = undefined
    input.optionsText = ''
  }
  if (input.type !== 'file') {
    input.multiple = undefined
  }
  if (input.type === 'file') {
    input.default = undefined
  }
}

function defaultFieldValue(type: string) {
  if (type === 'number') return 0
  if (type === 'credential') return 0
  if (type === 'switch') return false
  return ''
}
</script>

<style scoped>
.input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.input-source-row {
  margin-top: 10px;
  display: flex;
  align-items: center;
}

.input-source-config {
  margin-top: 8px;
  padding: 10px 12px;
  background: #f8fafc;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
}
</style>
