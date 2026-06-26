<template>
  <div class="panel canvas-panel">
    <template v-if="pipeline.nodes.length">
      <VueFlow
        :nodes="flowNodes"
        :edges="flowEdges"
        :node-types="nodeTypes"
        fit-view-on-init
        class="vf-wrapper"
        @node-click="onNodeClick"
        @pane-click="emit('update:selectedNodeId', null)"
      >
        <Background pattern-color="#dce4ef" :gap="20" />
        <Controls />
        <Panel position="top-right" class="vf-hint">
          <span>点击节点选中配置 · 拖拽画布移动视图</span>
        </Panel>
      </VueFlow>
    </template>
    <el-empty v-else description="从左侧添加节点以在流程图中查看" style="height: 100%" />
  </div>
</template>

<script setup lang="ts">
import { computed, markRaw } from 'vue'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MarkerType, Panel, VueFlow, type Edge, type Node, type NodeMouseEvent } from '@vue-flow/core'
import dagre from '@dagrejs/dagre'
import PipelineNodeCard from './PipelineNodeCard.vue'
import type { PipelineDefinition } from '@/types'

const props = defineProps<{
  pipeline: PipelineDefinition
  selectedNodeId: string | null
}>()

const emit = defineEmits<{
  'update:selectedNodeId': [id: string | null]
}>()

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeTypes = { 'pipeline-node': markRaw(PipelineNodeCard) } as any

const NODE_W = 200
const NODE_H = 72

function buildPositions() {
  const nodes = props.pipeline.nodes
  const positions = new Map<string, { x: number; y: number }>()
  if (!nodes.length) return positions

  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: 'TB', ranksep: 90, nodesep: 70 })
  g.setDefaultEdgeLabel(() => ({}))

  for (const node of nodes) {
    g.setNode(node.id, { width: NODE_W, height: NODE_H })
  }
  for (const node of nodes) {
    if (node.nextNodeId && props.pipeline.nodes.find((n) => n.id === node.nextNodeId)) {
      g.setEdge(node.id, node.nextNodeId)
    }
    if (node.fallbackNodeId && props.pipeline.nodes.find((n) => n.id === node.fallbackNodeId)) {
      g.setEdge(node.id, node.fallbackNodeId)
    }
  }

  dagre.layout(g)

  for (const node of nodes) {
    const pos = g.node(node.id)
    if (pos) positions.set(node.id, { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 })
  }
  return positions
}

const flowNodes = computed<Node[]>(() => {
  const positions = buildPositions()
  return props.pipeline.nodes.map((node, index) => ({
    id: node.id,
    type: 'pipeline-node',
    position: positions.get(node.id) ?? { x: index * 260, y: 0 },
    data: { node, index },
    selected: node.id === props.selectedNodeId,
  }))
})

const flowEdges = computed<Edge[]>(() => {
  const edges: Edge[] = []
  const nodeIds = new Set(props.pipeline.nodes.map((n) => n.id))
  for (const node of props.pipeline.nodes) {
    if (node.nextNodeId && nodeIds.has(node.nextNodeId)) {
      edges.push({
        id: `${node.id}->next->${node.nextNodeId}`,
        source: node.id,
        sourceHandle: 'next',
        target: node.nextNodeId,
        type: 'smoothstep',
        style: { stroke: '#2dd4bf', strokeWidth: 2 },
        markerEnd: { type: MarkerType.ArrowClosed, color: '#2dd4bf', width: 20, height: 20 },
        label: '成功',
        labelStyle: { fontSize: 11, fill: '#0d9488', fontWeight: 600 },
        labelBgStyle: { fill: '#f0fdfb' },
      })
    }
    if (node.fallbackNodeId && nodeIds.has(node.fallbackNodeId)) {
      edges.push({
        id: `${node.id}->fallback->${node.fallbackNodeId}`,
        source: node.id,
        sourceHandle: 'fallback',
        target: node.fallbackNodeId,
        type: 'smoothstep',
        animated: true,
        style: { stroke: '#ef4444', strokeWidth: 2 },
        markerEnd: { type: MarkerType.ArrowClosed, color: '#ef4444', width: 20, height: 20 },
        label: '失败',
        labelStyle: { fontSize: 11, fill: '#ef4444', fontWeight: 600 },
        labelBgStyle: { fill: '#fef2f2' },
      })
    }
  }
  return edges
})

function onNodeClick({ node }: NodeMouseEvent) {
  emit('update:selectedNodeId', node.id)
}
</script>

<style>
@import '@vue-flow/core/dist/style.css';
</style>

<style scoped>
.canvas-panel {
  padding: 0;
  overflow: hidden;
  min-height: 400px;
}

.vf-wrapper {
  height: calc(100vh - 230px);
  border-radius: 8px;
}

.vf-hint {
  background: rgba(255, 255, 255, 0.85);
  border: 1px solid #dce4ef;
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 12px;
  color: #64748b;
  backdrop-filter: blur(4px);
}
</style>
