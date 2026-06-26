<template>
  <div class="run-page">
    <!-- ── Top bar ───────────────────────────────────────── -->
    <header class="run-bar">
      <div class="run-bar-left">
        <button class="run-back-btn" @click="$router.back()">
          <el-icon :size="14"><Back /></el-icon>
        </button>
        <div class="run-title">
          <span class="run-id">Run #{{ run?.id }}</span>
          <StatusBadge v-if="run" :status="run.status" />
          <span v-if="run?.errorMessage" class="run-error">{{ run.errorMessage }}</span>
        </div>
      </div>
      <el-space>
        <el-button size="small" :icon="Refresh" @click="load">刷新</el-button>
        <el-button v-if="run && canCancel(run.status)" size="small" type="danger" @click="cancelRun">
          取消
        </el-button>
      </el-space>
    </header>

    <!-- ── Meta strip ────────────────────────────────────── -->
    <div v-if="run" class="run-meta">
      <div class="run-meta-item"><span class="rmi-label">触发者</span><span>{{ run.triggeredBy || '—' }}</span></div>
      <div class="run-meta-item"><span class="rmi-label">方式</span><span>{{ run.triggerType || '—' }}</span></div>
      <div class="run-meta-item"><span class="rmi-label">开始</span><span>{{ fmtDate(run.startedAt) }}</span></div>
      <div class="run-meta-item"><span class="rmi-label">耗时</span><span>{{ fmtDuration(run.durationMs) }}</span></div>
      <div v-if="inputEntries.length" class="run-meta-item">
        <span class="rmi-label">参数</span>
        <span>
          <span v-for="([k, v], i) in inputEntries" :key="k">
            <strong>{{ k }}</strong>={{ v }}<span v-if="i < inputEntries.length - 1"> · </span>
          </span>
        </span>
      </div>
    </div>

    <!-- ── Main: DAG + Log ────────────────────────────────── -->
    <div v-if="run" class="run-body">
      <!-- Left: DAG -->
      <div class="run-dag-panel">
        <RunDAG
          v-if="pipelineSnapshot"
          :pipeline="pipelineSnapshot"
          :node-runs="nodeRuns"
          :selected-node-run-id="selectedNodeRunId"
          @node-click="selectByNodeId"
        />
      </div>

      <!-- Right: Node list + Logs -->
      <div class="run-right">
        <!-- Node list -->
        <div class="run-node-list">
          <div
            class="rn-item"
            :class="{ 'rn-item--active': selectedNodeRunId === null }"
            @click="selectedNodeRunId = null"
          >
            <div class="rn-name">全部日志</div>
            <el-tag size="small" type="info" effect="dark">{{ logs.length }}</el-tag>
          </div>
          <div
            v-for="nr in nodeRuns"
            :key="nr.id"
            class="rn-item"
            :class="{ 'rn-item--active': selectedNodeRunId === nr.id }"
            @click="selectedNodeRunId = nr.id"
          >
            <div>
              <div class="rn-name">{{ nr.nodeIndex + 1 }}. {{ nr.nodeName }}</div>
              <div class="rn-sub">{{ nr.nodeType }} · {{ fmtDuration(nr.durationMs) }}</div>
              <div v-if="nr.errorMessage" class="rn-err">{{ nr.errorMessage }}</div>
            </div>
            <StatusBadge :status="nr.status" />
          </div>
        </div>

        <!-- Log viewer -->
        <div ref="logViewer" class="run-log">
          <div
            v-for="log in filteredLogs"
            :key="log.id ?? `${log.sequence}-${log.content}`"
            :class="logClass(log)"
          >{{ logPrefix(log) }}{{ log.content }}</div>
          <div v-if="!filteredLogs.length" class="run-log-empty">暂无日志</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Back, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import RunDAG from '@/components/run/RunDAG.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import type { NodeRun, PipelineDefinition, RunLog, TaskRun } from '@/types'
import { fmtDate, fmtDuration } from '@/utils/format'

const route = useRoute()
const runId = Number(route.params.id)

const run = ref<TaskRun>()
const nodeRuns = ref<NodeRun[]>([])
const logs = ref<RunLog[]>([])
const selectedNodeRunId = ref<number | null>(null)
const logViewer = ref<HTMLElement>()
let sse: EventSource | undefined
let transientId = -1

const pipelineSnapshot = computed<PipelineDefinition | null>(() => {
  if (!run.value?.pipelineSnapshotJson) return null
  try { return JSON.parse(run.value.pipelineSnapshotJson) } catch { return null }
})

const inputEntries = computed<[string, unknown][]>(() => {
  if (!run.value?.inputJson) return []
  try { return Object.entries(JSON.parse(run.value.inputJson)) } catch { return [] }
})

const filteredLogs = computed(() => {
  if (selectedNodeRunId.value === null) return logs.value
  return logs.value.filter((l) => l.nodeRunId === selectedNodeRunId.value || l.nodeRunId === 0)
})

function selectByNodeId(nodeId: string) {
  const nr = nodeRuns.value.find((r) => r.nodeId === nodeId)
  selectedNodeRunId.value = nr?.id ?? null
}

function canCancel(status: string) { return status === 'pending' || status === 'running' }

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

async function cancelRun() {
  await ElMessageBox.confirm('确认取消这个执行记录？')
  run.value = await api.cancelTaskRun(runId)
  ElMessage.success('已取消')
  await load()
}

function connect() {
  sse?.close()
  const token = encodeURIComponent(localStorage.getItem('puppet_token') || '')
  sse = new EventSource(`/api/task-runs/${runId}/events?token=${token}`)
  sse.addEventListener('log', (e) => {
    const d = JSON.parse(e.data)
    logs.value.push({ id: transientId--, taskRunId: d.taskRunId, nodeRunId: d.nodeRunId, sequence: d.sequence, stream: d.stream, content: d.content, createdAt: new Date().toISOString() })
  })
  sse.addEventListener('node_status', (e) => {
    const d = JSON.parse(e.data)
    if (!d.nodeRun) return
    const idx = nodeRuns.value.findIndex((r) => r.id === d.nodeRun.id)
    if (idx >= 0) nodeRuns.value[idx] = d.nodeRun
    else nodeRuns.value.push(d.nodeRun)
    nodeRuns.value.sort((a, b) => a.nodeIndex - b.nodeIndex)
  })
  sse.addEventListener('task_status', (e) => {
    const d = JSON.parse(e.data)
    if (d.run) run.value = d.run
    else if (run.value) run.value.status = d.status
  })
}

// Auto-scroll logs
watch(() => logs.value.length, async () => {
  await nextTick()
  if (logViewer.value) logViewer.value.scrollTop = logViewer.value.scrollHeight
})

function logPrefix(log: RunLog) {
  return `[${String(log.sequence).padStart(4, '0')}] ${log.stream.padEnd(6)} `
}

function logClass(log: RunLog) {
  const c = log.content.toLowerCase()
  const classes = [`log-${log.stream}`]
  if (c.includes('[node:failed]') || c.includes('status=failed') || c.includes(' error=') || c.includes(' failed:')) classes.push('log-failed')
  else if (c.includes('[node:start]') || c.includes('[node:end]') || c.includes('status=success') || c.includes(' succeeded')) classes.push('log-success')
  return classes
}

onMounted(async () => { await load(); connect() })
onBeforeUnmount(() => sse?.close())
</script>

<style scoped>
.run-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 72px);
  margin: -22px;
  background: #1a1b23;
}

/* Top bar */
.run-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  height: 52px;
  background: #1e1f2e;
  border-bottom: 1px solid #2d2e3d;
  flex-shrink: 0;
  gap: 12px;
}

.run-bar-left { display: flex; align-items: center; gap: 10px; }

.run-back-btn {
  background: #252633;
  border: 1px solid #3a3b4e;
  color: #94a3b8;
  cursor: pointer;
  width: 30px; height: 30px;
  border-radius: 6px;
  display: grid; place-items: center;
  transition: color 0.15s, background 0.15s;
}
.run-back-btn:hover { background: #2d2e3d; color: #e2e8f0; }

.run-title { display: flex; align-items: center; gap: 8px; }
.run-id { font-weight: 700; color: #e2e8f0; font-size: 15px; }
.run-error { font-size: 12px; color: #f87171; }

:deep(.run-bar .el-button) {
  background: #252633; border-color: #3a3b4e; color: #c4cad4;
}
:deep(.run-bar .el-button:hover) { background: #2d2e3d; color: #e2e8f0; }
:deep(.run-bar .el-button--danger) { background: #7f1d1d !important; border-color: #991b1b !important; color: #fca5a5 !important; }

/* Meta strip */
.run-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0;
  background: #1e1f2e;
  border-bottom: 1px solid #2d2e3d;
  flex-shrink: 0;
}

.run-meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 16px;
  font-size: 12px;
  color: #c4cad4;
  border-right: 1px solid #2d2e3d;
}

.rmi-label { color: #64748b; }

/* Main body */
.run-body {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 360px;
  overflow: hidden;
}

/* DAG panel */
.run-dag-panel {
  border-right: 1px solid #2d2e3d;
  overflow: hidden;
}

/* Right side */
.run-right {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #1a1b23;
}

/* Node list */
.run-node-list {
  flex-shrink: 0;
  max-height: 40%;
  overflow-y: auto;
  border-bottom: 1px solid #2d2e3d;
}

.run-node-list::-webkit-scrollbar { width: 4px; }
.run-node-list::-webkit-scrollbar-thumb { background: #2d2e3d; border-radius: 2px; }

.rn-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 9px 14px;
  cursor: pointer;
  border-bottom: 1px solid #1e1f2e;
  transition: background 0.1s;
}

.rn-item:hover { background: #1e1f2e; }
.rn-item--active { background: #252633; border-left: 2px solid #2dd4bf; }

.rn-name { font-size: 12px; font-weight: 600; color: #c4cad4; }
.rn-sub  { font-size: 11px; color: #64748b; margin-top: 2px; }
.rn-err  { font-size: 11px; color: #f87171; margin-top: 2px; }

/* Log viewer */
.run-log {
  flex: 1;
  overflow: auto;
  background: #0c1220;
  color: #dbeafe;
  font-family: 'Cascadia Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  padding: 12px;
  white-space: pre-wrap;
}

.run-log::-webkit-scrollbar { width: 4px; }
.run-log::-webkit-scrollbar-thumb { background: #1e2a3d; border-radius: 2px; }

.run-log-empty { color: #2d3748; text-align: center; padding: 24px; }

:global(.log-stderr)  { color: #fecaca; }
:global(.log-system)  { color: #fde68a; }
:global(.log-success) { color: #86efac; font-weight: 700; }
:global(.log-failed)  { color: #fca5a5; font-weight: 700; }
</style>
