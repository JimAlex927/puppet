<template>
  <div class="editor-page" v-loading="loading" element-loading-background="#1a1b23">
    <!-- ── Top bar ─────────────────────────────────────────────────── -->
    <header class="editor-bar">
      <el-breadcrumb separator="/" class="editor-breadcrumb">
        <el-breadcrumb-item :to="{ path: '/projects' }">项目</el-breadcrumb-item>
        <el-breadcrumb-item v-if="task" :to="{ path: `/projects/${task.projectId}` }">
          {{ projectName }}
        </el-breadcrumb-item>
        <el-breadcrumb-item>Pipeline</el-breadcrumb-item>
      </el-breadcrumb>

      <el-input
        v-model="taskForm.name"
        placeholder="任务名称"
        size="small"
        class="editor-title-input"
      />

      <el-space>
        <el-button size="small" :icon="Setting" @click="settingsVisible = true">设置</el-button>
        <el-button size="small" :icon="Back" @click="goBack">返回</el-button>
        <el-button size="small" type="primary" :icon="DocumentChecked" :loading="saving" @click="onSave">
          保存
        </el-button>
      </el-space>
    </header>

    <!-- ── Editor area: palette + canvas + config ──────────────────── -->
    <div v-if="pipeline" class="editor-body">
      <NodePalette :node-types="nodeTypes" />

      <PipelineCanvas
        ref="canvasRef"
        @connect="handleConnect"
        @edges-delete="(d) => handleEdgesDelete(d.map(x => [x.sourceId, x.sourceHandle] as [string, string]))"
        @nodes-delete="handleNodesDelete"
        @node-click="selectedNodeId = $event"
        @pane-click="selectedNodeId = null"
        @node-drop="onNodeDrop"
      />

      <NodeConfigDrawer
        :node="selectedNode"
        :metadata="selectedMetadata"
        :credentials="credentials"
        @close="selectedNodeId = null"
      />
    </div>

    <!-- ── Task settings drawer ───────────────────────────────────── -->
    <el-drawer
      v-model="settingsVisible"
      title="任务设置"
      direction="rtl"
      size="400px"
      :append-to-body="true"
    >
      <el-tabs>
        <!-- Basic settings -->
        <el-tab-pane label="基本设置">
          <el-form label-position="top" style="padding-top:4px">
            <el-form-item label="描述">
              <el-input v-model="taskForm.description" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item label="超时时间 (秒)">
              <el-input-number v-model="taskForm.timeoutSeconds" :min="0" style="width: 100%" />
              <div class="muted" style="margin-top: 6px; font-size: 12px">0 表示不限制</div>
            </el-form-item>
            <el-form-item label="允许并发执行">
              <el-switch v-model="taskForm.allowConcurrent" />
            </el-form-item>
            <el-form-item label="Agent">
              <el-select v-model="pipeline!.agentSelector.labels" multiple style="width: 100%">
                <el-option label="local" value="local" />
              </el-select>
            </el-form-item>
            <el-form-item label="起始节点">
              <el-select v-model="pipeline!.startNodeId" clearable style="width: 100%">
                <el-option
                  v-for="n in pipeline!.nodes"
                  :key="n.id"
                  :label="`${n.name} (${n.id})`"
                  :value="n.id"
                />
              </el-select>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Pipeline inputs (run parameters) -->
        <el-tab-pane :label="`运行参数 (${pipeline?.inputs.length ?? 0})`">
          <div style="padding-top:8px">
            <!-- Input list -->
            <div
              v-for="(inp, idx) in pipeline!.inputs"
              :key="idx"
              class="pi-row"
            >
              <div class="pi-info">
                <span class="pi-name">{{ inp.name }}</span>
                <el-tag size="small" style="margin-left:6px">{{ inp.type }}</el-tag>
                <el-tag v-if="inp.required" size="small" type="danger" style="margin-left:4px">必填</el-tag>
                <div class="pi-label">{{ inp.label }}</div>
              </div>
              <el-space>
                <el-button link :icon="EditPen" @click="openEditInput(idx)" />
                <el-button link :icon="Delete" type="danger" @click="removeInput(idx)" />
              </el-space>
            </div>

            <el-empty v-if="!pipeline!.inputs.length" description="尚未配置运行参数" :image-size="60" />

            <el-button
              style="width:100%;margin-top:12px"
              :icon="Plus"
              @click="openAddInput"
            >
              添加参数
            </el-button>
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="settingsVisible = false">关闭</el-button>
      </template>
    </el-drawer>

    <!-- Input edit dialog -->
    <el-dialog
      v-model="inputDialogVisible"
      :title="editingInputIdx === null ? '添加运行参数' : '编辑运行参数'"
      width="440px"
      append-to-body
      @closed="resetInputForm"
    >
      <el-form label-position="top">
        <el-form-item label="参数名 (变量名)" required>
          <el-input v-model="inputForm.name" placeholder="例：branch" />
        </el-form-item>
        <el-form-item label="显示标签">
          <el-input v-model="inputForm.label" placeholder="例：Git 分支" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="inputForm.type" style="width:100%">
            <el-option label="文本 (string)" value="string" />
            <el-option label="下拉选择 (select)" value="select" />
            <el-option label="数字 (number)" value="number" />
            <el-option label="开关 (boolean)" value="boolean" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认值">
          <el-input v-if="inputForm.type !== 'boolean'" v-model="inputForm.defaultText" placeholder="可选" />
          <el-switch v-else v-model="inputForm.defaultBool" />
        </el-form-item>
        <template v-if="inputForm.type === 'select'">
          <el-form-item label="数据来源">
            <el-select v-model="inputForm.sourceType" style="width:100%" @change="onSourceTypeChange">
              <el-option label="静态选项" value="static" />
              <el-option
                v-for="t in sourceTypes"
                :key="t.type"
                :label="t.name"
                :value="t.type"
              />
            </el-select>
          </el-form-item>

          <!-- Static options -->
          <el-form-item v-if="inputForm.sourceType === 'static'" label="选项 (每行一个)">
            <el-input
              v-model="inputForm.optionsText"
              type="textarea"
              :rows="4"
              placeholder="选项1&#10;选项2&#10;选项3"
            />
          </el-form-item>

          <!-- Dynamic source params -->
          <template v-else>
            <el-form-item
              v-for="field in currentSourceMeta?.fields ?? []"
              :key="field.name"
              :label="field.label"
            >
              <el-input v-if="field.type === 'input'" v-model="(inputForm.sourceParams as any)[field.name]" />
              <el-input
                v-else-if="field.type === 'textarea'"
                v-model="(inputForm.sourceParams as any)[field.name]"
                type="textarea"
                :rows="4"
                placeholder="标准输出每行作为一个选项"
              />
              <el-input-number
                v-else-if="field.type === 'number'"
                v-model="(inputForm.sourceParams as any)[field.name]"
                style="width:100%"
              />
              <el-select
                v-else-if="field.type === 'select'"
                v-model="(inputForm.sourceParams as any)[field.name]"
                style="width:100%"
              >
                <el-option v-for="opt in field.options ?? []" :key="opt" :label="opt" :value="opt" />
              </el-select>
              <el-select
                v-else-if="field.type === 'credential'"
                v-model="(inputForm.sourceParams as any)[field.name]"
                clearable
                style="width:100%"
              >
                <el-option label="无需凭据" :value="0" />
                <el-option
                  v-for="cred in credentials"
                  :key="cred.id"
                  :label="`${cred.name} (${cred.type})`"
                  :value="cred.id"
                />
              </el-select>
            </el-form-item>
          </template>
        </template>
        <el-form-item label="必填">
          <el-switch v-model="inputForm.required" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="inputDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveInput">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Back, Delete, DocumentChecked, EditPen, Plus, Setting } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { usePipelineEditor } from '@/composables/usePipelineEditor'
import NodePalette from '@/components/canvas/NodePalette.vue'
import PipelineCanvas from '@/components/canvas/PipelineCanvas.vue'
import NodeConfigDrawer from '@/components/canvas/NodeConfigDrawer.vue'
import type { NodeMetadata, PipelineInput } from '@/types'

const route = useRoute()
const router = useRouter()
const taskId = Number(route.params.id)

const {
  pipeline, task, projectName, nodeTypes, sourceTypes, credentials,
  loading, saving,
  selectedNodeId, selectedNode, selectedMetadata,
  taskForm,
  load, buildFlowElements,
  handleConnect, handleEdgesDelete, handleNodesDelete,
  createPipelineNode, savePipelineOnly, save,
} = usePipelineEditor(taskId)

const canvasRef = ref<InstanceType<typeof PipelineCanvas>>()
const settingsVisible = ref(false)

// ── Pipeline inputs management ─────────────────────────────────
const inputDialogVisible = ref(false)
const editingInputIdx = ref<number | null>(null)

const inputForm = reactive({
  name: '',
  label: '',
  type: 'string' as PipelineInput['type'],
  required: false,
  defaultText: '',
  defaultBool: false,
  optionsText: '',
  sourceType: 'static',
  sourceParams: {} as Record<string, unknown>,
})

const currentSourceMeta = computed(() =>
  sourceTypes.value.find(t => t.type === inputForm.sourceType),
)

function resetInputForm() {
  editingInputIdx.value = null
  Object.assign(inputForm, {
    name: '', label: '', type: 'string', required: false,
    defaultText: '', defaultBool: false, optionsText: '',
    sourceType: 'static', sourceParams: {},
  })
}

function openAddInput() {
  resetInputForm()
  inputDialogVisible.value = true
}

function openEditInput(idx: number) {
  const inp = pipeline.value!.inputs[idx]
  editingInputIdx.value = idx
  inputForm.name = inp.name
  inputForm.label = inp.label
  inputForm.type = inp.type
  inputForm.required = inp.required
  inputForm.defaultBool = inp.type === 'boolean' ? Boolean(inp.default) : false
  inputForm.defaultText = inp.type !== 'boolean' && inp.default != null ? String(inp.default) : ''
  inputForm.sourceType = inp.source?.type ?? 'static'
  inputForm.sourceParams = inp.source ? { ...inp.source.params } : {}
  inputForm.optionsText = (inp.options ?? []).join('\n')
  inputDialogVisible.value = true
}

// Only called when user manually changes the source type dropdown — NOT during programmatic form population
function onSourceTypeChange(type: string) {
  if (type === 'static') { inputForm.sourceParams = {}; return }
  const meta = sourceTypes.value.find(t => t.type === type)
  const params: Record<string, unknown> = {}
  for (const field of meta?.fields ?? []) {
    params[field.name] = field.default ?? (field.type === 'number' ? 0 : field.type === 'credential' ? 0 : '')
  }
  inputForm.sourceParams = params
}

async function saveInput() {
  if (!inputForm.name.trim()) { ElMessage.warning('请填写参数名'); return }

  const isStatic = inputForm.sourceType === 'static'
  const inp: PipelineInput = {
    name: inputForm.name.trim(),
    label: inputForm.label.trim() || inputForm.name.trim(),
    type: inputForm.type,
    required: inputForm.required,
    default: inputForm.type === 'boolean' ? inputForm.defaultBool : (inputForm.defaultText.trim() || undefined),
    options: (inputForm.type === 'select' && isStatic)
      ? inputForm.optionsText.split('\n').map(s => s.trim()).filter(Boolean)
      : undefined,
    source: (inputForm.type === 'select' && !isStatic)
      ? { type: inputForm.sourceType, params: { ...inputForm.sourceParams } }
      : undefined,
  }

  if (editingInputIdx.value === null) {
    pipeline.value!.inputs.push(inp)
  } else {
    pipeline.value!.inputs[editingInputIdx.value] = inp
  }
  inputDialogVisible.value = false
  try {
    await savePipelineOnly()
    ElMessage.success('参数已保存')
  } catch {
    ElMessage.error('保存失败，请点击顶部"保存"按钮重试')
  }
}

function removeInput(idx: number) {
  pipeline.value!.inputs.splice(idx, 1)
}

// Init canvas once pipeline + canvas are both ready
watch([pipeline, () => !!canvasRef.value], ([pl, ready]) => {
  if (!pl || !ready) return
  const { nodes, edges } = buildFlowElements()
  canvasRef.value!.initCanvas(nodes, edges)
}, { immediate: false })

function onNodeDrop(meta: NodeMetadata, position: { x: number; y: number }) {
  if (!pipeline.value) return
  const { vfNode } = createPipelineNode(meta, position)
  canvasRef.value?.addVFNode(vfNode)
}

async function onSave() {
  const vfNodes = canvasRef.value?.getCurrentNodes()
  await save(vfNodes)
}

function goBack() {
  if (task.value) router.push(`/projects/${task.value.projectId}`)
  else router.back()
}

onMounted(load)
</script>

<style scoped>
.editor-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 72px); /* subtract topbar */
  margin: -22px; /* cancel main-panel padding */
  background: #1a1b23;
}

.editor-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  height: 52px;
  background: #1e1f2e;
  border-bottom: 1px solid #2d2e3d;
  flex-shrink: 0;
}

.editor-breadcrumb {
  flex-shrink: 0;
}

:deep(.editor-breadcrumb .el-breadcrumb__item .el-breadcrumb__inner) {
  color: #64748b !important;
  font-size: 12px;
}

:deep(.editor-breadcrumb .el-breadcrumb__item .el-breadcrumb__inner.is-link:hover) {
  color: #94a3b8 !important;
}

:deep(.editor-breadcrumb .el-breadcrumb__separator) { color: #3a3b4e !important; }

.editor-title-input {
  flex: 1;
  max-width: 280px;
}

:deep(.editor-title-input .el-input__wrapper) {
  background: #252633 !important;
  box-shadow: none !important;
  border: 1px solid #3a3b4e !important;
}

:deep(.editor-title-input .el-input__inner) {
  color: #e2e8f0 !important;
  font-weight: 600;
  font-size: 13px;
}

:deep(.editor-bar .el-button) {
  background: #252633;
  border-color: #3a3b4e;
  color: #c4cad4;
}

:deep(.editor-bar .el-button:hover) {
  background: #2d2e3d;
  border-color: #4a4b5e;
  color: #e2e8f0;
}

:deep(.editor-bar .el-button--primary) {
  background: #0d9488 !important;
  border-color: #0d9488 !important;
  color: #fff !important;
}

:deep(.editor-bar .el-button--primary:hover) {
  background: #0f766e !important;
  border-color: #0f766e !important;
}

.editor-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* Pipeline input list */
.pi-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.pi-info { flex: 1; min-width: 0; }

.pi-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.pi-label {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}
</style>
