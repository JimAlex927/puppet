<template>
  <div>
    <div class="page-actions">
      <div>
        <h2 style="margin: 0">共享文件</h2>
        <p class="muted">局域网内登录用户可上传、下载和管理文件</p>
      </div>
      <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
    </div>

    <div class="shared-layout">
      <section class="panel upload-panel">
        <div class="section-title">上传</div>
        <div ref="uploaderTarget" class="uppy-target"></div>
      </section>

      <section class="panel files-panel">
        <div class="section-title">文件列表</div>
        <el-table v-loading="loading" :data="files" style="width: 100%">
          <el-table-column prop="name" label="文件名" min-width="260" show-overflow-tooltip />
          <el-table-column label="大小" width="120">
            <template #default="{ row }">{{ fmtSize(row.size) }}</template>
          </el-table-column>
          <el-table-column prop="uploadedBy" label="上传者" width="140" />
          <el-table-column label="上传时间" width="180">
            <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link :icon="Download" @click="download(row)">下载</el-button>
              <el-button size="small" link :icon="Link" @click="openShare(row)">分享</el-button>
              <el-button size="small" link type="danger" :icon="Delete" @click="remove(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && !files.length" description="还没有共享文件" :image-size="72" />
      </section>
    </div>

    <el-dialog v-model="shareDialogVisible" title="创建分享链接" width="420px" @closed="resetShare">
      <el-form label-position="top">
        <el-form-item label="文件">
          <el-input :model-value="shareFile?.name ?? ''" disabled />
        </el-form-item>
        <el-form-item label="有效期">
          <el-select v-model="shareDurationPreset" style="width: 100%">
            <el-option label="1 小时" :value="60" />
            <el-option label="1 天" :value="1440" />
            <el-option label="7 天" :value="10080" />
            <el-option label="30 天" :value="43200" />
            <el-option label="永久有效" :value="0" />
            <el-option label="自定义分钟数" :value="-1" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="shareDurationPreset === -1" label="自定义有效期（分钟）">
          <el-input-number v-model="customShareMinutes" :min="1" :max="525600" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="shareUrl" label="分享链接">
          <el-input v-model="shareUrl" readonly>
            <template #append>
              <el-button @click="copyShareUrl">复制</el-button>
            </template>
          </el-input>
          <div class="muted share-expiry">{{ shareExpiryText }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="shareDialogVisible = false">关闭</el-button>
        <el-button type="primary" :loading="creatingShare" @click="createShare">生成链接</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Uppy from '@uppy/core'
import Dashboard from '@uppy/dashboard'
import Tus from '@uppy/tus'
import '@uppy/core/css/style.min.css'
import '@uppy/dashboard/css/style.min.css'
import { Delete, Download, Link, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import type { SharedFile } from '@/types'
import { fmtDate } from '@/utils/format'

const files = ref<SharedFile[]>([])
const loading = ref(false)
const uploaderTarget = ref<HTMLElement>()
const shareDialogVisible = ref(false)
const creatingShare = ref(false)
const shareFile = ref<SharedFile>()
const shareDurationPreset = ref(1440)
const customShareMinutes = ref(60)
const shareUrl = ref('')
const shareExpiryText = ref('')
let uppy: Uppy | undefined

async function load() {
  loading.value = true
  try {
    files.value = await api.sharedFiles()
  } finally {
    loading.value = false
  }
}

function initUploader() {
  const token = localStorage.getItem('puppet_token') || ''
  uppy = new Uppy({
    autoProceed: false,
    restrictions: {
      maxFileSize: 10 * 1024 * 1024 * 1024,
    },
  })
    .use(Dashboard, {
      target: uploaderTarget.value,
      inline: true,
      height: 320,
      proudlyDisplayPoweredByUppy: false,
      hideProgressDetails: false,
      hideUploadButton: false,
    })
    .use(Tus, {
      endpoint: '/api/shared-file-uploads/',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      chunkSize: 8 * 1024 * 1024,
      limit: 3,
      retryDelays: [0, 1000, 3000, 5000],
    })

  uppy.on('file-added', (file) => {
    uppy?.setFileMeta(file.id, {
      filename: file.name,
      filetype: file.type || 'application/octet-stream',
    })
  })
  uppy.on('complete', async (result) => {
    const successful = result.successful ?? []
    if (successful.length > 0) {
      ElMessage.success(`已上传 ${successful.length} 个文件`)
      await load()
    }
  })
}

function download(file: SharedFile) {
  const link = document.createElement('a')
  link.href = api.sharedFileDownloadUrl(file.id)
  link.download = file.name
  document.body.appendChild(link)
  link.click()
  link.remove()
}

async function remove(file: SharedFile) {
  await ElMessageBox.confirm(`确认删除「${file.name}」？`, '删除共享文件', { type: 'warning' })
  await api.deleteSharedFile(file.id)
  ElMessage.success('已删除')
  await load()
}

function openShare(file: SharedFile) {
  shareFile.value = file
  shareDialogVisible.value = true
}

function resetShare() {
  shareFile.value = undefined
  shareDurationPreset.value = 1440
  customShareMinutes.value = 60
  shareUrl.value = ''
  shareExpiryText.value = ''
}

async function createShare() {
  if (!shareFile.value) return
  const minutes = shareDurationPreset.value === -1 ? customShareMinutes.value : shareDurationPreset.value
  creatingShare.value = true
  try {
    const share = await api.createSharedFileShare(shareFile.value.id, minutes)
    shareUrl.value = new URL(share.url, window.location.origin).href
    shareExpiryText.value = share.expiresAt ? `有效期至：${fmtDate(share.expiresAt)}` : '永久有效'
    await copyShareUrl()
  } finally {
    creatingShare.value = false
  }
}

async function copyShareUrl() {
  if (!shareUrl.value) return
  await navigator.clipboard.writeText(shareUrl.value)
  ElMessage.success('分享链接已复制')
}

function fmtSize(size: number) {
  if (!Number.isFinite(size) || size < 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

onMounted(async () => {
  initUploader()
  await load()
})

onBeforeUnmount(() => {
  uppy?.destroy()
})
</script>

<style scoped>
.shared-layout {
  display: grid;
  grid-template-columns: minmax(320px, 420px) minmax(0, 1fr);
  gap: 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 700;
  color: #1f2937;
  margin-bottom: 12px;
}

.upload-panel,
.files-panel {
  min-width: 0;
}

.uppy-target {
  min-height: 320px;
}

.share-expiry {
  margin-top: 6px;
  font-size: 12px;
}

:deep(.uppy-Dashboard-inner) {
  width: 100% !important;
  border-radius: 8px;
}

@media (max-width: 980px) {
  .shared-layout {
    grid-template-columns: 1fr;
  }
}
</style>
