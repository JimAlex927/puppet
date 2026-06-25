<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">Run #{{ run?.id }}</h2>
        <p class="muted" style="margin: 4px 0 0">
          <StatusBadge v-if="run" :status="run.status" />
          <span v-if="run?.errorMessage" style="margin-left: 8px; color: #ef4444">{{ run.errorMessage }}</span>
        </p>
      </div>
      <el-button :icon="Refresh" @click="load">刷新</el-button>
    </div>

    <div v-if="run" class="run-meta-grid">
      <div class="run-meta-item">
        <div class="run-meta-label">触发者</div>
        <div class="run-meta-value">{{ run.triggeredBy || '—' }}</div>
      </div>
      <div class="run-meta-item">
        <div class="run-meta-label">触发方式</div>
        <div class="run-meta-value">{{ run.triggerType || '—' }}</div>
      </div>
      <div class="run-meta-item">
        <div class="run-meta-label">开始时间</div>
        <div class="run-meta-value">{{ fmtDate(run.startedAt) }}</div>
      </div>
      <div class="run-meta-item">
        <div class="run-meta-label">耗时</div>
        <div class="run-meta-value">{{ fmtDuration(run.durationMs) }}</div>
      </div>
      <div v-if="runInputEntries.length > 0" class="run-meta-item" style="grid-column: span 2">
        <div class="run-meta-label">运行参数</div>
        <div class="run-meta-value" style="font-size: 13px; font-weight: 400">
          <span v-for="([k, v], i) in runInputEntries" :key="k">
            <strong>{{ k }}</strong>={{ v }}<span v-if="i < runInputEntries.length - 1">&nbsp;&nbsp;</span>
          </span>
        </div>
      </div>
    </div>

    <div class="run-detail">
      <div>
        <div
          class="node-run-item"
          :class="{ selected: selectedNodeRunId === null }"
          @click="selectedNodeRunId = null"
        >
          <div class="node-title">
            <strong>全部日志</strong>
            <el-tag size="small" type="info">{{ logs.length }} 行</el-tag>
          </div>
        </div>
        <div
          v-for="node in nodeRuns"
          :key="node.id"
          class="node-run-item"
          :class="{ selected: selectedNodeRunId === node.id }"
          @click="selectedNodeRunId = node.id"
        >
          <div class="node-title">
            <strong>{{ node.nodeIndex + 1 }}. {{ node.nodeName }}</strong>
            <StatusBadge :status="node.status" />
          </div>
          <div class="muted" style="margin-top: 4px">{{ node.nodeType }} · {{ fmtDuration(node.durationMs) }}</div>
          <div v-if="node.errorMessage" class="muted" style="color: #ef4444; margin-top: 2px">{{ node.errorMessage }}</div>
        </div>
        <el-empty v-if="nodeRuns.length === 0" description="等待节点启动" />
      </div>
      <div>
        <RunLogViewer :logs="filteredLogs" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { api } from '@/api'
import RunLogViewer from '@/components/RunLogViewer.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import type { NodeRun, RunLog, TaskRun } from '@/types'
import { fmtDate, fmtDuration } from '@/utils/format'

const route = useRoute()
const runId = Number(route.params.id)
const run = ref<TaskRun>()
const nodeRuns = ref<NodeRun[]>([])
const logs = ref<RunLog[]>([])
const selectedNodeRunId = ref<number | null>(null)
let source: EventSource | undefined
let transientLogId = -1

const runInputEntries = computed<[string, unknown][]>(() => {
  if (!run.value?.inputJson) return []
  try {
    const obj = JSON.parse(run.value.inputJson)
    return Object.entries(obj)
  } catch {
    return []
  }
})

const filteredLogs = computed(() => {
  if (selectedNodeRunId.value === null) return logs.value
  return logs.value.filter(
    (l) => l.nodeRunId === selectedNodeRunId.value || l.nodeRunId === 0,
  )
})

async function load() {
  const [runData, nodeData, logData] = await Promise.all([
    api.taskRun(runId),
    api.nodeRuns(runId),
    api.runLogs(runId),
  ])
  run.value = runData
  nodeRuns.value = nodeData
  logs.value = logData
}

function connect() {
  source?.close()
  const token = encodeURIComponent(localStorage.getItem('puppet_token') || '')
  source = new EventSource(`/api/task-runs/${runId}/events?token=${token}`)
  source.addEventListener('log', (event) => {
    const data = JSON.parse(event.data)
    logs.value.push({
      id: transientLogId--,
      taskRunId: data.taskRunId,
      nodeRunId: data.nodeRunId,
      sequence: data.sequence,
      stream: data.stream,
      content: data.content,
      createdAt: new Date().toISOString(),
    })
  })
  source.addEventListener('node_status', (event) => {
    const data = JSON.parse(event.data)
    if (!data.nodeRun) return
    const index = nodeRuns.value.findIndex((item) => item.id === data.nodeRun.id)
    if (index >= 0) nodeRuns.value[index] = data.nodeRun
    else nodeRuns.value.push(data.nodeRun)
    nodeRuns.value.sort((a, b) => a.nodeIndex - b.nodeIndex)
  })
  source.addEventListener('task_status', (event) => {
    const data = JSON.parse(event.data)
    if (data.run) run.value = data.run
    else if (run.value) run.value.status = data.status
  })
}

onMounted(async () => {
  await load()
  connect()
})

onBeforeUnmount(() => source?.close())
</script>
