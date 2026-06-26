<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">{{ task?.name }}</h2>
        <p class="muted">执行记录</p>
      </div>
      <el-space>
        <el-button :icon="Back" @click="$router.back()">返回</el-button>
        <el-button type="primary" :icon="VideoPlay" @click="runDialog?.open(taskId)">执行</el-button>
      </el-space>
    </div>
    <div class="panel">
      <el-table :data="runs" empty-text="暂无执行记录">
        <el-table-column prop="id" label="Run ID" width="90" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <StatusBadge :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column prop="triggeredBy" label="触发者" width="120" show-overflow-tooltip />
        <el-table-column label="参数" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="muted">{{ inputSummary(row.inputJson) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="开始时间" width="170">
          <template #default="{ row }">{{ fmtDate(row.startedAt) }}</template>
        </el-table-column>
        <el-table-column label="耗时" width="100">
          <template #default="{ row }">{{ fmtDuration(row.durationMs) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="$router.push(`/runs/${row.id}`)">详情</el-button>
            <el-button
              v-if="canCancel(row.status)"
              link
              type="danger"
              @click="cancelRun(row.id)"
            >
              取消
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <RunTaskDialog ref="runDialog" @success="onRunSuccess" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Back, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import RunTaskDialog from '@/components/RunTaskDialog.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import type { Task, TaskRun } from '@/types'
import { fmtDate, fmtDuration } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const taskId = Number(route.params.id)
const task = ref<Task>()
const runs = ref<TaskRun[]>([])
const runDialog = ref<InstanceType<typeof RunTaskDialog>>()

async function load() {
  task.value = await api.task(taskId)
  runs.value = await api.taskRuns(taskId)
}

function onRunSuccess(run: TaskRun) {
  router.push(`/runs/${run.id}`)
}

function canCancel(status: string) {
  return status === 'pending' || status === 'running'
}

async function cancelRun(runId: number) {
  await ElMessageBox.confirm('确认取消这个执行记录？')
  await api.cancelTaskRun(runId)
  ElMessage.success('已取消')
  await load()
}

function inputSummary(inputJson: string): string {
  if (!inputJson) return '—'
  try {
    const obj = JSON.parse(inputJson)
    const entries = Object.entries(obj)
    if (entries.length === 0) return '—'
    return entries.map(([k, v]) => `${k}=${v}`).join(', ')
  } catch {
    return '—'
  }
}

onMounted(load)
</script>
