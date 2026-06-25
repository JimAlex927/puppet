<template>
  <div>
    <div class="metrics-grid">
      <div v-for="item in metrics" :key="item.label" class="metric-card">
        <div class="metric-label">{{ item.label }}</div>
        <div class="metric-value">{{ item.value }}</div>
      </div>
    </div>
    <div class="panel">
      <div class="page-actions">
        <h3>最近执行</h3>
        <el-button :icon="Refresh" circle @click="load" />
      </div>
      <el-table :data="summary?.recentRuns || []" empty-text="暂无执行记录">
        <el-table-column prop="id" label="Run ID" width="100" />
        <el-table-column prop="taskId" label="Task ID" width="110" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <StatusBadge :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column label="开始时间">
          <template #default="{ row }">{{ fmtDate(row.startedAt) }}</template>
        </el-table-column>
        <el-table-column label="耗时" width="120">
          <template #default="{ row }">{{ fmtDuration(row.durationMs) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="$router.push(`/runs/${row.id}`)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { api } from '@/api'
import StatusBadge from '@/components/StatusBadge.vue'
import type { DashboardSummary } from '@/types'
import { fmtDate, fmtDuration } from '@/utils/format'

const summary = ref<DashboardSummary>()

const metrics = computed(() => [
  { label: '项目数量', value: summary.value?.projectCount ?? 0 },
  { label: '任务数量', value: summary.value?.taskCount ?? 0 },
  { label: '今日执行', value: summary.value?.todayRunCount ?? 0 },
  { label: '运行中', value: summary.value?.runningCount ?? 0 },
  { label: '成功', value: summary.value?.successCount ?? 0 },
  { label: '失败', value: summary.value?.failedCount ?? 0 },
  { label: 'Agent 在线', value: summary.value?.agentOnlineCount ?? 0 },
])

async function load() {
  summary.value = await api.dashboard()
}

onMounted(load)
</script>
