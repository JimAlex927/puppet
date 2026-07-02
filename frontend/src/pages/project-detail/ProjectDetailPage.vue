<template>
  <div>
    <!-- Header -->
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">{{ project?.name }}</h2>
        <p class="muted">{{ project?.description || '暂无描述' }}</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建任务</el-button>
    </div>

    <!-- Task cards -->
    <div v-loading="loading" class="task-list-wrap">
    <div v-if="taskItems.length" class="task-grid">
      <div
        v-for="item in taskItems"
        :key="item.task.id"
        class="task-card"
      >
        <!-- Status bar on top edge -->
        <div class="tc-status-bar" :class="`tc-status-bar--${item.lastRun?.status ?? 'none'}`" />

        <div class="tc-body">
          <div class="tc-row">
            <div class="tc-name" @click="$router.push(`/tasks/${item.task.id}/pipeline`)">
              {{ item.task.name }}
            </div>
            <el-dropdown trigger="click">
              <button class="tc-menu-btn">
                <el-icon :size="15"><MoreFilled /></el-icon>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item :icon="VideoPlay" @click="run(item.task.id)">执行</el-dropdown-item>
                  <el-dropdown-item :icon="Operation" @click="$router.push(`/tasks/${item.task.id}/pipeline`)">编辑 Pipeline</el-dropdown-item>
                  <el-dropdown-item :icon="Clock" @click="$router.push(`/tasks/${item.task.id}/runs`)">执行记录</el-dropdown-item>
                  <el-dropdown-item :icon="EditPen" @click="openEdit(item.task)">编辑设置</el-dropdown-item>
                  <el-dropdown-item :icon="CopyDocument" @click="forkTask(item.task)">Fork</el-dropdown-item>
                  <el-dropdown-item divided style="color: #ef4444" :icon="Delete" @click="remove(item.task.id)">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>

          <div v-if="item.task.description" class="tc-desc">{{ item.task.description }}</div>
          <!-- Last run info -->
          <div v-if="item.lastRun" class="tc-run">
            <StatusBadge :status="item.lastRun.status" size="small" />
            <span class="tc-run-info">
              #{{ item.lastRun.id }} · {{ fmtDate(item.lastRun.startedAt) }}
              <span v-if="item.lastRun.durationMs"> · {{ fmtDuration(item.lastRun.durationMs) }}</span>
            </span>
            <button class="tc-run-link" @click="$router.push(`/runs/${item.lastRun.id}`)">
              查看
            </button>
          </div>
          <div v-else class="tc-no-run">从未执行</div>
        </div>

        <!-- Footer actions -->
        <div class="tc-footer">
          <el-button size="small" link :icon="VideoPlay" type="primary" @click="run(item.task.id)">执行</el-button>
          <el-button size="small" link :icon="Operation" @click="$router.push(`/tasks/${item.task.id}/pipeline`)">Pipeline</el-button>
          <el-button size="small" link :icon="Clock" @click="$router.push(`/tasks/${item.task.id}/runs`)">记录</el-button>
        </div>
      </div>
    </div>

    <el-empty v-else-if="!loading" description="还没有任务，创建第一个吧" :image-size="80" />

    <div v-if="total > pageSize" class="pagination-row">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[12, 24, 48, 96]"
        layout="total, sizes, prev, pager, next"
        background
        @current-change="load"
        @size-change="onPageSizeChange"
      />
    </div>
    </div>

    <!-- Create / Edit dialog -->
    <el-dialog v-model="dialogVisible" :title="editId ? '编辑任务' : '新建任务'" width="480px" @closed="resetForm">
      <el-form label-position="top">
        <el-form-item label="任务名称" required>
          <el-input v-model="form.name" placeholder="任务名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="可选描述" />
        </el-form-item>
        <el-form-item label="超时时间 (秒)">
          <el-input-number v-model="form.timeoutSeconds" :min="0" style="width: 100%" />
          <div class="muted" style="margin-top: 4px; font-size: 12px">0 表示不限制</div>
        </el-form-item>
        <el-form-item label="允许并发执行">
          <el-switch v-model="form.allowConcurrent" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="save">
          {{ editId ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <RunTaskDialog ref="runDialog" @success="onRunSuccess" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Clock, CopyDocument, Delete, EditPen, MoreFilled, Operation, Plus, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import RunTaskDialog from '@/components/RunTaskDialog.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import type { Project, Task, TaskRun } from '@/types'
import { fmtDate, fmtDuration } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const projectId = Number(route.params.id)

const project = ref<Project>()
const tasks = ref<Task[]>([])
const recentRuns = ref<Map<number, TaskRun>>(new Map())
const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const editId = ref<number | null>(null)
const runDialog = ref<InstanceType<typeof RunTaskDialog>>()
const page = ref(1)
const pageSize = ref(12)
const total = ref(0)

const form = reactive({
  name: '',
  description: '',
  timeoutSeconds: 600,
  allowConcurrent: false,
})

const taskItems = computed(() =>
  tasks.value.map((task) => ({
    task,
    lastRun: recentRuns.value.get(task.id),
  })),
)

async function load() {
  loading.value = true
  try {
    const [proj, taskPage] = await Promise.all([
      api.project(projectId),
      api.tasksPage(projectId, page.value, pageSize.value),
    ])
    project.value = proj
    tasks.value = taskPage.items
    total.value = taskPage.total
    if (tasks.value.length === 0 && total.value > 0 && page.value > 1) {
      page.value -= 1
      await load()
      return
    }

    // Load recent runs for each task on the current page (take the first/latest one)
    const runLists = await Promise.all(
      tasks.value.map((t) => api.taskRuns(t.id).catch(() => [] as TaskRun[])),
    )
    const map = new Map<number, TaskRun>()
    for (let i = 0; i < tasks.value.length; i++) {
      const latest = runLists[i][0]
      if (latest) map.set(tasks.value[i].id, latest)
    }
    recentRuns.value = map
  } finally {
    loading.value = false
  }
}

function onPageSizeChange() {
  page.value = 1
  load()
}

function resetForm() {
  editId.value = null
  Object.assign(form, {
    name: '',
    description: '',
    timeoutSeconds: 600,
    allowConcurrent: false,
  })
}

function openCreate() { resetForm(); dialogVisible.value = true }

function openEdit(task: Task) {
  editId.value = task.id
  Object.assign(form, {
    name: task.name,
    description: task.description,
    timeoutSeconds: task.timeoutSeconds,
    allowConcurrent: task.allowConcurrent,
  })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入任务名称')
  submitting.value = true
  try {
    if (editId.value) {
      await api.updateTask(editId.value, form)
      ElMessage.success('已更新')
    } else {
      const task = await api.createTask(projectId, form)
      ElMessage.success('已创建')
      dialogVisible.value = false
      router.push(`/tasks/${task.id}/pipeline`)
      return
    }
    dialogVisible.value = false
    await load()
  } finally {
    submitting.value = false
  }
}

function run(taskId: number) { runDialog.value?.open(taskId) }

function onRunSuccess(run: TaskRun) { router.push(`/runs/${run.id}`) }

async function forkTask(task: Task) {
  const { value } = await ElMessageBox.prompt('请输入 Fork 后的新 Task 名称', 'Fork Task', {
    inputValue: `${task.name} copy`,
    inputPlaceholder: '新 Task 名称',
    inputValidator: (val) => Boolean(String(val || '').trim()) || '请填写 Task 名称',
    confirmButtonText: '创建',
    cancelButtonText: '取消',
  })
  const created = await api.createTask(projectId, {
    name: String(value).trim(),
    description: task.description,
    pipelineJson: task.pipelineJson,
    allowConcurrent: task.allowConcurrent,
    timeoutSeconds: task.timeoutSeconds,
  })
  ElMessage.success('已 Fork 新 Task')
  await load()
  router.push(`/tasks/${created.id}/pipeline`)
}

async function remove(taskId: number) {
  await ElMessageBox.confirm('确认删除该任务？此操作不可撤销。', '删除任务', { type: 'warning' })
  await api.deleteTask(taskId)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.task-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.task-list-wrap {
  min-height: 220px;
}

.task-card {
  background: #fff;
  border: 1px solid #dce4ef;
  border-radius: 10px;
  overflow: hidden;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.task-card:hover {
  border-color: #94a3b8;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

/* Top status bar */
.tc-status-bar {
  height: 3px;
  background: #e2e8f0;
}

.tc-status-bar--running  { background: #3b82f6; animation: slide 1.5s linear infinite; }
.tc-status-bar--success  { background: #22c55e; }
.tc-status-bar--failed   { background: #ef4444; }
.tc-status-bar--timeout  { background: #f97316; }
.tc-status-bar--canceled { background: #94a3b8; }
.tc-status-bar--none     { background: #e2e8f0; }

@keyframes slide {
  from { background-position: -200px 0; }
  to   { background-position: 200px 0; }
}

.tc-body { padding: 14px 16px 0; }

.tc-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.tc-name {
  font-size: 15px;
  font-weight: 700;
  color: #1f2328;
  cursor: pointer;
  flex: 1;
  line-height: 1.3;
  transition: color 0.15s;
}

.tc-name:hover { color: #0d9488; }

.tc-menu-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: #94a3b8;
  width: 26px; height: 26px;
  border-radius: 5px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  transition: background 0.15s, color 0.15s;
}
.tc-menu-btn:hover { background: #f1f5f9; color: #475569; }

.tc-desc {
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
  margin-bottom: 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.tc-run {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 0;
  border-top: 1px solid #f1f5f9;
}

.tc-run-info {
  font-size: 11px;
  color: #94a3b8;
  flex: 1;
}

.tc-run-link {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 11px;
  color: #64748b;
  padding: 2px 6px;
  border-radius: 4px;
  transition: background 0.15s, color 0.15s;
}
.tc-run-link:hover { background: #f1f5f9; color: #0d9488; }

.tc-no-run {
  font-size: 12px;
  color: #cbd5e1;
  padding: 8px 0;
  border-top: 1px solid #f1f5f9;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.tc-footer {
  display: flex;
  gap: 0;
  padding: 8px 10px;
  border-top: 1px solid #f1f5f9;
  background: #fafbfc;
}
</style>
