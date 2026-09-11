<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div v-if="node && draftNode" class="ncf-layer">
        <button class="ncf-backdrop" aria-label="关闭节点配置" @click="emit('close')" />
        <section class="ncf" role="dialog" aria-modal="true" aria-labelledby="ncf-title">
      <!-- Header -->
      <div class="ncf-header">
        <div class="ncf-title-row">
          <div class="ncf-heading">
            <span class="ncf-eyebrow">节点配置</span>
            <el-input
              id="ncf-title"
              v-model="draftNode.name"
              class="ncf-name-input"
              size="large"
              placeholder="节点名称"
            />
          </div>
          <button class="ncf-close" aria-label="关闭" @click="emit('close')">
            <el-icon :size="18"><Close /></el-icon>
          </button>
        </div>
        <div class="ncf-type">{{ draftNode.type }} · 修改后保存即可应用</div>
      </div>

      <div class="ncf-body" :class="{ 'ncf-body--single': !visibleFields.length }">
        <!-- Flow control -->
        <div class="ncf-section">
          <div class="ncf-section-title">执行控制</div>
          <div class="ncf-field">
            <label class="ncf-label">超时 (秒)</label>
            <el-input-number v-model="draftNode.timeoutSeconds" :min="0" size="small" style="width: 100%" />
          </div>
          <div class="ncf-field">
            <label class="ncf-label">失败重试次数</label>
            <el-input-number v-model="draftNode.retryTimes" :min="0" :max="10" size="small" style="width: 100%" />
          </div>
          <div class="ncf-field ncf-field--inline">
            <label class="ncf-label">出错时继续执行</label>
            <el-switch v-model="draftNode.continueOnError" size="small" />
          </div>
        </div>

        <!-- Node params -->
        <div v-if="visibleFields.length" class="ncf-section">
          <div class="ncf-section-title">参数配置</div>
          <div v-for="field in visibleFields" :key="field.name" class="ncf-field">
            <label class="ncf-label">
              {{ field.label }}
              <span
                v-if="supportsTemplate(field)"
                class="ncf-template-hint"
                title="运行前会展开 ${input.xxx}、${xxx} 和 ${node.nodeId.key}"
              >
                （可用 ${input.xxx}）
              </span>
            </label>

            <el-input
              v-if="field.type === 'input'"
              v-model="draftNode.params[field.name] as string"
              :show-password="field.secret"
              size="small"
            />
            <el-input
              v-else-if="field.type === 'textarea'"
              v-model="draftNode.params[field.name] as string"
              type="textarea"
              :rows="field.name === 'script' ? 14 : 6"
              size="small"
              class="ncf-textarea"
            />
            <el-input-number
              v-else-if="field.type === 'number'"
              v-model="draftNode.params[field.name] as number"
              :min="0"
              size="small"
              style="width: 100%"
            />
            <el-select
              v-else-if="field.type === 'select'"
              v-model="draftNode.params[field.name]"
              size="small"
              style="width: 100%"
            >
              <el-option
                v-for="opt in field.options"
                :key="opt"
                :label="opt"
                :value="opt"
              />
            </el-select>
            <el-switch
              v-else-if="field.type === 'switch'"
              v-model="draftNode.params[field.name] as boolean"
              size="small"
            />
            <el-select
              v-else-if="field.type === 'credential'"
              v-model="draftNode.params[field.name]"
              size="small"
              style="width: 100%"
            >
              <el-option label="无需凭据" :value="0" />
              <el-option
                v-for="cred in credentials"
                :key="cred.id"
                :label="`${cred.name} (${cred.type})`"
                :value="cred.id"
              />
            </el-select>
          </div>
        </div>

        <!-- Node ID (read-only reference) -->
        <div class="ncf-section">
          <div class="ncf-section-title">节点信息</div>
          <div class="ncf-field">
            <label class="ncf-label">节点 ID</label>
            <div class="ncf-mono">{{ draftNode.id }}</div>
          </div>
          <div class="ncf-field">
            <label class="ncf-label">成功出口 (next)</label>
            <div class="ncf-mono ncf-mono--teal">{{ draftNode.nextNodeId || '— 未连接' }}</div>
          </div>
          <div class="ncf-field">
            <label class="ncf-label">失败出口 (fallback)</label>
            <div class="ncf-mono ncf-mono--red">{{ draftNode.fallbackNodeId || '— 未连接' }}</div>
          </div>
        </div>
      </div>

      <!-- Footer save -->
      <div class="ncf-footer">
        <button class="ncf-cancel-btn" @click="emit('close')">取消</button>
        <button class="ncf-save-btn" :disabled="saving" @click="onSave">
          <el-icon v-if="saving" class="is-loading" :size="13"><Loading /></el-icon>
          <el-icon v-else :size="13"><Check /></el-icon>
          {{ saving ? '保存中…' : '保存节点' }}
        </button>
      </div>
        </section>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Close, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { Credential, NodeField, NodeMetadata, PipelineNode } from '@/types'

const props = defineProps<{
  node?: PipelineNode
  metadata?: NodeMetadata
  credentials: Credential[]
  onSavePipeline?: () => Promise<void>
}>()

const emit = defineEmits<{ close: []; save: [] }>()

const saving = ref(false)
const draftNode = ref<PipelineNode>()

watch(
  () => props.node,
  (node) => {
    draftNode.value = node ? cloneNode(node) : undefined
  },
  { immediate: true },
)

const visibleFields = computed<NodeField[]>(() => {
  if (!props.metadata || !draftNode.value) return []
  return props.metadata.fields.filter((f) => {
    if (!f.showWhen) return true
    return draftNode.value!.params[f.showWhen.field] === f.showWhen.equals
  })
})

async function onSave() {
  if (!props.node || !draftNode.value) return
  saving.value = true
  const original = cloneNode(props.node)
  Object.assign(props.node, cloneNode(draftNode.value))
  try {
    if (props.onSavePipeline) {
      await props.onSavePipeline()
    }
    emit('save')
    ElMessage.success('节点已保存')
  } catch (e: any) {
    Object.assign(props.node, original)
    ElMessage.error(e?.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}

function supportsTemplate(field: NodeField) {
  return field.type === 'input' || field.type === 'textarea'
}

function cloneNode(node: PipelineNode): PipelineNode {
  return JSON.parse(JSON.stringify(node)) as PipelineNode
}
</script>

<style scoped>
.ncf-layer {
  position: fixed;
  inset: 0;
  z-index: 2000;
  display: grid;
  place-items: center;
  padding: 36px;
}

.ncf-backdrop {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  border: 0;
  background: rgba(8, 11, 20, 0.7);
  cursor: default;
}

.ncf {
  position: relative;
  width: min(860px, 100%);
  max-height: min(760px, calc(100vh - 72px));
  background: #1e1f2e;
  border: 1px solid #3a3b4e;
  border-radius: 12px;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(45, 212, 191, 0.06);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ncf-header {
  padding: 14px 14px 10px;
  border-bottom: 1px solid #2d2e3d;
  flex-shrink: 0;
}

.ncf-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.ncf-heading {
  flex: 1;
  min-width: 0;
}

.ncf-eyebrow {
  display: block;
  margin-bottom: 5px;
  color: #2dd4bf;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

:deep(.ncf-name-input .el-input__wrapper) {
  background: #252633 !important;
  box-shadow: none !important;
  border: 1px solid #3a3b4e !important;
  flex: 1;
}

:deep(.ncf-name-input .el-input__inner) {
  color: #e2e8f0 !important;
  font-weight: 600;
  font-size: 16px;
}

.ncf-close {
  background: none;
  border: none;
  cursor: pointer;
  color: #64748b;
  padding: 4px;
  border-radius: 4px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  transition: color 0.15s, background 0.15s;
}

.ncf-close:hover { color: #e2e8f0; background: #2d2e3d; }

.ncf-type {
  font-size: 11px;
  color: #64748b;
}

.ncf-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(220px, 0.85fr) minmax(0, 1.5fr);
  align-content: start;
  gap: 0 26px;
  overflow-y: auto;
  padding: 4px 22px 26px;
}

.ncf-body--single {
  display: block;
}

.ncf-body::-webkit-scrollbar { width: 4px; }
.ncf-body::-webkit-scrollbar-thumb { background: #2d2e3d; border-radius: 2px; }

.ncf-section {
  padding: 16px 0 0;
}

.ncf-section + .ncf-section {
  border-top: 1px solid #2d2e3d;
  margin-top: 16px;
}

.ncf-section--params {
  padding-top: 16px;
}

.ncf-section-title {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #64748b;
  margin-bottom: 10px;
}

.ncf-field {
  margin-bottom: 10px;
}

.ncf-field--inline {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.ncf-label {
  display: block;
  font-size: 11px;
  color: #8892a4;
  margin-bottom: 5px;
}

.ncf-template-hint {
  color: #64748b;
  font-size: 10px;
}

.ncf-field--inline .ncf-label { margin-bottom: 0; }

/* Override Element Plus inputs for dark theme */
:deep(.ncf-field .el-input__wrapper),
:deep(.ncf-field .el-textarea__inner),
:deep(.ncf-field .el-select .el-input__wrapper) {
  background: #252633 !important;
  box-shadow: none !important;
  border: 1px solid #3a3b4e !important;
}

:deep(.ncf-field .el-input__inner),
:deep(.ncf-field .el-textarea__inner) {
  color: #e2e8f0 !important;
  font-size: 12px;
}

:deep(.ncf-field .el-input-number__decrease),
:deep(.ncf-field .el-input-number__increase) {
  background: #2d2e3d !important;
  border-color: #3a3b4e !important;
  color: #94a3b8;
}

:deep(.ncf-textarea .el-textarea__inner) {
  font-family: 'Cascadia Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
}

.ncf-mono {
  font-size: 11px;
  font-family: 'Cascadia Mono', Consolas, monospace;
  color: #94a3b8;
  background: #252633;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid #3a3b4e;
  word-break: break-all;
}

.ncf-mono--teal { color: #2dd4bf; }
.ncf-mono--red  { color: #f87171; }

.ncf-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 22px;
  border-top: 1px solid #2d2e3d;
  flex-shrink: 0;
}

.ncf-cancel-btn,
.ncf-save-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 112px;
  padding: 9px 18px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, opacity 0.15s;
}

.ncf-cancel-btn {
  background: #252633;
  color: #c4cad4;
  border: 1px solid #3a3b4e;
}

.ncf-cancel-btn:hover {
  background: #2d2e3d;
  color: #e2e8f0;
}

.ncf-save-btn {
  background: #2dd4bf;
  color: #0f172a;
  border: none;
}

.ncf-save-btn:hover:not(:disabled) { background: #5eead4; }
.ncf-save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.modal-fade-enter-active,
.modal-fade-leave-active { transition: opacity 0.18s ease, transform 0.18s ease; }
.modal-fade-enter-from,
.modal-fade-leave-to { opacity: 0; transform: scale(0.98); }

@media (max-width: 720px) {
  .ncf-layer { padding: 16px; }
  .ncf { max-height: calc(100vh - 32px); }
  .ncf-body {
    display: block;
    padding-left: 16px;
    padding-right: 16px;
  }
  .ncf-footer { padding: 12px 16px; }
  .ncf-cancel-btn,
  .ncf-save-btn { flex: 1; }
}
</style>
