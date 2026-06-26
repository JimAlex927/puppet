<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">{{ project?.name }}</h2>
        <p class="muted">{{ project?.description || '暂无描述' }}</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建任务</el-button>
    </div>

    <div class="panel">
      <el-table :data="tasks" empty-text="暂无任务">
        <el-table-column prop="name" label="任务" min-width="180" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="并发" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.allowConcurrent ? 'success' : 'info'">
              {{ row.allowConcurrent ? '允许' : '禁止' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="timeoutSeconds" label="超时(s)" width="90" />
        <el-table-column label="更新时间" width="155">
          <template #default="{ row }">{{ fmtDate(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="340" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :icon="VideoPlay" @click="run(row.id)">执行</el-button>
            <el-button link :icon="EditPen" @click="openEdit(row)">编辑</el-button>
            <el-button link :icon="Operation" @click="$router.push(`/tasks/${row.id}/pipeline`)">Pipeline</el-button>
            <el-button link :icon="Clock" @click="$router.push(`/tasks/${row.id}/runs`)">记录</el-button>
            <el-button link type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

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
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Clock, Delete, EditPen, Operation, Plus, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import RunTaskDialog from '@/components/RunTaskDialog.vue'
import type { Project, Task, TaskRun } from '@/types'
import { fmtDate } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const projectId = Number(route.params.id)
const project = ref<Project>()
const tasks = ref<Task[]>([])
const dialogVisible = ref(false)
const submitting = ref(false)
const editId = ref<number | null>(null)
const runDialog = ref<InstanceType<typeof RunTaskDialog>>()

const form = reactive({
  name: '',
  description: '',
  timeoutSeconds: 600,
  allowConcurrent: false,
})

async function load() {
  project.value = await api.project(projectId)
  tasks.value = await api.tasks(projectId)
}

function resetForm() {
  editId.value = null
  Object.assign(form, { name: '', description: '', timeoutSeconds: 600, allowConcurrent: false })
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

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

function run(taskId: number) {
  runDialog.value?.open(taskId)
}

function onRunSuccess(run: TaskRun) {
  router.push(`/runs/${run.id}`)
}

async function remove(taskId: number) {
  await ElMessageBox.confirm('确认删除该任务？此操作不可撤销。', '删除任务', { type: 'warning' })
  await api.deleteTask(taskId)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
