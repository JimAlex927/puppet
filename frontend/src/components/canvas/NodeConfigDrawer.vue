<template>
  <transition name="drawer-slide">
    <div v-if="node" class="ncf">
      <!-- Header -->
      <div class="ncf-header">
        <div class="ncf-title-row">
          <el-input
            v-model="node.name"
            class="ncf-name-input"
            size="small"
            placeholder="节点名称"
          />
          <button class="ncf-close" @click="emit('close')">
            <el-icon :size="16"><Close /></el-icon>
          </button>
        </div>
        <div class="ncf-type">{{ node.type }}</div>
      </div>

      <div class="ncf-body">
        <!-- Flow control -->
        <div class="ncf-section">
          <div class="ncf-section-title">执行控制</div>
          <div class="ncf-field">
            <label class="ncf-label">超时 (秒)</label>
            <el-input-number v-model="node.timeoutSeconds" :min="0" size="small" style="width: 100%" />
          </div>
          <div class="ncf-field">
            <label class="ncf-label">失败重试次数</label>
            <el-input-number v-model="node.retryTimes" :min="0" :max="10" size="small" style="width: 100%" />
          </div>
          <div class="ncf-field ncf-field--inline">
            <label class="ncf-label">出错时继续执行</label>
            <el-switch v-model="node.continueOnError" size="small" />
          </div>
        </div>

        <!-- Node params -->
        <div v-if="visibleFields.length" class="ncf-section">
          <div class="ncf-section-title">参数配置</div>
          <div v-for="field in visibleFields" :key="field.name" class="ncf-field">
            <label class="ncf-label">{{ field.label }}</label>

            <el-input
              v-if="field.type === 'input'"
              v-model="node.params[field.name] as string"
              :show-password="field.secret"
              size="small"
            />
            <el-input
              v-else-if="field.type === 'textarea'"
              v-model="node.params[field.name] as string"
              type="textarea"
              :rows="field.name === 'script' ? 8 : 4"
              size="small"
              class="ncf-textarea"
            />
            <el-input-number
              v-else-if="field.type === 'number'"
              v-model="node.params[field.name] as number"
              :min="0"
              size="small"
              style="width: 100%"
            />
            <el-select
              v-else-if="field.type === 'select'"
              v-model="node.params[field.name]"
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
              v-model="node.params[field.name] as boolean"
              size="small"
            />
            <el-select
              v-else-if="field.type === 'credential'"
              v-model="node.params[field.name]"
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
            <div class="ncf-mono">{{ node.id }}</div>
          </div>
          <div class="ncf-field">
            <label class="ncf-label">成功出口 (next)</label>
            <div class="ncf-mono ncf-mono--teal">{{ node.nextNodeId || '— 未连接' }}</div>
          </div>
          <div class="ncf-field">
            <label class="ncf-label">失败出口 (fallback)</label>
            <div class="ncf-mono ncf-mono--red">{{ node.fallbackNodeId || '— 未连接' }}</div>
          </div>
        </div>
      </div>

      <!-- Footer save -->
      <div class="ncf-footer">
        <button class="ncf-save-btn" :disabled="saving" @click="onSave">
          <el-icon v-if="saving" class="is-loading" :size="13"><Loading /></el-icon>
          <el-icon v-else :size="13"><Check /></el-icon>
          {{ saving ? '保存中…' : '保存节点' }}
        </button>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
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

const visibleFields = computed<NodeField[]>(() => {
  if (!props.metadata || !props.node) return []
  return props.metadata.fields.filter((f) => {
    if (!f.showWhen) return true
    return props.node!.params[f.showWhen.field] === f.showWhen.equals
  })
})

async function onSave() {
  saving.value = true
  try {
    if (props.onSavePipeline) {
      await props.onSavePipeline()
    }
    emit('save')
    ElMessage.success('节点已保存')
  } catch (e: any) {
    ElMessage.error(e?.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.ncf {
  width: 300px;
  flex-shrink: 0;
  background: #1e1f2e;
  border-left: 1px solid #2d2e3d;
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
  margin-bottom: 6px;
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
  font-size: 13px;
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
  overflow-y: auto;
  padding-bottom: 24px;
}

.ncf-body::-webkit-scrollbar { width: 4px; }
.ncf-body::-webkit-scrollbar-thumb { background: #2d2e3d; border-radius: 2px; }

.ncf-section {
  padding: 12px 14px 0;
}

.ncf-section + .ncf-section {
  border-top: 1px solid #2d2e3d;
  margin-top: 12px;
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
  padding: 10px 14px;
  border-top: 1px solid #2d2e3d;
  flex-shrink: 0;
}

.ncf-save-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 7px 0;
  background: #2dd4bf;
  color: #0f172a;
  border: none;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
}

.ncf-save-btn:hover:not(:disabled) { background: #5eead4; }
.ncf-save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* Transition */
.drawer-slide-enter-active,
.drawer-slide-leave-active { transition: width 0.2s ease, opacity 0.2s ease; }
.drawer-slide-enter-from,
.drawer-slide-leave-to { width: 0; opacity: 0; }
</style>
