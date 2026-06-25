<template>
  <div>
    <div class="page-actions">
      <el-form :inline="true" v-if="pipeline">
        <el-form-item label="Pipeline">
          <el-input v-model="pipeline.name" style="width: 220px" />
        </el-form-item>
        <el-form-item label="Agent Labels">
          <el-select v-model="pipeline.agentSelector.labels" multiple style="width: 200px">
            <el-option label="local" value="local" />
          </el-select>
        </el-form-item>
        <el-form-item label="Start Node">
          <el-select v-model="pipeline.startNodeId" style="width: 220px" clearable>
            <el-option
              v-for="item in pipeline.nodes"
              :key="item.id"
              :label="`${item.name} (${item.id})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <el-space>
        <el-button :icon="Back" @click="$router.back()">返回</el-button>
        <el-button type="primary" :icon="DocumentChecked" @click="save">保存</el-button>
      </el-space>
    </div>

    <div v-if="pipeline" class="pipeline-layout">
      <NodePalette :node-types="nodeTypes" @add="addNode" />

      <div class="panel">
        <el-tabs v-model="activeTab">
          <el-tab-pane label="节点" name="nodes">
            <div class="pipeline-list">
              <div
                v-for="(item, index) in pipeline.nodes"
                :key="item.id"
                class="pipeline-node"
                :class="{ active: selectedIndex === index }"
                @click="selectedIndex = index"
              >
                <div class="node-title">
                  <div>
                    <strong>{{ index + 1 }}. {{ item.name }}</strong>
                    <div class="muted">{{ item.type }} · {{ item.id }}</div>
                    <div class="muted">
                      成功 → {{ item.nextNodeId || 'end' }}
                      <span v-if="item.fallbackNodeId"> · 失败 → {{ item.fallbackNodeId }}</span>
                      <span v-if="item.continueOnError"> · 错误继续</span>
                    </div>
                  </div>
                  <el-space @click.stop>
                    <el-button :icon="ArrowUp" circle size="small" :disabled="index === 0" @click="move(index, -1)" />
                    <el-button
                      :icon="ArrowDown"
                      circle
                      size="small"
                      :disabled="index === pipeline.nodes.length - 1"
                      @click="move(index, 1)"
                    />
                    <el-button :icon="Delete" circle size="small" type="danger" @click="removeNode(index)" />
                  </el-space>
                </div>
              </div>
            </div>
            <el-empty v-if="pipeline.nodes.length === 0" description="从左侧添加节点" />
          </el-tab-pane>

          <el-tab-pane name="inputs">
            <template #label>
              运行参数
              <el-badge v-if="pipeline.inputs.length > 0" :value="pipeline.inputs.length" style="margin-left: 4px" />
            </template>
            <div style="margin-bottom: 12px">
              <el-button size="small" :icon="Plus" @click="addInput">添加参数</el-button>
            </div>
            <div class="pipeline-list">
              <div v-for="(input, index) in pipeline.inputs" :key="index" class="pipeline-node">
                <!-- Core fields row -->
                <div class="input-row">
                  <el-input v-model="input.name" placeholder="变量名" size="small" style="width: 120px" />
                  <el-input v-model="input.label" placeholder="显示标签" size="small" style="width: 120px" />
                  <el-select
                    v-model="input.type"
                    size="small"
                    style="width: 100px"
                    @change="onInputTypeChange(input)"
                  >
                    <el-option label="文本" value="string" />
                    <el-option label="下拉" value="select" />
                    <el-option label="开关" value="boolean" />
                    <el-option label="数字" value="number" />
                  </el-select>
                  <el-checkbox v-model="input.required" size="small">必填</el-checkbox>
                  <el-input
                    v-if="input.type !== 'boolean'"
                    v-model="input.default as string"
                    placeholder="默认值"
                    size="small"
                    style="width: 110px"
                  />
                  <el-switch v-else v-model="input.default as boolean" size="small" />
                  <el-button
                    :icon="Delete"
                    circle
                    size="small"
                    type="danger"
                    @click="pipeline.inputs.splice(index, 1)"
                  />
                </div>

                <!-- Select: source config -->
                <template v-if="input.type === 'select'">
                  <div class="input-source-row">
                    <span class="muted" style="font-size: 12px; margin-right: 8px">来源</span>
                    <el-select
                      :model-value="getSourceType(input)"
                      size="small"
                      style="width: 160px"
                      @update:model-value="setSourceType(input, $event as string)"
                    >
                      <el-option label="静态选项" value="static" />
                      <el-option
                        v-for="t in sourceTypes"
                        :key="t.type"
                        :label="t.name"
                        :value="t.type"
                      />
                    </el-select>
                  </div>
                  <!-- Static options -->
                  <div v-if="getSourceType(input) === 'static'" class="input-source-config">
                    <el-input
                      v-model="input.optionsText"
                      placeholder="选项列表，逗号分隔，如：main, dev, release"
                      size="small"
                    />
                  </div>
                  <!-- Dynamic source: fields rendered from backend metadata -->
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
                          placeholder="标准输出每行作为一个选项&#10;&#10;示例：&#10;curl -s https://api.example.com/items | jq -r '.[].name'"
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
            <el-empty v-if="pipeline.inputs.length === 0" description="点击「添加参数」配置运行时参数" />
          </el-tab-pane>
        </el-tabs>
      </div>

      <NodeConfigPanel
        :node="selectedNode"
        :metadata="selectedMetadata"
        :nodes="pipeline.nodes"
        :credentials="credentials"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowDown, ArrowUp, Back, Delete, DocumentChecked, Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { api } from '@/api'
import NodeConfigPanel from '@/components/PipelineEditor/NodeConfigPanel.vue'
import NodePalette from '@/components/PipelineEditor/NodePalette.vue'
import type {
  Credential,
  InputSource,
  NodeMetadata,
  PipelineDefinition,
  PipelineInput,
  PipelineNode,
} from '@/types'

const route = useRoute()
const taskId = Number(route.params.id)
const pipeline = ref<PipelineDefinition>()
const nodeTypes = ref<NodeMetadata[]>([])
const sourceTypes = ref<NodeMetadata[]>([])
const credentials = ref<Credential[]>([])
const selectedIndex = ref(0)
const activeTab = ref('nodes')

const selectedNode = computed(() =>
  selectedIndex.value >= 0 ? pipeline.value?.nodes[selectedIndex.value] : undefined,
)
const selectedMetadata = computed(() =>
  nodeTypes.value.find((item) => item.type === selectedNode.value?.type),
)

async function load() {
  const [data, types, sources, credentialItems] = await Promise.all([
    api.pipeline(taskId),
    api.nodeTypes(),
    api.sourceTypes(),
    api.credentials(),
  ])
  pipeline.value = normalizePipeline(data)
  nodeTypes.value = types
  sourceTypes.value = sources
  credentials.value = credentialItems
}

function normalizePipeline(data: PipelineDefinition): PipelineDefinition {
  return {
    name: data.name || 'Pipeline',
    startNodeId: data.startNodeId || data.nodes?.[0]?.id || '',
    agentSelector: data.agentSelector || { labels: ['local'] },
    inputs: (data.inputs || []).map((item) => ({
      ...item,
      optionsText: item.source ? '' : (item.options || []).join(', '),
    })) as PipelineInput[],
    nodes: (data.nodes || []).map((item, index) => ({
      ...item,
      id: item.id || `node-${index + 1}`,
      params: normalizeParams(item.params),
      timeoutSeconds: item.timeoutSeconds || 60,
      retryTimes: item.retryTimes || 0,
      nextNodeId: item.nextNodeId || '',
      fallbackNodeId: item.fallbackNodeId || '',
      continueOnError: Boolean(item.continueOnError),
    })),
  }
}

function normalizeParams(params: Record<string, unknown>) {
  const next = { ...(params || {}) }
  if (next.headers && typeof next.headers === 'object') {
    next.headers = JSON.stringify(next.headers, null, 2)
  }
  return next
}

function getSourceType(input: PipelineInput): string {
  return input.source?.type || 'static'
}

function getSourceMeta(input: PipelineInput): NodeMetadata | undefined {
  return sourceTypes.value.find((t) => t.type === input.source?.type)
}

function setSourceType(input: PipelineInput, type: string) {
  if (type === 'static') {
    input.source = undefined
    return
  }
  const meta = sourceTypes.value.find((t) => t.type === type)
  const params: Record<string, unknown> = {}
  for (const field of meta?.fields ?? []) {
    params[field.name] =
      field.default ?? (field.type === 'number' ? 0 : field.type === 'credential' ? 0 : '')
  }
  input.source = { type, params } as InputSource
  input.optionsText = ''
}

function onInputTypeChange(input: PipelineInput) {
  if (input.type !== 'select') {
    input.source = undefined
    input.optionsText = ''
  }
}

function addNode(meta: NodeMetadata) {
  if (!pipeline.value) return
  const params: Record<string, unknown> = {}
  for (const field of meta.fields) {
    params[field.name] = field.default ?? (field.type === 'number' ? 0 : '')
  }
  const node: PipelineNode = {
    id: `${meta.type}-${Date.now()}`,
    name: meta.name,
    type: meta.type,
    params,
    timeoutSeconds: meta.type === 'sleep' ? 30 : 60,
    retryTimes: 0,
    nextNodeId: '',
    fallbackNodeId: '',
    continueOnError: false,
  }
  const previous = pipeline.value.nodes[pipeline.value.nodes.length - 1]
  if (previous && !previous.nextNodeId) {
    previous.nextNodeId = node.id
  }
  pipeline.value.nodes.push(node)
  if (!pipeline.value.startNodeId) {
    pipeline.value.startNodeId = node.id
  }
  selectedIndex.value = pipeline.value.nodes.length - 1
  activeTab.value = 'nodes'
}

function move(index: number, offset: number) {
  if (!pipeline.value) return
  const target = index + offset
  const nodes = pipeline.value.nodes
  const [item] = nodes.splice(index, 1)
  nodes.splice(target, 0, item)
  selectedIndex.value = target
}

function removeNode(index: number) {
  if (!pipeline.value) return
  const [removed] = pipeline.value.nodes.splice(index, 1)
  for (const item of pipeline.value.nodes) {
    if (item.nextNodeId === removed.id) item.nextNodeId = ''
    if (item.fallbackNodeId === removed.id) item.fallbackNodeId = ''
  }
  if (pipeline.value.startNodeId === removed.id) {
    pipeline.value.startNodeId = pipeline.value.nodes[0]?.id || ''
  }
  selectedIndex.value = Math.max(0, index - 1)
}

function addInput() {
  pipeline.value?.inputs.push({
    name: `param${(pipeline.value.inputs.length || 0) + 1}`,
    label: '参数',
    type: 'string',
    required: false,
    default: '',
    optionsText: '',
  })
}

async function save() {
  if (!pipeline.value) return
  await api.savePipeline(taskId, serializePipeline(pipeline.value))
  ElMessage.success('Pipeline 已保存')
}

function serializePipeline(data: PipelineDefinition): PipelineDefinition {
  return {
    ...data,
    inputs: data.inputs.map((item: any) => {
      const result = { ...item }
      delete result.optionsText
      if (!result.source) {
        result.options =
          typeof item.optionsText === 'string' && item.optionsText.trim()
            ? item.optionsText
                .split(',')
                .map((v: string) => v.trim())
                .filter(Boolean)
            : item.options || []
      } else {
        result.options = []
      }
      return result
    }),
    nodes: data.nodes.map((item) => ({
      ...item,
      params: serializeParams(item.params),
    })),
  }
}

function serializeParams(params: Record<string, unknown>) {
  const next = { ...params }
  if (typeof next.headers === 'string' && next.headers.trim()) {
    try {
      next.headers = JSON.parse(next.headers)
    } catch {
      next.headers = {}
    }
  }
  return next
}

onMounted(load)
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
}

.input-source-config {
  margin-top: 8px;
  padding: 10px 12px;
  background: #f8fafc;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
}
</style>
