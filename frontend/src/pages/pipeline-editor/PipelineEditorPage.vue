<template>
  <div v-loading="loading">
    <!-- ── Header ──────────────────────────────────────────────────── -->
    <div class="page-actions" style="align-items: flex-start; margin-bottom: 12px">
      <div style="flex: 1; min-width: 0">
        <el-breadcrumb separator="/" style="margin-bottom: 8px; font-size: 13px">
          <el-breadcrumb-item :to="{ path: '/projects' }">项目</el-breadcrumb-item>
          <el-breadcrumb-item v-if="task" :to="{ path: `/projects/${task.projectId}` }">
            {{ projectName }}
          </el-breadcrumb-item>
          <el-breadcrumb-item>Pipeline</el-breadcrumb-item>
        </el-breadcrumb>
        <el-input
          v-model="taskForm.name"
          placeholder="任务名称"
          size="large"
          style="width: 320px; font-weight: 600"
        />
      </div>
      <el-space style="flex-shrink: 0; margin-top: 28px">
        <el-segmented v-model="viewMode" :options="viewOptions" />
        <el-button :icon="Setting" @click="settingsVisible = true">任务设置</el-button>
        <el-button :icon="Back" @click="goBack">返回</el-button>
        <el-button type="primary" :icon="DocumentChecked" :loading="saving" @click="save">
          保存
        </el-button>
      </el-space>
    </div>

    <!-- ── Pipeline Config Strip ──────────────────────────────────── -->
    <div v-if="pipeline" class="panel pipeline-strip">
      <el-form :inline="true" style="margin: 0">
        <el-form-item label="Agent" style="margin-bottom: 0">
          <el-select v-model="pipeline.agentSelector.labels" multiple style="width: 200px">
            <el-option label="local" value="local" />
          </el-select>
        </el-form-item>
        <el-form-item label="起始节点" style="margin-bottom: 0">
          <el-select
            v-model="pipeline.startNodeId"
            clearable
            placeholder="默认第一个节点"
            style="width: 240px"
          >
            <el-option
              v-for="n in pipeline.nodes"
              :key="n.id"
              :label="`${n.name} (${n.id})`"
              :value="n.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
    </div>

    <!-- ── Editor Layout ──────────────────────────────────────────── -->
    <div v-if="pipeline" class="pipeline-layout" style="margin-top: 16px">
      <!-- Left: Node Palette -->
      <NodePalette :node-types="nodeTypes" @add="addNode" />

      <!-- Center: List or Canvas -->
      <PipelineListView
        v-if="viewMode === 'list'"
        :pipeline="pipeline"
        :source-types="sourceTypes"
        :credentials="credentials"
        :selected-node-id="selectedNodeId"
        @update:selected-node-id="selectedNodeId = $event"
        @remove="removeNode"
        @move="moveNode"
        @add-input="addInput"
      />
      <PipelineCanvasView
        v-else
        :pipeline="pipeline"
        :selected-node-id="selectedNodeId"
        @update:selected-node-id="selectedNodeId = $event"
      />

      <!-- Right: Config Panel -->
      <NodeConfigPanel
        :node="selectedNode"
        :metadata="selectedMetadata"
        :nodes="pipeline.nodes"
        :credentials="credentials"
      />
    </div>

    <!-- ── Task Settings Drawer ───────────────────────────────────── -->
    <el-drawer v-model="settingsVisible" title="任务设置" direction="rtl" size="360px">
      <el-form label-position="top" style="padding: 0 4px">
        <el-form-item label="描述">
          <el-input
            v-model="taskForm.description"
            type="textarea"
            :rows="4"
            placeholder="任务描述（可选）"
          />
        </el-form-item>
        <el-form-item label="超时时间 (秒)">
          <el-input-number v-model="taskForm.timeoutSeconds" :min="0" style="width: 100%" />
          <div class="muted" style="margin-top: 6px; font-size: 12px">0 表示不限制超时</div>
        </el-form-item>
        <el-form-item label="允许并发执行">
          <el-switch v-model="taskForm.allowConcurrent" />
          <span class="muted" style="margin-left: 10px; font-size: 12px">
            {{ taskForm.allowConcurrent ? '同一任务可同时运行多次' : '同一时间只允许运行一次' }}
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="settingsVisible = false">关闭</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Back, DocumentChecked, List, Operation, Setting } from '@element-plus/icons-vue'
import { usePipeline } from '@/composables/usePipeline'
import NodeConfigPanel from '@/components/PipelineEditor/NodeConfigPanel.vue'
import NodePalette from '@/components/PipelineEditor/NodePalette.vue'
import PipelineCanvasView from '@/components/PipelineEditor/PipelineCanvasView.vue'
import PipelineListView from '@/components/PipelineEditor/PipelineListView.vue'

const route = useRoute()
const router = useRouter()
const taskId = Number(route.params.id)

const {
  pipeline,
  task,
  projectName,
  nodeTypes,
  sourceTypes,
  credentials,
  loading,
  saving,
  selectedNodeId,
  selectedNode,
  selectedMetadata,
  taskForm,
  load,
  addNode,
  removeNode,
  moveNode,
  addInput,
  save,
} = usePipeline(taskId)

const viewMode = ref<'list' | 'canvas'>('list')
const settingsVisible = ref(false)

const viewOptions = [
  { label: '列表', value: 'list', icon: List },
  { label: '流程图', value: 'canvas', icon: Operation },
]

function goBack() {
  if (task.value) {
    router.push(`/projects/${task.value.projectId}`)
  } else {
    router.back()
  }
}

onMounted(load)
</script>

<style scoped>
.pipeline-strip {
  padding: 10px 16px;
  margin-bottom: 0;
}
</style>
