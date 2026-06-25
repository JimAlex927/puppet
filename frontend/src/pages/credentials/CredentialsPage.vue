<template>
  <div>
    <div class="page-actions">
      <div />
      <el-button type="primary" :icon="Plus" @click="openCreate">新建凭据</el-button>
    </div>
    <div class="panel">
      <el-table :data="credentials" empty-text="暂无凭据">
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column label="类型" width="170">
          <template #default="{ row }">
            <el-tag>{{ typeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" min-width="160" />
        <el-table-column prop="description" label="描述" min-width="220" />
        <el-table-column label="Secret" width="100">
          <template #default="{ row }">
            <el-tag :type="row.hasSecret ? 'success' : 'info'">{{ row.hasSecret ? '已保存' : '无' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="190">
          <template #default="{ row }">{{ fmtDate(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link :icon="Edit" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑凭据' : '新建凭据'" width="560px">
      <el-form label-position="top">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width: 100%">
            <el-option label="Username / Password" value="username_password" />
            <el-option label="Token" value="token" />
            <el-option label="SSH Private Key" value="ssh_key" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.type !== 'token'" label="用户名">
          <el-input v-model="form.username" :placeholder="form.type === 'ssh_key' ? 'git' : ''" />
        </el-form-item>
        <el-form-item v-if="form.type === 'token'" label="用户名">
          <el-input v-model="form.username" placeholder="默认 x-access-token" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-alert
          v-if="form.id"
          type="info"
          show-icon
          :closable="false"
          title="Secret 留空会保留原值"
          style="margin-bottom: 12px"
        />
        <el-form-item v-if="form.type === 'username_password'" label="Password">
          <el-input v-model="form.password" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item v-if="form.type === 'token'" label="Token">
          <el-input v-model="form.token" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item v-if="form.type === 'ssh_key'" label="Private Key">
          <el-input v-model="form.privateKey" type="textarea" :rows="8" />
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
import type { Credential, CredentialInput } from '@/types'
import { fmtDate } from '@/utils/format'

const credentials = ref<Credential[]>([])
const dialogVisible = ref(false)
const form = reactive<CredentialInput & { id: number }>({
  id: 0,
  name: '',
  type: 'username_password',
  description: '',
  username: '',
  password: '',
  token: '',
  privateKey: '',
})

async function load() {
  credentials.value = await api.credentials()
}

function openCreate() {
  Object.assign(form, {
    id: 0,
    name: '',
    type: 'username_password',
    description: '',
    username: '',
    password: '',
    token: '',
    privateKey: '',
  })
  dialogVisible.value = true
}

function openEdit(credential: Credential) {
  Object.assign(form, {
    id: credential.id,
    name: credential.name,
    type: credential.type,
    description: credential.description,
    username: credential.username,
    password: '',
    token: '',
    privateKey: '',
  })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入凭据名称')
  const payload: CredentialInput = {
    name: form.name,
    type: form.type,
    description: form.description,
    username: form.username,
  }
  if (form.password) payload.password = form.password
  if (form.token) payload.token = form.token
  if (form.privateKey) payload.privateKey = form.privateKey
  if (form.id) await api.updateCredential(form.id, payload)
  else await api.createCredential(payload)
  dialogVisible.value = false
  ElMessage.success('已保存')
  await load()
}

async function remove(id: number) {
  await ElMessageBox.confirm('确认删除该凭据？正在使用它的 Pipeline 会执行失败。')
  await api.deleteCredential(id)
  ElMessage.success('已删除')
  await load()
}

function typeLabel(type: Credential['type']) {
  if (type === 'username_password') return 'Username / Password'
  if (type === 'token') return 'Token'
  return 'SSH Key'
}

onMounted(load)
</script>
