<template>
  <div class="canvas-wrap">
    <VueFlow
      id="puppet-canvas"
      :node-types="nodeTypes"
      :default-edge-options="defaultEdgeOpts"
      fit-view-on-init
      :delete-key-code="['Backspace', 'Delete']"
      class="pipeline-flow"
      @node-click="onNodeClick"
      @pane-click="emit('pane-click')"
      @nodes-delete="onNodesDelete"
      @edges-delete="onEdgesDelete"
      @connect="handleConnect"
      @dragover="onDragOver"
      @drop="onDrop"
    >
      <Background
        :gap="20"
        :size="1"
        :pattern-color="theme === 'light' ? '#dbe3ef' : '#2d2e3d'"
        :bg-color="theme === 'light' ? '#f6f8fc' : '#1a1b23'"
      />
      <Controls position="bottom-right" class="vf-controls" />
      <MiniMap
        position="bottom-left"
        :node-color="theme === 'light' ? '#ffffff' : '#252633'"
        :node-stroke-color="theme === 'light' ? '#94a3b8' : '#3a3b4e'"
        :mask-color="theme === 'light' ? 'rgba(226,232,240,0.72)' : 'rgba(26,27,35,0.7)'"
        class="vf-minimap"
      />
    </VueFlow>

    <div class="canvas-toolbar" aria-label="画布工具">
      <div class="canvas-toolbar-title">
        <span class="canvas-toolbar-kicker">WORKFLOW CANVAS</span>
        <span class="canvas-toolbar-summary">{{ nodeCount }} 个节点 · {{ edgeCount }} 条连接</span>
      </div>
      <div class="canvas-toolbar-actions">
        <button class="canvas-tool-btn" title="重新整理节点位置" @click="emit('layout')">
          <el-icon :size="14"><Rank /></el-icon>
          整理布局
        </button>
        <button class="canvas-tool-btn" title="让所有节点适配画布" @click="fitCanvas">
          <el-icon :size="14"><FullScreen /></el-icon>
          适应画布
        </button>
      </div>
    </div>

    <div v-if="isEmpty" class="canvas-empty">
      <div class="canvas-empty-icon">
        <el-icon :size="40"><Share /></el-icon>
      </div>
      <div class="canvas-empty-title">从左侧面板拖入节点</div>
      <div class="canvas-empty-desc">拖拽节点到画布，然后连接节点的 handle 构建流程</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, markRaw } from 'vue'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import {
  MarkerType,
  VueFlow,
  useVueFlow,
  type Connection,
  type Edge,
  type Node,
  type NodeMouseEvent,
} from '@vue-flow/core'
import { FullScreen, Rank, Share } from '@element-plus/icons-vue'
import CanvasNode from './CanvasNode.vue'
import type { NodeMetadata } from '@/types'

const emit = defineEmits<{
  connect: [source: string, sourceHandle: string, target: string]
  'nodes-delete': [ids: string[]]
  'edges-delete': [deletions: { sourceId: string; sourceHandle: string }[]]
  'node-click': [id: string]
  'pane-click': []
  'node-drop': [meta: NodeMetadata, position: { x: number; y: number }]
  layout: []
}>()

defineProps<{ theme: 'dark' | 'light' }>()

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeTypes = { 'canvas-node': markRaw(CanvasNode as any) }

const defaultEdgeOpts = { type: 'smoothstep', style: { strokeWidth: 2 } }

const {
  addNodes,
  removeNodes,
  addEdges,
  removeEdges,
  setNodes,
  setEdges,
  getNodes,
  getEdges,
  screenToFlowCoordinate,
  fitView,
} = useVueFlow('puppet-canvas')

const isEmpty = computed(() => getNodes.value.length === 0)
const nodeCount = computed(() => getNodes.value.length)
const edgeCount = computed(() => getEdges.value.length)

// ── VF event handlers ────────────────────────────────────────────

function handleConnect(connection: Connection) {
  if (!connection.source || !connection.target) return
  const isNext = connection.sourceHandle === 'next'
  const color = isNext ? '#2dd4bf' : '#f87171'

  // Remove any existing edge from the same source handle
  const existing = getEdges.value.filter(
    (e) => e.source === connection.source && e.sourceHandle === connection.sourceHandle,
  )
  if (existing.length) removeEdges(existing.map((e) => e.id))

  addEdges([
    {
      id: `${connection.source}--${connection.sourceHandle}--${connection.target}`,
      source: connection.source,
      sourceHandle: connection.sourceHandle ?? 'next',
      target: connection.target,
      type: 'smoothstep',
      style: { stroke: color, strokeWidth: 2 },
      markerEnd: { type: MarkerType.ArrowClosed, color, width: 18, height: 18 },
      animated: !isNext,
      label: isNext ? '成功' : '失败',
      labelStyle: { fontSize: 10, fill: color, fontWeight: 600 },
      labelBgStyle: {
        fill: 'var(--edge-label-bg, #252633)',
        stroke: 'var(--edge-label-border, #3a3b4e)',
        strokeWidth: 1,
        fillOpacity: 0.96,
      },
      labelBgPadding: [6, 4],
      labelBgBorderRadius: 5,
    },
  ])

  emit('connect', connection.source, connection.sourceHandle ?? 'next', connection.target)
}

function onNodesDelete(deleted: Node[]) {
  emit('nodes-delete', deleted.map((n) => n.id))
}

function onEdgesDelete(deleted: Edge[]) {
  emit(
    'edges-delete',
    deleted.map((e) => ({ sourceId: e.source, sourceHandle: e.sourceHandle ?? 'next' })),
  )
}

function onNodeClick({ node }: NodeMouseEvent) {
  emit('node-click', node.id)
}

function fitCanvas() {
  void fitView({ padding: 0.2, duration: 240 })
}

// ── Drag & drop from palette ─────────────────────────────────────

function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = e.dataTransfer.types.includes('application/puppet-node') ? 'copy' : 'none'
  }
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  const raw = e.dataTransfer?.getData('application/puppet-node')
  if (!raw) return
  const meta = JSON.parse(raw) as NodeMetadata
  const position = screenToFlowCoordinate({ x: e.clientX, y: e.clientY })
  emit('node-drop', meta, position)
}

// ── Imperative API exposed to parent ─────────────────────────────

function initCanvas(nodes: Node[], edges: Edge[]) {
  setNodes(nodes)
  setEdges(edges)
  setTimeout(() => fitView({ padding: 0.2 }), 50)
}

function addVFNode(node: Node) { addNodes([node]) }
function removeVFNode(id: string) { removeNodes([id]) }
function addVFEdge(edge: Edge) { addEdges([edge]) }
function removeVFEdges(ids: string[]) { removeEdges(ids) }
function getCurrentNodes() { return getNodes.value }
function getCurrentEdges() { return getEdges.value }

defineExpose({ initCanvas, addVFNode, removeVFNode, addVFEdge, removeVFEdges, getCurrentNodes, getCurrentEdges })
</script>

<style>
@import '@vue-flow/core/dist/style.css';
</style>

<style scoped>
.canvas-wrap {
  flex: 1;
  position: relative;
  overflow: hidden;
  background: #1a1b23;
}

.pipeline-flow { width: 100%; height: 100%; }

.canvas-toolbar {
  position: absolute;
  top: 14px;
  left: 16px;
  right: 16px;
  z-index: 5;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  pointer-events: none;
}

.canvas-toolbar-title,
.canvas-toolbar-actions {
  pointer-events: auto;
  display: flex;
  align-items: center;
}

.canvas-toolbar-title {
  gap: 10px;
  padding: 8px 12px;
  border: 1px solid rgba(58, 59, 78, 0.76);
  border-radius: 10px;
  background: rgba(30, 31, 46, 0.86);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.12);
  backdrop-filter: blur(10px);
}

.canvas-toolbar-kicker {
  color: #2dd4bf;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.1em;
}

.canvas-toolbar-summary {
  color: #94a3b8;
  font-size: 11px;
}

.canvas-toolbar-actions {
  gap: 6px;
  padding: 4px;
  border: 1px solid rgba(58, 59, 78, 0.76);
  border-radius: 10px;
  background: rgba(30, 31, 46, 0.86);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.12);
  backdrop-filter: blur(10px);
}

.canvas-tool-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 9px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #c4cad4;
  cursor: pointer;
  font-size: 11px;
  font-weight: 700;
  transition: background 0.15s, color 0.15s;
}

.canvas-tool-btn:hover {
  background: #2d2e3d;
  color: #e2e8f0;
}

:deep(.vf-controls) {
  background: #1e1f2e;
  border: 1px solid #2d2e3d;
  border-radius: 8px;
  overflow: hidden;
}

:deep(.vf-controls button) {
  background: transparent;
  color: #94a3b8;
  border-bottom: 1px solid #2d2e3d;
}

:deep(.vf-controls button:hover) { background: #252633; color: #e2e8f0; }
:deep(.vf-controls button:last-child) { border-bottom: none; }

:deep(.vf-minimap) {
  border: 1px solid #2d2e3d;
  border-radius: 8px;
  background: #1e1f2e;
}

:deep(.vue-flow__edge-path) { stroke-width: 2; }

.canvas-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  pointer-events: none;
}

.canvas-empty-icon { color: #2d2e3d; }
.canvas-empty-title { font-size: 16px; font-weight: 600; color: #3a3b4e; }
.canvas-empty-desc  { font-size: 13px; color: #2d2e3d; text-align: center; max-width: 280px; }

@media (max-width: 720px) {
  .canvas-toolbar { top: 10px; left: 10px; right: 10px; }
  .canvas-toolbar-title { display: none; }
  .canvas-toolbar-actions { margin-left: auto; }
  .canvas-tool-btn { padding: 7px 8px; }
}
</style>
