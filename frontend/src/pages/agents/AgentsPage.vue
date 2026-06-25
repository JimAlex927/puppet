<template>
  <div>
    <div class="page-actions">
      <div />
      <el-button type="primary" :icon="Plus" @click="openCreate">新建 Agent</el-button>
    </div>
    <div class="panel">
      <el-table :data="agents" empty-text="暂无 Agent">
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column prop="endpointUrl" label="Endpoint" min-width="220" />
        <el-table-column prop="os" label="OS" width="100" />
        <el-table-column prop="arch" label="Arch" width="110" />
        <el-table-column prop="hostname" label="Hostname" min-width="160" />
        <el-table-column label="Labels" min-width="180">
          <template #default="{ row }">
            <el-tag v-for="label in labels(row.labelsJson)" :key="label" size="small" style="margin-right: 6px">
              {{ label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Status" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' || row.status === 'running' ? 'success' : 'info'">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Last Heartbeat" width="190">
          <template #default="{ row }">{{ fmtDate(row.lastHeartbeatAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link :icon="Edit" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.name !== 'local-agent'" link type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑 Agent' : '新建 Agent'" width="560px">
      <el-form label-position="top">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="Endpoint URL">
          <el-input v-model="form.endpointUrl" placeholder="http://agent-host:9090" />
        </el-form-item>
        <el-form-item label="Labels">
          <el-select v-model="form.labels" multiple filterable allow-create default-first-option style="width: 100%">
            <el-option label="local" value="local" />
            <el-option label="linux" value="linux" />
            <el-option label="windows" value="windows" />
            <el-option label="docker" value="docker" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="tokenVisible" title="Agent Token" width="620px">
      <el-alert type="warning" :closable="false" show-icon title="Token 只显示一次，请保存到 Agent 启动参数中。" />
      <el-input v-model="createdToken" type="textarea" :rows="4" readonly style="margin-top: 14px" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import type { Agent, AgentInput } from '@/types'
import { fmtDate } from '@/utils/format'

const agents = ref<Agent[]>([])
const dialogVisible = ref(false)
const tokenVisible = ref(false)
const createdToken = ref('')
const form = reactive<AgentInput & { id: number }>({ id: 0, name: '', endpointUrl: '', labels: [] })

function labels(value: string) {
  try {
    return JSON.parse(value) as string[]
  } catch {
    return []
  }
}

async function load() {
  agents.value = await api.agents()
}

function openCreate() {
  Object.assign(form, { id: 0, name: '', endpointUrl: '', labels: [] })
  dialogVisible.value = true
}

function openEdit(agent: Agent) {
  Object.assign(form, { id: agent.id, name: agent.name, endpointUrl: agent.endpointUrl, labels: labels(agent.labelsJson) })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请输入 Agent 名称')
  if (form.id) {
    await api.updateAgent(form.id, form)
    ElMessage.success('已保存')
  } else {
    const resp = await api.createAgent(form)
    createdToken.value = resp.token
    tokenVisible.value = true
    ElMessage.success('已创建')
  }
  dialogVisible.value = false
  await load()
}

async function remove(id: number) {
  await ElMessageBox.confirm('确认删除该 Agent？')
  await api.deleteAgent(id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
