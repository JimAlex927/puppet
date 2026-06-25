<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">{{ project?.name }}</h2>
        <p class="muted">{{ project?.description || 'No description' }}</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建任务</el-button>
    </div>
    <div class="panel">
      <el-table :data="tasks" empty-text="暂无任务">
        <el-table-column prop="name" label="任务" min-width="180" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="并发" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.allowConcurrent ? 'success' : 'info'">
              {{ row.allowConcurrent ? '允许' : '禁止' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="timeoutSeconds" label="超时(s)" width="90" />
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">{{ fmtDate(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="320" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :icon="VideoPlay" @click="run(row.id)">执行</el-button>
            <el-button link :icon="Operation" @click="$router.push(`/tasks/${row.id}/pipeline`)">Pipeline</el-button>
            <el-button link :icon="Clock" @click="$router.push(`/tasks/${row.id}/runs`)">记录</el-button>
            <el-button link type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" title="新建任务" width="480px">
      <el-form label-position="top">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="任务名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="可选描述" />
        </el-form-item>
        <el-form-item label="超时秒数">
          <el-input-number v-model="form.timeoutSeconds" :min="0" style="width: 100%" />
        </el-form-item>
        <el-form-item label="允许并发">
          <el-switch v-model="form.allowConcurrent" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">创建</el-button>
      </template>
    </el-dialog>

    <RunTaskDialog ref="runDialog" @success="onRunSuccess" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Clock, Delete, Operation, Plus, VideoPlay } from '@element-plus/icons-vue'
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
const runDialog = ref<InstanceType<typeof RunTaskDialog>>()
const form = reactive({ name: '', description: '', timeoutSeconds: 600, allowConcurrent: false })

async function load() {
  project.value = await api.project(projectId)
  tasks.value = await api.tasks(projectId)
}

function openCreate() {
  Object.assign(form, { name: '', description: '', timeoutSeconds: 600, allowConcurrent: false })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入任务名称')
  await api.createTask(projectId, form)
  dialogVisible.value = false
  ElMessage.success('已创建')
  await load()
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
