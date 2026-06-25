<template>
  <div>
    <div class="page-actions">
      <div />
      <el-button type="primary" :icon="Plus" @click="openCreate">新建用户</el-button>
    </div>
    <div class="panel">
      <el-table :data="users">
        <el-table-column prop="username" label="用户名" min-width="160" />
        <el-table-column prop="displayName" label="显示名" min-width="160" />
        <el-table-column prop="role" label="角色" width="100" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最后登录" width="190">
          <template #default="{ row }">{{ fmtDate(row.lastLoginAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170">
          <template #default="{ row }">
            <el-button link :icon="Edit" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑用户' : '新建用户'" width="460px">
      <el-form label-position="top">
        <el-form-item label="用户名">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="form.displayName" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="admin" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="active" value="active" />
            <el-option label="disabled" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item :label="form.id ? '新密码（留空不修改）' : '密码'">
          <el-input v-model="form.password" type="password" show-password />
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
import type { User, UserInput } from '@/types'
import { fmtDate } from '@/utils/format'

const users = ref<User[]>([])
const dialogVisible = ref(false)
const form = reactive<UserInput & { id: number }>({ id: 0, username: '', displayName: '', role: 'admin', status: 'active', password: '' })

async function load() {
  users.value = await api.users()
}

function openCreate() {
  Object.assign(form, { id: 0, username: '', displayName: '', role: 'admin', status: 'active', password: '' })
  dialogVisible.value = true
}

function openEdit(user: User) {
  Object.assign(form, { id: user.id, username: user.username, displayName: user.displayName, role: user.role, status: user.status, password: '' })
  dialogVisible.value = true
}

async function save() {
  const payload: UserInput = { username: form.username, displayName: form.displayName, role: form.role, status: form.status }
  if (form.password) payload.password = form.password
  if (form.id) await api.updateUser(form.id, payload)
  else await api.createUser(payload)
  dialogVisible.value = false
  ElMessage.success('已保存')
  await load()
}

async function remove(id: number) {
  await ElMessageBox.confirm('确认删除该用户？')
  await api.deleteUser(id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
