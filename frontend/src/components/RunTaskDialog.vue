<template>
  <el-dialog v-model="visible" title="运行任务" width="500px" :close-on-click-modal="false">
    <div v-if="loading" class="run-dialog-loading">
      <el-icon class="is-loading" size="32"><Loading /></el-icon>
      <p>加载参数配置...</p>
    </div>
    <template v-else>
      <el-form label-position="top">
        <el-form-item
          v-for="input in inputs"
          :key="input.name"
          :label="input.label || input.name"
          :required="input.required"
        >
          <el-alert
            v-if="input.error"
            type="warning"
            :closable="false"
            show-icon
            style="margin-bottom: 8px; width: 100%"
          >
            <template #title>动态选项加载失败，请手动输入</template>
            <div style="font-size: 12px; color: #92400e">{{ input.error }}</div>
          </el-alert>

          <el-input
            v-if="input.type === 'string'"
            v-model="form[input.name] as string"
            :placeholder="input.default != null ? String(input.default) : ''"
          />
          <el-select
            v-else-if="input.type === 'select' && !input.error"
            v-model="form[input.name]"
            filterable
            style="width: 100%"
          >
            <el-option v-for="opt in input.options" :key="opt" :label="opt" :value="opt" />
          </el-select>
          <!-- Fallback to text input when select source failed -->
          <el-input
            v-else-if="input.type === 'select' && input.error"
            v-model="form[input.name] as string"
            placeholder="请输入值"
          />
          <el-switch v-else-if="input.type === 'boolean'" v-model="form[input.name] as boolean" />
          <el-input-number
            v-else-if="input.type === 'number'"
            v-model="form[input.name] as number"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
    </template>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="running" :disabled="loading" @click="confirm">
        运行
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import type { RunConfigInput, TaskRun } from '@/types'

const emit = defineEmits<{ success: [run: TaskRun] }>()

const visible = ref(false)
const loading = ref(false)
const running = ref(false)
const inputs = ref<RunConfigInput[]>([])
const form = reactive<Record<string, unknown>>({})
let activeTaskId = 0

async function open(taskId: number) {
  activeTaskId = taskId
  loading.value = true
  visible.value = true
  try {
    const config = await api.runConfig(activeTaskId)
    inputs.value = config.inputs
    for (const key of Object.keys(form)) delete form[key]
    for (const input of config.inputs) {
      if (input.type === 'boolean') {
        form[input.name] = input.default === true || input.default === 'true'
      } else if (input.type === 'number') {
        form[input.name] = typeof input.default === 'number' ? input.default : 0
      } else {
        form[input.name] = input.default ?? (input.options?.[0] ?? '')
      }
    }
  } catch (e: any) {
    ElMessage.error(e.message || '加载参数失败')
    visible.value = false
    return
  } finally {
    loading.value = false
  }

  if (inputs.value.length === 0) {
    visible.value = false
    try {
      await ElMessageBox.confirm('该任务无需配置参数，直接运行？', '确认运行', { type: 'info' })
    } catch {
      return
    }
    await doRun()
  }
}

async function confirm() {
  for (const input of inputs.value) {
    if (input.type === 'boolean' || input.type === 'number') continue
    const val = String(form[input.name] ?? '').trim()
    if (input.required && !val) {
      ElMessage.warning(`请填写「${input.label || input.name}」`)
      return
    }
    // Only validate against options when source did not error and options are available.
    if (input.type === 'select' && !input.error && input.options?.length && val && !input.options.includes(val)) {
      ElMessage.warning(`「${input.label || input.name}」的值不在可选范围内`)
      return
    }
  }
  await doRun()
}

async function doRun() {
  running.value = true
  try {
    const run = await api.runTask(activeTaskId, { ...form })
    visible.value = false
    ElMessage.success(`Run #${run.id} 已启动`)
    emit('success', run)
  } catch (e: any) {
    ElMessage.error(e.message || '启动失败')
  } finally {
    running.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
.run-dialog-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 32px 0;
  color: #64748b;
  gap: 12px;
}
</style>
