<template>
  <div>
    <div class="page-actions">
      <div />
      <el-button type="primary" :icon="Plus" @click="openCreate">新建项目</el-button>
    </div>
    <div class="panel">
      <el-table :data="projects" empty-text="暂无项目">
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column prop="description" label="描述" min-width="260" />
        <el-table-column label="创建时间" width="190">
          <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="$router.push(`/projects/${row.id}`)">进入</el-button>
            <el-button link :icon="Edit" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑项目' : '新建项目'" width="460px">
      <el-form label-position="top">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="4" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import type { Project } from '@/types'
import { fmtDate } from '@/utils/format'

const projects = ref<Project[]>([])
const dialogVisible = ref(false)
const form = reactive({ id: 0, name: '', description: '' })

async function load() {
  projects.value = await api.projects()
}

function openCreate() {
  Object.assign(form, { id: 0, name: '', description: '' })
  dialogVisible.value = true
}

function openEdit(project: Project) {
  Object.assign(form, { id: project.id, name: project.name, description: project.description })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入项目名称')
  if (form.id) await api.updateProject(form.id, form)
  else await api.createProject(form)
  dialogVisible.value = false
  ElMessage.success('已保存')
  await load()
}

async function remove(id: number) {
  await ElMessageBox.confirm('确认删除该项目？')
  await api.deleteProject(id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
