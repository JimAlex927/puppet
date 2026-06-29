<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">项目</h2>
        <p class="muted">管理你的 pipeline 项目</p>
      </div>
      <div class="project-actions">
        <el-button :icon="Upload" :loading="importing" @click="openImport">导入项目</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建项目</el-button>
      </div>
      <input ref="fileInput" class="hidden-file" type="file" accept=".zip,application/zip" @change="importFile" />
    </div>

    <div v-if="projects.length" class="project-grid">
      <div
        v-for="project in projects"
        :key="project.id"
        class="project-card"
        @click="$router.push(`/projects/${project.id}`)"
      >
        <div class="pc-header">
          <div class="pc-icon">
            <el-icon :size="18"><FolderOpened /></el-icon>
          </div>
          <el-dropdown trigger="click" @click.stop>
            <button class="pc-menu-btn" @click.stop>
              <el-icon :size="16"><MoreFilled /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click.stop="openEdit(project)">编辑</el-dropdown-item>
                <el-dropdown-item :icon="Download" @click.stop="download(project)">导出 ZIP</el-dropdown-item>
                <el-dropdown-item divided style="color: #ef4444" @click.stop="remove(project.id)">删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div class="pc-name">{{ project.name }}</div>
        <div class="pc-desc">{{ project.description || '暂无描述' }}</div>
        <div class="pc-footer">
          <span class="pc-date">更新于 {{ fmtDate(project.updatedAt) }}</span>
        </div>
      </div>
    </div>

    <el-empty v-else description="还没有项目，创建第一个吧" :image-size="80" />

    <el-dialog v-model="dialogVisible" :title="editId ? '编辑项目' : '新建项目'" width="440px" @closed="resetForm">
      <el-form label-position="top">
        <el-form-item label="项目名称" required>
          <el-input v-model="form.name" placeholder="我的项目" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="可选描述" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="save">
          {{ editId ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Download, FolderOpened, MoreFilled, Plus, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import type { Project } from '@/types'
import { fmtDate } from '@/utils/format'

const projects = ref<Project[]>([])
const dialogVisible = ref(false)
const submitting = ref(false)
const importing = ref(false)
const fileInput = ref<HTMLInputElement>()
const editId = ref<number | null>(null)

const form = reactive({ name: '', description: '' })

async function load() {
  projects.value = await api.projects()
}

function resetForm() {
  editId.value = null
  Object.assign(form, { name: '', description: '' })
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

function openEdit(p: Project) {
  editId.value = p.id
  Object.assign(form, { name: p.name, description: p.description })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入项目名称')
  submitting.value = true
  try {
    if (editId.value) {
      await api.updateProject(editId.value, form)
      ElMessage.success('已更新')
    } else {
      await api.createProject(form)
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    await load()
  } finally {
    submitting.value = false
  }
}

async function remove(id: number) {
  await ElMessageBox.confirm('确认删除该项目？项目下的所有任务将一并删除。', '删除项目', { type: 'warning' })
  await api.deleteProject(id)
  ElMessage.success('已删除')
  await load()
}

function openImport() {
  fileInput.value?.click()
}

async function importFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importing.value = true
  try {
    const project = await api.importProject(file)
    ElMessage.success(`已导入：${project.name}`)
    await load()
  } finally {
    importing.value = false
    input.value = ''
  }
}

async function download(project: Project) {
  const blob = await api.exportProject(project.id)
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${safeFileName(project.name)}.zip`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function safeFileName(name: string) {
  const value = name.trim().replace(/[<>:"/\\|?*\x00-\x1F]+/g, '-').replace(/^[. ]+|[. ]+$/g, '')
  return value || 'project'
}

onMounted(load)
</script>

<style scoped>
.project-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.hidden-file {
  display: none;
}

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}

.project-card {
  background: #fff;
  border: 1px solid #dce4ef;
  border-radius: 10px;
  padding: 18px;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s, transform 0.15s;
}

.project-card:hover {
  border-color: #2dd4bf;
  box-shadow: 0 4px 16px rgba(45, 212, 191, 0.12);
  transform: translateY(-1px);
}

.pc-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 12px;
}

.pc-icon {
  width: 40px; height: 40px;
  border-radius: 8px;
  background: #f0fdfb;
  border: 1px solid #ccfbf1;
  display: grid;
  place-items: center;
  color: #0d9488;
}

.pc-menu-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: #94a3b8;
  width: 28px; height: 28px;
  border-radius: 6px;
  display: grid;
  place-items: center;
  transition: background 0.15s, color 0.15s;
}
.pc-menu-btn:hover { background: #f1f5f9; color: #475569; }

.pc-name {
  font-size: 15px;
  font-weight: 700;
  color: #1f2328;
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pc-desc {
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 36px;
}

.pc-footer {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid #f1f5f9;
}

.pc-date {
  font-size: 11px;
  color: #94a3b8;
}
</style>
