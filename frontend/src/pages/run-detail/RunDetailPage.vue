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

    <div v-if="run" class="run-tabs">
      <button
        class="run-tab"
        :class="{ 'run-tab--active': activeTab === 'execution' }"
        @click="activeTab = 'execution'"
      >
        执行视图
      </button>
      <button
        class="run-tab"
        :class="{ 'run-tab--active': activeTab === 'files' }"
        @click="activeTab = 'files'"
      >
        运行文件
      </button>
      <button
        class="run-tab"
        :class="{ 'run-tab--active': activeTab === 'history' }"
        @click="activeTab = 'history'"
      >
        历史版本
      </button>
    </div>

    <!-- ── Main: DAG + Log ────────────────────────────────── -->
    <div v-if="run && activeTab === 'execution'" class="run-body">
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

        <!-- Context viewer -->
        <div class="run-context">
          <div class="rc-header">
            <span>{{ selectedNodeRun ? '节点上下文' : '运行上下文' }}</span>
            <span class="rc-sub">{{ selectedNodeRun ? selectedNodeRun.nodeId : `taskrun-${run.id}` }}</span>
          </div>

          <template v-if="selectedNodeRun">
            <div class="rc-grid">
              <div><span>节点</span><strong>{{ selectedNodeRun.nodeName }}</strong></div>
              <div><span>类型</span><strong>{{ selectedNodeRun.nodeType }}</strong></div>
              <div><span>状态</span><strong>{{ selectedNodeRun.status }}</strong></div>
              <div><span>重试</span><strong>{{ selectedNodeRun.retryCount }}</strong></div>
              <div><span>耗时</span><strong>{{ fmtDuration(selectedNodeRun.durationMs) }}</strong></div>
              <div><span>工作目录</span><strong>{{ workspaceHint }}</strong></div>
            </div>
            <div v-if="selectedNodeRun.errorMessage" class="rc-error">{{ selectedNodeRun.errorMessage }}</div>
            <div class="rc-block">
              <div class="rc-block-title">节点参数快照</div>
              <pre>{{ prettyJSON(selectedNodeParams) }}</pre>
            </div>
            <div class="rc-block">
              <div class="rc-block-title">展开后的参数</div>
              <pre>{{ prettyJSON(selectedNodeExpandedParams) }}</pre>
            </div>
            <div class="rc-block">
              <div class="rc-block-title">节点输出</div>
              <pre>{{ prettyJSON(selectedNodeOutput) }}</pre>
            </div>
          </template>

          <template v-else>
            <div class="rc-grid">
              <div><span>Run ID</span><strong>#{{ run.id }}</strong></div>
              <div><span>Task ID</span><strong>#{{ run.taskId }}</strong></div>
              <div><span>触发</span><strong>{{ run.triggerType || '—' }}</strong></div>
              <div><span>工作目录</span><strong>{{ workspaceHint }}</strong></div>
              <div><span>Agent</span><strong>{{ agentLabelsText }}</strong></div>
              <div><span>节点数</span><strong>{{ pipelineSnapshot?.nodes.length ?? 0 }}</strong></div>
            </div>
            <div class="rc-block">
              <div class="rc-block-title">运行参数</div>
              <pre>{{ prettyJSON(runInputObject) }}</pre>
            </div>
          </template>
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

    <div v-else-if="run && activeTab === 'files'" class="run-files">
      <div class="run-files-toolbar">
        <div class="rf-path-box">
          <span class="rf-label">目录</span>
          <span class="rf-path">{{ fileList?.path || '根目录' }}</span>
        </div>
        <el-space>
          <el-button
            size="small"
            :icon="ArrowUp"
            :disabled="!fileList?.path"
            @click="loadTaskRunFiles(fileList?.parent || '')"
          >
            上级
          </el-button>
          <el-button size="small" :icon="Refresh" :loading="fileLoading" @click="loadTaskRunFiles(filePath)">
            刷新
          </el-button>
          <el-button
            size="small"
            type="primary"
            :icon="Download"
            :disabled="!selectedFilePaths.length"
            @click="bundleSelectedFiles"
          >
            打包下载 {{ selectedFilePaths.length ? `(${selectedFilePaths.length})` : '' }}
          </el-button>
        </el-space>
      </div>

      <div v-if="fileBundles.length" class="rf-bundles">
        <div v-for="bundle in fileBundles" :key="bundle.id" class="rf-bundle">
          <div>
            <span class="rf-bundle-title">打包任务 {{ shortBundleId(bundle.id) }}</span>
            <span class="rf-bundle-message">{{ bundle.message || bundle.status }}</span>
          </div>
          <el-button
            v-if="bundle.status === 'ready' && bundle.downloadUrl"
            size="small"
            type="success"
            :icon="Download"
            @click="downloadBundle(bundle)"
          >
            下载
          </el-button>
          <el-tag v-else-if="bundle.status === 'failed'" size="small" type="danger">失败</el-tag>
          <el-tag v-else size="small" type="info">处理中</el-tag>
        </div>
      </div>

      <el-table
        class="run-files-table"
        :data="fileEntries"
        height="100%"
        v-loading="fileLoading"
        empty-text="这个运行目录暂无文件"
        @selection-change="handleFileSelectionChange"
      >
        <el-table-column type="selection" width="44" />
        <el-table-column label="名称" min-width="320">
          <template #default="{ row }">
            <button class="rf-name" @click="openFileEntry(row)">
              <el-icon :size="16">
                <Folder v-if="row.isDir" />
                <Document v-else />
              </el-icon>
              <span>{{ row.name }}</span>
            </button>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120">
          <template #default="{ row }">{{ row.isDir ? '文件夹' : '文件' }}</template>
        </el-table-column>
        <el-table-column label="大小" width="140">
          <template #default="{ row }">{{ row.isDir ? '—' : fmtFileSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="修改时间" width="190">
          <template #default="{ row }">{{ fmtDate(row.modTime) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="!row.isDir"
              size="small"
              text
              type="primary"
              :icon="Download"
              @click.stop="downloadFile(row)"
            >
              下载
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-else-if="run && activeTab === 'history'" class="run-history">
      <div class="rh-header">
        <div>
          <div class="rh-title">Run #{{ run.id }} Pipeline Snapshot</div>
          <div class="rh-sub">
            {{ run.triggeredBy || 'system' }} · {{ run.triggerType || 'manual' }} · {{ fmtDate(run.createdAt) }}
          </div>
        </div>
        <el-space>
          <StatusBadge :status="run.status" size="small" />
          <el-button size="small" :icon="VideoPlay" @click="runThisHistoryVersion">
            用此版本运行
          </el-button>
          <el-button
            size="small"
            type="primary"
            :icon="CopyDocument"
            :loading="creatingTaskFromHistory"
            @click="createTaskFromThisHistory"
          >
            生成新 Task
          </el-button>
        </el-space>
      </div>

      <div class="rh-summary">
        <div><span>Task ID</span><strong>#{{ run.taskId }}</strong></div>
        <div><span>Run ID</span><strong>#{{ run.id }}</strong></div>
        <div><span>节点数</span><strong>{{ pipelineSnapshot?.nodes.length ?? 0 }}</strong></div>
        <div><span>运行参数</span><strong>{{ pipelineSnapshot?.inputs.length ?? 0 }}</strong></div>
      </div>

      <pre class="rh-json">{{ prettyJSON(pipelineSnapshot ?? parseJSONRecord(run.pipelineSnapshotJson)) }}</pre>
    </div>

    <RunTaskDialog ref="runDialog" @success="onRunSuccess" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowUp, Back, CopyDocument, Document, Download, Folder, Refresh, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import RunDAG from '@/components/run/RunDAG.vue'
import RunTaskDialog from '@/components/RunTaskDialog.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import type { NodeRun, PipelineDefinition, RunLog, TaskRun, TaskRunFileBundle, TaskRunFileEntry, TaskRunFileList } from '@/types'
import { fmtDate, fmtDuration } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const runId = Number(route.params.id)

const run = ref<TaskRun>()
const nodeRuns = ref<NodeRun[]>([])
const logs = ref<RunLog[]>([])
const selectedNodeRunId = ref<number | null>(null)
const activeTab = ref<'execution' | 'files' | 'history'>('execution')
const fileList = ref<TaskRunFileList>()
const filePath = ref('')
const fileLoading = ref(false)
const selectedFilePaths = ref<string[]>([])
const fileBundles = ref<TaskRunFileBundle[]>([])
const creatingTaskFromHistory = ref(false)
const logViewer = ref<HTMLElement>()
const runDialog = ref<InstanceType<typeof RunTaskDialog>>()
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

const runInputObject = computed<Record<string, unknown>>(() => parseJSONRecord(run.value?.inputJson))

const selectedNodeRun = computed(() =>
  selectedNodeRunId.value === null
    ? undefined
    : nodeRuns.value.find((item) => item.id === selectedNodeRunId.value),
)

const selectedNodeParams = computed<Record<string, unknown>>(() =>
  parseJSONRecord(selectedNodeRun.value?.paramsSnapshotJson),
)

const selectedNodeExpandedParams = computed<Record<string, unknown>>(() =>
  expandRuntimePlaceholders(selectedNodeParams.value),
)

const selectedNodeOutput = computed<Record<string, unknown>>(() =>
  parseJSONRecord(selectedNodeRun.value?.outputJson),
)

const workspaceHint = computed(() => run.value ? `data/workspaces/taskrun-${run.value.id}` : '—')

const agentLabelsText = computed(() => {
  const labels = pipelineSnapshot.value?.agentSelector?.labels ?? []
  return labels.length ? labels.join(', ') : 'local'
})

const filteredLogs = computed(() => {
  if (selectedNodeRunId.value === null) return logs.value
  return logs.value.filter((l) => l.nodeRunId === selectedNodeRunId.value || l.nodeRunId === 0)
})

const fileEntries = computed(() => fileList.value?.entries ?? [])

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
  if (activeTab.value === 'files') await loadTaskRunFiles(filePath.value)
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
  sse.addEventListener('file_bundle', (e) => {
    const d = JSON.parse(e.data) as TaskRunFileBundle
    applyFileBundleEvent(d)
  })
}

async function loadTaskRunFiles(path = '') {
  fileLoading.value = true
  try {
    const data = await api.taskRunFiles(runId, path)
    fileList.value = data
    filePath.value = data.path
    selectedFilePaths.value = []
  } finally {
    fileLoading.value = false
  }
}

function openFileEntry(entry: TaskRunFileEntry) {
  if (entry.isDir) {
    void loadTaskRunFiles(entry.path)
    return
  }
  downloadFile(entry)
}

function downloadFile(entry: TaskRunFileEntry) {
  triggerDownload(api.taskRunFileDownloadUrl(runId, entry.path))
}

function handleFileSelectionChange(selection: TaskRunFileEntry[]) {
  selectedFilePaths.value = selection.map((entry) => entry.path)
}

async function bundleSelectedFiles() {
  if (!selectedFilePaths.value.length) return
  const bundle = await api.createTaskRunFileBundle(runId, selectedFilePaths.value)
  applyFileBundleEvent(bundle)
  ElMessage.info('已开始打包，完成后会显示下载按钮')
}

function applyFileBundleEvent(bundle: TaskRunFileBundle) {
  const idx = fileBundles.value.findIndex((item) => item.id === bundle.id)
  if (idx >= 0) fileBundles.value[idx] = { ...fileBundles.value[idx], ...bundle }
  else fileBundles.value.unshift(bundle)
  if (bundle.status === 'ready') ElMessage.success('运行文件打包完成')
  if (bundle.status === 'failed') ElMessage.error(bundle.message || '运行文件打包失败')
}

function downloadBundle(bundle: TaskRunFileBundle) {
  if (!bundle.downloadUrl) return
  triggerDownload(api.taskRunFileBundleDownloadUrl(bundle.downloadUrl))
}

function triggerDownload(url: string) {
  const link = document.createElement('a')
  link.href = url
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
}

function shortBundleId(id: string) {
  return id.length > 8 ? id.slice(0, 8) : id
}

function fmtFileSize(size: number) {
  if (!Number.isFinite(size) || size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  const digits = value >= 10 || unit === 0 ? 0 : 1
  return `${value.toFixed(digits)} ${units[unit]}`
}

function runThisHistoryVersion() {
  if (!run.value) return
  runDialog.value?.open(run.value.taskId, {
    pipelineVersionId: run.value.id,
    title: `运行任务 - Run #${run.value.id} 版本`,
  })
}

async function createTaskFromThisHistory() {
  if (!run.value) return
  const { value } = await ElMessageBox.prompt('请输入新 Task 名称', '生成新 Task', {
    inputValue: `Task copy from run #${run.value.id}`,
    inputPlaceholder: '新 Task 名称',
    inputValidator: (val) => Boolean(String(val || '').trim()) || '请填写 Task 名称',
    confirmButtonText: '创建',
    cancelButtonText: '取消',
  })
  creatingTaskFromHistory.value = true
  try {
    const created = await api.createTaskFromPipelineVersion(run.value.taskId, run.value.id, String(value).trim())
    ElMessage.success('已生成新 Task')
    router.push(`/tasks/${created.id}/pipeline`)
  } finally {
    creatingTaskFromHistory.value = false
  }
}

function onRunSuccess(nextRun: TaskRun) {
  router.push(`/runs/${nextRun.id}`)
}

// Auto-scroll logs
watch(() => logs.value.length, async () => {
  await nextTick()
  if (logViewer.value) logViewer.value.scrollTop = logViewer.value.scrollHeight
})

watch(activeTab, (tab) => {
  if (tab === 'files' && !fileList.value) void loadTaskRunFiles()
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

function parseJSONRecord(content?: string): Record<string, unknown> {
  if (!content) return {}
  try {
    const parsed = JSON.parse(content)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : { value: parsed }
  } catch {
    return { raw: content }
  }
}

function prettyJSON(value: unknown) {
  if (!value || (typeof value === 'object' && Object.keys(value as Record<string, unknown>).length === 0)) return '{}'
  return JSON.stringify(value, null, 2)
}

function expandRuntimePlaceholders(value: unknown): Record<string, unknown> {
  const expanded = expandValue(value)
  return expanded && typeof expanded === 'object' && !Array.isArray(expanded)
    ? expanded as Record<string, unknown>
    : { value: expanded }
}

function expandValue(value: unknown): unknown {
  if (typeof value === 'string') {
    return value.split('${workspace}').join(workspaceHint.value)
  }
  if (Array.isArray(value)) {
    return value.map((item) => expandValue(item))
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, item]) => [key, expandValue(item)]),
    )
  }
  return value
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

/* Tabs */
.run-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 40px;
  padding: 0 16px;
  background: #1e1f2e;
  border-bottom: 1px solid #2d2e3d;
  flex-shrink: 0;
}

.run-tab {
  height: 28px;
  padding: 0 14px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: #8892a4;
  cursor: pointer;
  font-size: 12px;
  font-weight: 700;
}

.run-tab:hover {
  color: #e2e8f0;
  background: #252633;
}

.run-tab--active {
  color: #2dd4bf;
  background: rgba(45, 212, 191, 0.1);
  border-color: rgba(45, 212, 191, 0.35);
}

/* Main body */
.run-body {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 360px;
  overflow: hidden;
}

.run-files {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 14px 16px;
  background: #1a1b23;
  overflow: hidden;
}

.run-files-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  margin-bottom: 12px;
}

.rf-path-box {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 12px;
  border: 1px solid #2d2e3d;
  border-radius: 6px;
  background: #111827;
}

.rf-label {
  color: #64748b;
  font-size: 12px;
}

.rf-path {
  color: #dbeafe;
  font-size: 12px;
  font-family: 'Cascadia Mono', Consolas, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rf-bundles {
  display: grid;
  gap: 8px;
  flex-shrink: 0;
  margin-bottom: 12px;
}

.rf-bundle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 40px;
  padding: 8px 10px;
  border: 1px solid #2d2e3d;
  border-radius: 6px;
  background: #111827;
}

.rf-bundle-title {
  display: block;
  color: #e2e8f0;
  font-size: 12px;
  font-weight: 700;
}

.rf-bundle-message {
  display: block;
  margin-top: 2px;
  color: #8892a4;
  font-size: 11px;
}

.run-files-table {
  flex: 1;
  min-height: 0;
  border: 1px solid #2d2e3d;
  border-radius: 6px;
  overflow: hidden;
  background: #111827;
}

.rf-name {
  max-width: 100%;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 0;
  padding: 0;
  background: transparent;
  color: #dbeafe;
  cursor: pointer;
  font: inherit;
  font-weight: 700;
}

.rf-name:hover {
  color: #2dd4bf;
}

.rf-name span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.run-files .el-button:not(.el-button--primary):not(.el-button--success)) {
  background: #252633;
  border-color: #3a3b4e;
  color: #c4cad4;
}

:deep(.run-files .el-button:not(.el-button--primary):not(.el-button--success):hover) {
  background: #2d2e3d;
  color: #e2e8f0;
}

:deep(.run-files-table.el-table) {
  --el-table-bg-color: #111827;
  --el-table-tr-bg-color: #111827;
  --el-table-header-bg-color: #1e1f2e;
  --el-table-header-text-color: #c4cad4;
  --el-table-text-color: #c4cad4;
  --el-table-border-color: #2d2e3d;
  --el-table-row-hover-bg-color: #1e1f2e;
  --el-fill-color-lighter: #1e1f2e;
  --el-text-color-regular: #c4cad4;
}

.run-history {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 14px 16px;
  background: #1a1b23;
  overflow: hidden;
}

.rh-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  margin-bottom: 12px;
  padding: 12px;
  border: 1px solid #2d2e3d;
  border-radius: 6px;
  background: #111827;
}

.rh-title {
  color: #e2e8f0;
  font-weight: 800;
  font-size: 14px;
}

.rh-sub {
  margin-top: 4px;
  color: #8892a4;
  font-size: 12px;
}

.rh-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  flex-shrink: 0;
  margin-bottom: 12px;
}

.rh-summary div {
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid #2d2e3d;
  border-radius: 6px;
  background: #111827;
}

.rh-summary span {
  display: block;
  color: #64748b;
  font-size: 11px;
  margin-bottom: 3px;
}

.rh-summary strong {
  display: block;
  color: #c4cad4;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rh-json {
  flex: 1;
  min-height: 0;
  margin: 0;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: #bfdbfe;
  background: #0c1220;
  border: 1px solid #1e2a3d;
  border-radius: 6px;
  padding: 12px;
  font-family: 'Cascadia Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
}

:deep(.run-history .el-button:not(.el-button--primary):not(.el-button--success)) {
  background: #252633;
  border-color: #3a3b4e;
  color: #c4cad4;
}

:deep(.run-history .el-button:not(.el-button--primary):not(.el-button--success):hover) {
  background: #2d2e3d;
  color: #e2e8f0;
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

/* Context viewer */
.run-context {
  flex-shrink: 0;
  max-height: 260px;
  overflow: auto;
  padding: 10px 12px;
  border-bottom: 1px solid #2d2e3d;
  background: #111827;
}

.run-context::-webkit-scrollbar { width: 4px; }
.run-context::-webkit-scrollbar-thumb { background: #2d2e3d; border-radius: 2px; }

.rc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  color: #e2e8f0;
  font-size: 12px;
  font-weight: 700;
}

.rc-sub {
  color: #64748b;
  font-family: 'Cascadia Mono', Consolas, monospace;
  font-size: 10px;
  font-weight: 500;
}

.rc-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  margin-bottom: 8px;
}

.rc-grid div {
  min-width: 0;
  background: #1a1b23;
  border: 1px solid #2d2e3d;
  border-radius: 6px;
  padding: 6px 7px;
}

.rc-grid span {
  display: block;
  color: #64748b;
  font-size: 10px;
  margin-bottom: 2px;
}

.rc-grid strong {
  display: block;
  color: #c4cad4;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rc-error {
  color: #fecaca;
  background: rgba(127, 29, 29, 0.35);
  border: 1px solid rgba(248, 113, 113, 0.35);
  border-radius: 6px;
  padding: 6px 8px;
  font-size: 11px;
  margin-bottom: 8px;
}

.rc-block + .rc-block { margin-top: 8px; }

.rc-block-title {
  color: #8892a4;
  font-size: 10px;
  font-weight: 700;
  margin-bottom: 4px;
}

.rc-block pre {
  margin: 0;
  max-height: 130px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: #bfdbfe;
  background: #0c1220;
  border: 1px solid #1e2a3d;
  border-radius: 6px;
  padding: 7px 8px;
  font-family: 'Cascadia Mono', Consolas, monospace;
  font-size: 11px;
  line-height: 1.45;
}

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
