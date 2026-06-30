<template>
  <router-view v-if="$route.path === '/login'" />
  <el-container v-else class="app-shell">
    <el-aside width="232px" class="sidebar">
      <div class="brand">
        <div class="brand-mark">P</div>
        <div>
          <strong>Puppet</strong>
          <span>Pipeline</span>
        </div>
      </div>
      <el-menu router :default-active="$route.path" class="nav-menu">
        <el-menu-item index="/dashboard">
          <el-icon><DataBoard /></el-icon>
          <span>Dashboard</span>
        </el-menu-item>
        <el-menu-item index="/projects">
          <el-icon><FolderOpened /></el-icon>
          <span>Projects</span>
        </el-menu-item>
        <el-menu-item index="/shared-files">
          <el-icon><Files /></el-icon>
          <span>Shared Files</span>
        </el-menu-item>
        <el-menu-item index="/schedules">
          <el-icon><Calendar /></el-icon>
          <span>Schedules</span>
        </el-menu-item>
        <el-menu-item index="/agents">
          <el-icon><Monitor /></el-icon>
          <span>Agents</span>
        </el-menu-item>
        <el-menu-item index="/credentials">
          <el-icon><Key /></el-icon>
          <span>Credentials</span>
        </el-menu-item>
        <el-menu-item index="/users">
          <el-icon><User /></el-icon>
          <span>Users</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="topbar">
        <div>
          <h1>{{ title }}</h1>
        </div>
        <div class="topbar-user">
          <span class="topbar-username">{{ currentUser?.displayName || currentUser?.username }}</span>
          <el-button size="small" :icon="SwitchButton" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main-panel">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Calendar, DataBoard, Files, FolderOpened, Key, Monitor, SwitchButton, User } from '@element-plus/icons-vue'
import { api } from '@/api'
import type { User as UserType } from '@/types'

const route = useRoute()
const router = useRouter()
const currentUser = ref<UserType>()
const title = computed(() => String(route.meta.title || 'Puppet'))

async function loadUser() {
  if (!localStorage.getItem('puppet_token')) return
  try {
    currentUser.value = await api.me()
  } catch {
    // token invalid — interceptor will redirect to login
  }
}

async function logout() {
  try {
    await api.logout()
  } catch {
    // ignore errors
  }
  localStorage.removeItem('puppet_token')
  router.push('/login')
}

onMounted(loadUser)
</script>
