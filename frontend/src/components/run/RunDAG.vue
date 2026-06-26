<template>
  <div class="run-dag">
    <VueFlow
      :nodes="flowNodes"
      :edges="flowEdges"
      :node-types="nodeTypes"
      fit-view-on-init
      :nodes-draggable="false"
      :nodes-connectable="false"
      :elements-selectable="true"
      class="run-flow"
      @node-click="onNodeClick"
    >
      <Background :gap="20" :size="1" pattern-color="#2d2e3d" bg-color="#1a1b23" />
      <Controls position="bottom-right" class="vf-controls" />
    </VueFlow>
  </div>
</template>

<script setup lang="ts">
import { computed, markRaw } from 'vue'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MarkerType, VueFlow, type Edge, type Node, type NodeMouseEvent } from '@vue-flow/core'
import dagre from '@dagrejs/dagre'
import CanvasNode from '@/components/canvas/CanvasNode.vue'
import type { NodeRun, PipelineDefinition, Status } from '@/types'

const props = defineProps<{
  pipeline: PipelineDefinition
  nodeRuns: NodeRun[]
  selectedNodeRunId?: number | null
}>()

const emit = defineEmits<{ 'node-click': [nodeId: string] }>()

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeTypes = { 'canvas-node': markRaw(CanvasNode as any) }

const NODE_W = 210
const NODE_H = 72

function computeLayout() {
  const nodes = props.pipeline.nodes
  const positions = new Map<string, { x: number; y: number }>()
  if (!nodes.length) return positions

  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: 'TB', ranksep: 100, nodesep: 80 })
  g.setDefaultEdgeLabel(() => ({}))
  const idSet = new Set(nodes.map((n) => n.id))
  for (const n of nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H })
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

const flowNodes = computed<Node[]>(() => {
  const positions = computeLayout()
  const runMap = new Map(props.nodeRuns.map((r) => [r.nodeId, r]))

  return props.pipeline.nodes.map((node, i) => {
    const run = runMap.get(node.id)
    const isSelected = run && props.selectedNodeRunId === run.id
    return {
      id: node.id,
      type: 'canvas-node',
      position: node.position ?? positions.get(node.id) ?? { x: i * 260, y: 0 },
      selected: !!isSelected,
      data: {
        pipelineNode: node,
        category: 'process',
        status: (run?.status ?? 'pending') as Status,
        durationMs: run?.durationMs,
      },
    }
  })
})

const flowEdges = computed<Edge[]>(() => {
  const idSet = new Set(props.pipeline.nodes.map((n) => n.id))
  const edges: Edge[] = []
  for (const node of props.pipeline.nodes) {
    if (node.nextNodeId && idSet.has(node.nextNodeId)) {
      edges.push({
        id: `${node.id}--next--${node.nextNodeId}`,
        source: node.id,
        sourceHandle: 'next',
        target: node.nextNodeId,
        type: 'smoothstep',
        style: { stroke: '#2dd4bf', strokeWidth: 2 },
        markerEnd: { type: MarkerType.ArrowClosed, color: '#2dd4bf', width: 18, height: 18 },
      })
    }
    if (node.fallbackNodeId && idSet.has(node.fallbackNodeId)) {
      edges.push({
        id: `${node.id}--fallback--${node.fallbackNodeId}`,
        source: node.id,
        sourceHandle: 'fallback',
        target: node.fallbackNodeId,
        type: 'smoothstep',
        animated: true,
        style: { stroke: '#f87171', strokeWidth: 2 },
        markerEnd: { type: MarkerType.ArrowClosed, color: '#f87171', width: 18, height: 18 },
      })
    }
  }
  return edges
})

function onNodeClick({ node }: NodeMouseEvent) {
  emit('node-click', node.id)
}
</script>

<style>
@import '@vue-flow/core/dist/style.css';
</style>

<style scoped>
.run-dag {
  width: 100%;
  height: 100%;
  background: #1a1b23;
}

.run-flow { width: 100%; height: 100%; }

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
</style>
