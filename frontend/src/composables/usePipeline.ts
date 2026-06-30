import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '@/api'
import type { Credential, InputSource, NodeMetadata, PipelineDefinition, PipelineInput, PipelineNode, Task } from '@/types'

export function usePipeline(taskId: number) {
  const pipeline = ref<PipelineDefinition | null>(null)
  const task = ref<Task | null>(null)
  const projectName = ref('')
  const nodeTypes = ref<NodeMetadata[]>([])
  const sourceTypes = ref<NodeMetadata[]>([])
  const credentials = ref<Credential[]>([])
  const loading = ref(true)
  const saving = ref(false)
  const selectedNodeId = ref<string | null>(null)

  const taskForm = reactive({
    name: '',
    description: '',
    timeoutSeconds: 600,
    allowConcurrent: false,
  })

  const selectedNode = computed(() =>
    pipeline.value?.nodes.find((n) => n.id === selectedNodeId.value) ?? undefined,
  )
  const selectedMetadata = computed(() =>
    nodeTypes.value.find((m) => m.type === selectedNode.value?.type),
  )

  async function load() {
    loading.value = true
    try {
      const [data, types, sources, creds, taskData] = await Promise.all([
        api.pipeline(taskId),
        api.nodeTypes(),
        api.sourceTypes(),
        api.credentials(),
        api.task(taskId),
      ])
      pipeline.value = normalizePipeline(data)
      nodeTypes.value = types
      sourceTypes.value = sources
      credentials.value = creds
      task.value = taskData
      Object.assign(taskForm, {
        name: taskData.name,
        description: taskData.description,
        timeoutSeconds: taskData.timeoutSeconds,
        allowConcurrent: taskData.allowConcurrent,
      })
      const proj = await api.project(taskData.projectId)
      projectName.value = proj.name
    } finally {
      loading.value = false
    }
  }

  function addNode(meta: NodeMetadata) {
    if (!pipeline.value) return
    const params: Record<string, unknown> = {}
    for (const field of meta.fields) {
      params[field.name] = field.default ?? defaultFieldValue(field.type)
    }
    const node: PipelineNode = {
      id: `${meta.type}-${Date.now()}`,
      name: meta.name,
      type: meta.type,
      params,
      timeoutSeconds: 60,
      retryTimes: 0,
      nextNodeId: '',
      fallbackNodeId: '',
      continueOnError: false,
    }
    const prev = pipeline.value.nodes[pipeline.value.nodes.length - 1]
    if (prev && !prev.nextNodeId) prev.nextNodeId = node.id
    pipeline.value.nodes.push(node)
    if (!pipeline.value.startNodeId) pipeline.value.startNodeId = node.id
    selectedNodeId.value = node.id
  }

  function removeNode(id: string) {
    if (!pipeline.value) return
    const idx = pipeline.value.nodes.findIndex((n) => n.id === id)
    if (idx < 0) return
    pipeline.value.nodes.splice(idx, 1)
    for (const n of pipeline.value.nodes) {
      if (n.nextNodeId === id) n.nextNodeId = ''
      if (n.fallbackNodeId === id) n.fallbackNodeId = ''
    }
    if (pipeline.value.startNodeId === id) {
      pipeline.value.startNodeId = pipeline.value.nodes[0]?.id || ''
    }
    if (selectedNodeId.value === id) {
      selectedNodeId.value = pipeline.value.nodes[Math.max(0, idx - 1)]?.id ?? null
    }
  }

  function moveNode(id: string, offset: -1 | 1) {
    if (!pipeline.value) return
    const idx = pipeline.value.nodes.findIndex((n) => n.id === id)
    if (idx < 0) return
    const target = idx + offset
    if (target < 0 || target >= pipeline.value.nodes.length) return
    const nodes = pipeline.value.nodes
    ;[nodes[idx], nodes[target]] = [nodes[target], nodes[idx]]
  }

  function addInput() {
    if (!pipeline.value) return
    pipeline.value.inputs.push({
      name: `param${pipeline.value.inputs.length + 1}`,
      label: '参数',
      type: 'string',
      required: false,
      default: '',
      optionsText: '',
    })
  }

  async function save() {
    if (!pipeline.value || !task.value) return
    if (!taskForm.name.trim()) {
      ElMessage.warning('任务名称不能为空')
      return
    }
    saving.value = true
    try {
      await Promise.all([
        api.updateTask(taskId, {
          name: taskForm.name,
          description: taskForm.description,
          timeoutSeconds: taskForm.timeoutSeconds,
          allowConcurrent: taskForm.allowConcurrent,
        }),
        api.savePipeline(taskId, serializePipeline(pipeline.value)),
      ])
      ElMessage.success('已保存')
    } finally {
      saving.value = false
    }
  }

  return {
    pipeline,
    task,
    projectName,
    nodeTypes,
    sourceTypes,
    credentials,
    loading,
    saving,
    selectedNodeId,
    selectedNode,
    selectedMetadata,
    taskForm,
    load,
    addNode,
    removeNode,
    moveNode,
    addInput,
    save,
  }
}

function defaultFieldValue(type: string) {
  if (type === 'number') return 0
  if (type === 'credential') return 0
  if (type === 'switch') return false
  return ''
}

// ── helpers (module-level, not exported) ────────────────────────────────────

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

export type { InputSource }
