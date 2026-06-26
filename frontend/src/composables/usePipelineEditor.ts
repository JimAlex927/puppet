import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { MarkerType, type Edge, type Node } from '@vue-flow/core'
import dagre from '@dagrejs/dagre'
import { api } from '@/api'
import type { Credential, NodeMetadata, PipelineDefinition, PipelineInput, PipelineNode, Task } from '@/types'

export function usePipelineEditor(taskId: number) {
  const router = useRouter()

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
    pipeline.value?.nodes.find((n) => n.id === selectedNodeId.value),
  )
  const selectedMetadata = computed(() =>
    nodeTypes.value.find((m) => m.type === selectedNode.value?.type),
  )

  // ── Load ─────────────────────────────────────────────────────────

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

  // ── Pipeline → VueFlow ────────────────────────────────────────────

  function buildFlowElements(): { nodes: Node[]; edges: Edge[] } {
    if (!pipeline.value) return { nodes: [], edges: [] }
    const positions = computeLayout(pipeline.value.nodes)
    const nodeTypeMap = new Map(nodeTypes.value.map((m) => [m.type, m]))

    const nodes: Node[] = pipeline.value.nodes.map((node, index) => ({
      id: node.id,
      type: 'canvas-node',
      position: node.position ?? positions.get(node.id) ?? { x: index * 260, y: 0 },
      data: {
        pipelineNode: node,
        category: nodeTypeMap.get(node.type)?.category ?? 'default',
      },
    }))

    const edges = buildEdges(pipeline.value.nodes)
    return { nodes, edges }
  }

  // ── Canvas event handlers (called by PipelineEditorPage) ──────────

  function handleConnect(sourceId: string, sourceHandle: string, targetId: string) {
    const node = pipeline.value?.nodes.find((n) => n.id === sourceId)
    if (!node) return
    if (sourceHandle === 'next') {
      // clear any previously connected node's incoming from this source
      node.nextNodeId = targetId
    } else if (sourceHandle === 'fallback') {
      node.fallbackNodeId = targetId
    }
    // Future: node.outputs ??= {}; node.outputs[sourceHandle] = targetId
  }

  function handleEdgesDelete(deletions: [sourceId: string, sourceHandle: string][]) {
    for (const [sourceId, sourceHandle] of deletions) {
      const node = pipeline.value?.nodes.find((n) => n.id === sourceId)
      if (!node) continue
      if (sourceHandle === 'next') node.nextNodeId = ''
      else if (sourceHandle === 'fallback') node.fallbackNodeId = ''
    }
  }

  function handleNodesDelete(ids: string[]) {
    if (!pipeline.value) return
    const idSet = new Set(ids)
    pipeline.value.nodes = pipeline.value.nodes.filter((n) => !idSet.has(n.id))
    for (const node of pipeline.value.nodes) {
      if (idSet.has(node.nextNodeId)) node.nextNodeId = ''
      if (idSet.has(node.fallbackNodeId)) node.fallbackNodeId = ''
    }
    if (pipeline.value.startNodeId && idSet.has(pipeline.value.startNodeId)) {
      pipeline.value.startNodeId = pipeline.value.nodes[0]?.id ?? ''
    }
    if (selectedNodeId.value && idSet.has(selectedNodeId.value)) {
      selectedNodeId.value = null
    }
  }

  function createPipelineNode(
    meta: NodeMetadata,
    position: { x: number; y: number },
  ): { pipelineNode: PipelineNode; vfNode: Node } {
    const params: Record<string, unknown> = {}
    for (const f of meta.fields) {
      params[f.name] = f.default ?? (f.type === 'number' ? 0 : '')
    }
    const node: PipelineNode = {
      id: `${meta.type}-${Date.now()}`,
      name: meta.name,
      type: meta.type,
      params,
      position,
      timeoutSeconds: 60,
      retryTimes: 0,
      nextNodeId: '',
      fallbackNodeId: '',
      continueOnError: false,
    }
    pipeline.value!.nodes.push(node)
    if (!pipeline.value!.startNodeId) pipeline.value!.startNodeId = node.id
    selectedNodeId.value = node.id

    const vfNode: Node = {
      id: node.id,
      type: 'canvas-node',
      position,
      data: { pipelineNode: node, category: meta.category ?? 'default' },
    }
    return { pipelineNode: node, vfNode }
  }

  // ── Save ─────────────────────────────────────────────────────────

  async function savePipelineOnly() {
    if (!pipeline.value) return
    await api.savePipeline(taskId, serializePipeline(pipeline.value))
  }

  async function save(currentVFNodes?: Node[]) {
    if (!pipeline.value || !task.value) return
    if (!taskForm.name.trim()) {
      ElMessage.warning('任务名称不能为空')
      return
    }
    // Write back canvas positions
    if (currentVFNodes) {
      for (const vfNode of currentVFNodes) {
        const pn = pipeline.value.nodes.find((n) => n.id === vfNode.id)
        if (pn) pn.position = vfNode.position
      }
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
      router.push(`/projects/${task.value.projectId}`)
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
    buildFlowElements,
    handleConnect,
    handleEdgesDelete,
    handleNodesDelete,
    createPipelineNode,
    savePipelineOnly,
    save,
  }
}

// ── Helpers ────────────────────────────────────────────────────────────────

const NODE_W = 210
const NODE_H = 72

function computeLayout(nodes: PipelineNode[]): Map<string, { x: number; y: number }> {
  const positions = new Map<string, { x: number; y: number }>()
  if (!nodes.length) return positions

  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: 'TB', ranksep: 100, nodesep: 80 })
  g.setDefaultEdgeLabel(() => ({}))

  for (const n of nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H })
  const idSet = new Set(nodes.map((n) => n.id))
  for (const n of nodes) {
    if (n.nextNodeId && idSet.has(n.nextNodeId)) g.setEdge(n.id, n.nextNodeId)
    if (n.fallbackNodeId && idSet.has(n.fallbackNodeId)) g.setEdge(n.id, n.fallbackNodeId)
  }
  dagre.layout(g)
  for (const n of nodes) {
    const p = g.node(n.id)
    if (p) positions.set(n.id, { x: p.x - NODE_W / 2, y: p.y - NODE_H / 2 })
  }
  return positions
}

function buildEdges(nodes: PipelineNode[]): Edge[] {
  const idSet = new Set(nodes.map((n) => n.id))
  const edges: Edge[] = []
  for (const node of nodes) {
    if (node.nextNodeId && idSet.has(node.nextNodeId)) {
      edges.push(makeEdge(node.id, 'next', node.nextNodeId, '#2dd4bf', false))
    }
    if (node.fallbackNodeId && idSet.has(node.fallbackNodeId)) {
      edges.push(makeEdge(node.id, 'fallback', node.fallbackNodeId, '#f87171', true))
    }
  }
  return edges
}

function makeEdge(src: string, handle: string, tgt: string, color: string, animated: boolean): Edge {
  return {
    id: `${src}--${handle}--${tgt}`,
    source: src,
    sourceHandle: handle,
    target: tgt,
    type: 'smoothstep',
    style: { stroke: color, strokeWidth: 2 },
    markerEnd: { type: MarkerType.ArrowClosed, color, width: 18, height: 18 },
    animated,
    label: handle === 'next' ? '成功' : '失败',
    labelStyle: { fontSize: 10, fill: color, fontWeight: 600 },
    labelBgStyle: { fill: '#1a1b23', fillOpacity: 0.85 },
  }
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

function serializePipeline(data: PipelineDefinition): PipelineDefinition {
  return {
    ...data,
    inputs: data.inputs.map((item: any) => {
      const result = { ...item }
      delete result.optionsText
      result.options = !result.source && typeof item.optionsText === 'string' && item.optionsText.trim()
        ? item.optionsText.split(',').map((v: string) => v.trim()).filter(Boolean)
        : item.options || []
      if (result.source) result.options = []
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
    try { next.headers = JSON.parse(next.headers) } catch { next.headers = {} }
  }
  return next
}
