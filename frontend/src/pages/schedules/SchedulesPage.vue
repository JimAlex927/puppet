<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">定时任务</h2>
        <p class="muted">统一管理所有 Task 的 cron 调度规则</p>
      </div>
      <el-space>
        <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建定时</el-button>
      </el-space>
    </div>

    <div class="panel">
      <el-table v-loading="loading" :data="schedules" empty-text="暂无定时任务">
        <el-table-column prop="name" label="名称" min-width="180" show-overflow-tooltip />
        <el-table-column label="目标 Task" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="target-main">{{ row.taskName || `#${row.taskId}` }}</div>
            <div class="target-sub">{{ row.projectName || `Project #${row.projectId}` }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="cronExpression" label="Cron" width="150" />
        <el-table-column prop="cronTimezone" label="时区" width="150" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="下次执行" width="180">
          <template #default="{ row }">{{ fmtDate(row.nextRunAt) }}</template>
        </el-table-column>
        <el-table-column label="上次执行" width="180">
          <template #default="{ row }">{{ fmtDate(row.lastRunAt) }}</template>
        </el-table-column>
        <el-table-column label="上次结果" width="110">
          <template #default="{ row }">
            <StatusBadge v-if="row.lastStatus" :status="row.lastStatus" size="small" />
            <span v-else class="muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link :icon="VideoPlay" @click="runNow(row)">立即执行</el-button>
            <el-button size="small" link :icon="Edit" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" link @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
            <el-button size="small" link type="danger" :icon="Delete" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑定时任务' : '新建定时任务'" width="560px" @closed="resetForm">
      <el-form label-position="top">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如：每天构建" />
        </el-form-item>
        <el-form-item label="Project" required>
          <el-select v-model="form.projectId" filterable style="width: 100%" @change="onProjectChange">
            <el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Task" required>
          <el-select v-model="form.taskId" filterable style="width: 100%" :disabled="!form.projectId">
            <el-option v-for="task in projectTasks" :key="task.id" :label="task.name" :value="task.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Cron 表达式" required>
          <el-input v-model="form.cronExpression" placeholder="例如：*/5 * * * *" />
          <div class="muted field-tip">Linux cron 五段格式：分 时 日 月 周</div>
        </el-form-item>
        <el-form-item label="时区">
          <el-select v-model="form.cronTimezone" filterable allow-create style="width: 100%">
            <el-option label="系统本地时区 (Local)" value="Local" />
            <el-option label="Asia/Shanghai" value="Asia/Shanghai" />
            <el-option label="UTC" value="UTC" />
          </el-select>
        </el-form-item>
        <el-form-item label="运行输入 JSON">
          <el-input v-model="form.inputJson" type="textarea" :rows="5" placeholder='例如：{"branch":"main"}' />
          <div class="muted field-tip">留空时使用 Task 运行参数默认值；必填参数没有默认值时不会自动执行。</div>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Delete, Edit, Plus, Refresh, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import { api } from '@/api'
import StatusBadge from '@/components/StatusBadge.vue'
import type { Project, Task, TaskSchedule, TaskScheduleInput } from '@/types'
import { fmtDate } from '@/utils/format'

const router = useRouter()
const schedules = ref<TaskSchedule[]>([])
const projects = ref<Project[]>([])
const projectTasks = ref<Task[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)

const form = reactive<TaskScheduleInput & { id: number }>({
  id: 0,
  projectId: 0,
  taskId: 0,
  name: '',
  cronExpression: '',
  cronTimezone: 'Local',
  enabled: true,
  inputJson: '',
})

async function load() {
  loading.value = true
  try {
    const [scheduleItems, projectItems] = await Promise.all([
      api.schedules(),
      api.projects(),
    ])
    schedules.value = scheduleItems
    projects.value = projectItems
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, {
    id: 0,
    projectId: 0,
    taskId: 0,
    name: '',
    cronExpression: '',
    cronTimezone: 'Local',
    enabled: true,
    inputJson: '',
  })
  projectTasks.value = []
}

async function openCreate() {
  resetForm()
  dialogVisible.value = true
}

async function openEdit(schedule: TaskSchedule) {
  Object.assign(form, {
    id: schedule.id,
    projectId: schedule.projectId,
    taskId: schedule.taskId,
    name: schedule.name,
    cronExpression: schedule.cronExpression,
    cronTimezone: schedule.cronTimezone || 'Local',
    enabled: schedule.enabled,
    inputJson: schedule.inputJson || '',
  })
  await loadProjectTasks(schedule.projectId)
  dialogVisible.value = true
}

async function onProjectChange() {
  form.taskId = 0
  await loadProjectTasks(form.projectId)
}

async function loadProjectTasks(projectId: number) {
  projectTasks.value = projectId ? await api.tasks(projectId) : []
}

function payload(): TaskScheduleInput {
  return {
    projectId: form.projectId,
    taskId: form.taskId,
    name: form.name.trim(),
    cronExpression: form.cronExpression.trim(),
    cronTimezone: form.cronTimezone || 'Local',
    enabled: form.enabled,
    inputJson: form.inputJson.trim(),
  }
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入名称')
  if (!form.projectId || !form.taskId) return ElMessage.warning('请选择 Project 和 Task')
  if (!form.cronExpression.trim()) return ElMessage.warning('请输入 Cron 表达式')
  if (form.inputJson.trim()) {
    try {
      JSON.parse(form.inputJson)
    } catch {
      return ElMessage.warning('运行输入 JSON 格式不正确')
    }
  }
  saving.value = true
  try {
    if (form.id) {
      await api.updateSchedule(form.id, payload())
      ElMessage.success('已保存')
    } else {
      await api.createSchedule(payload())
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function toggle(schedule: TaskSchedule) {
  await api.updateSchedule(schedule.id, {
    projectId: schedule.projectId,
    taskId: schedule.taskId,
    name: schedule.name,
    cronExpression: schedule.cronExpression,
    cronTimezone: schedule.cronTimezone || 'Local',
    enabled: !schedule.enabled,
    inputJson: schedule.inputJson || '',
  })
  ElMessage.success(schedule.enabled ? '已停用' : '已启用')
  await load()
}

async function runNow(schedule: TaskSchedule) {
  const run = await api.runScheduleNow(schedule.id)
  ElMessage.success('已触发执行')
  router.push(`/runs/${run.id}`)
}

async function remove(schedule: TaskSchedule) {
  await ElMessageBox.confirm(`确认删除「${schedule.name}」？`, '删除定时任务', { type: 'warning' })
  await api.deleteSchedule(schedule.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.target-main {
  font-weight: 600;
  color: #1f2328;
}

.target-sub {
  margin-top: 2px;
  font-size: 12px;
  color: #64748b;
}

.field-tip {
  margin-top: 4px;
  font-size: 12px;
}
</style>
