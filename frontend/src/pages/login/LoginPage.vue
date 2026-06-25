<template>
  <div class="login-page">
    <div class="login-panel">
      <div class="brand login-brand">
        <div class="brand-mark">P</div>
        <div>
          <strong>Puppet</strong>
          <span>Pipeline MVP</span>
        </div>
      </div>
      <el-form label-position="top" @keyup.enter="submit">
        <el-form-item label="Username">
          <el-input v-model="form.username" autofocus />
        </el-form-item>
        <el-form-item label="Password">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-button type="primary" style="width: 100%" :loading="loading" @click="submit">登录</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '@/api'

const router = useRouter()
const loading = ref(false)
const form = reactive({ username: 'puppetadmin', password: 'puppetadmin' })

async function submit() {
  loading.value = true
  try {
    const resp = await api.login(form)
    localStorage.setItem('puppet_token', resp.token)
    ElMessage.success('登录成功')
    router.push('/dashboard')
  } finally {
    loading.value = false
  }
}
</script>
