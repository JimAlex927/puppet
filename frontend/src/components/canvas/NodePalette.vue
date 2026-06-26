<template>
  <aside class="palette">
    <div class="palette-search">
      <el-input
        v-model="query"
        placeholder="搜索节点…"
        :prefix-icon="Search"
        clearable
        size="small"
        class="palette-input"
      />
    </div>

    <div class="palette-body">
      <template v-if="query">
        <div class="palette-section-title">搜索结果</div>
        <div
          v-for="item in filtered"
          :key="item.type"
          class="palette-item"
          draggable="true"
          @dragstart="onDragStart($event, item)"
        >
          <div class="palette-item-icon" :style="{ background: categoryColor(item.category) + '22', color: categoryColor(item.category) }">
            <el-icon :size="13"><component :is="categoryIcon(item.category)" /></el-icon>
          </div>
          <div class="palette-item-text">
            <div class="palette-item-name">{{ item.name }}</div>
            <div class="palette-item-desc">{{ item.description }}</div>
          </div>
        </div>
        <div v-if="!filtered.length" class="palette-empty">无匹配节点</div>
      </template>

      <template v-else>
        <div v-for="[cat, items] in grouped" :key="cat" class="palette-group">
          <button class="palette-section-title palette-section-btn" @click="toggle(cat)">
            <span>{{ CAT_LABELS[cat] ?? cat }}</span>
            <el-icon :size="12" :class="open.has(cat) ? '' : 'is-collapsed'"><ArrowDown /></el-icon>
          </button>
          <template v-if="open.has(cat)">
            <div
              v-for="item in items"
              :key="item.type"
              class="palette-item"
              draggable="true"
              @dragstart="onDragStart($event, item)"
            >
              <div class="palette-item-icon" :style="{ background: categoryColor(item.category) + '22', color: categoryColor(item.category) }">
                <el-icon :size="13"><component :is="categoryIcon(item.category)" /></el-icon>
              </div>
              <div class="palette-item-text">
                <div class="palette-item-name">{{ item.name }}</div>
                <div class="palette-item-desc">{{ item.description }}</div>
              </div>
            </div>
          </template>
        </div>
      </template>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ArrowDown, Clock, Connection, Document, Monitor, Operation, Search } from '@element-plus/icons-vue'
import type { NodeMetadata } from '@/types'

const props = defineProps<{ nodeTypes: NodeMetadata[] }>()

const query = ref('')
const open = reactive(new Set<string>(['process', 'script']))

const CAT_LABELS: Record<string, string> = {
  process: '进程',
  script:  '脚本',
  http:    'HTTP',
  sleep:   '延迟',
  git:     'Git',
}

const CATEGORY_COLOR: Record<string, string> = {
  process: '#2dd4bf',
  script:  '#f59e0b',
  http:    '#6366f1',
  sleep:   '#94a3b8',
  git:     '#f97316',
  default: '#64748b',
}

const CATEGORY_ICON: Record<string, unknown> = {
  process: Monitor,
  script:  Document,
  http:    Connection,
  sleep:   Clock,
  default: Operation,
}

function categoryColor(cat: string) { return CATEGORY_COLOR[cat] ?? CATEGORY_COLOR.default }
function categoryIcon(cat: string)  { return CATEGORY_ICON[cat]  ?? CATEGORY_ICON.default }

const filtered = computed(() =>
  props.nodeTypes.filter(n =>
    n.name.toLowerCase().includes(query.value.toLowerCase()) ||
    n.type.toLowerCase().includes(query.value.toLowerCase()),
  ),
)

const grouped = computed<Map<string, NodeMetadata[]>>(() => {
  const map = new Map<string, NodeMetadata[]>()
  for (const item of props.nodeTypes) {
    const cat = item.category || 'default'
    if (!map.has(cat)) map.set(cat, [])
    map.get(cat)!.push(item)
  }
  return map
})

function toggle(cat: string) {
  open.has(cat) ? open.delete(cat) : open.add(cat)
}

function onDragStart(e: DragEvent, meta: NodeMetadata) {
  e.dataTransfer!.setData('application/puppet-node', JSON.stringify(meta))
  e.dataTransfer!.effectAllowed = 'copy'
}
</script>

<style scoped>
.palette {
  width: 220px;
  flex-shrink: 0;
  background: #1e1f2e;
  border-right: 1px solid #2d2e3d;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.palette-search {
  padding: 10px;
  border-bottom: 1px solid #2d2e3d;
}

:deep(.palette-input .el-input__wrapper) {
  background: #252633 !important;
  box-shadow: none !important;
  border: 1px solid #3a3b4e !important;
}

:deep(.palette-input .el-input__inner) {
  color: #c4cad4 !important;
  font-size: 12px;
}

:deep(.palette-input .el-input__prefix) { color: #64748b; }

.palette-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0 16px;
}

.palette-body::-webkit-scrollbar { width: 4px; }
.palette-body::-webkit-scrollbar-thumb { background: #2d2e3d; border-radius: 2px; }

.palette-section-title {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: #64748b;
  text-transform: uppercase;
  padding: 10px 12px 4px;
  display: block;
}

.palette-section-btn {
  width: 100%;
  background: none;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  padding: 8px 12px 4px;
}

.palette-section-btn .el-icon {
  color: #64748b;
  transition: transform 0.2s;
}

.palette-section-btn .el-icon.is-collapsed {
  transform: rotate(-90deg);
}

.palette-item {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 10px;
  margin: 1px 6px;
  border-radius: 6px;
  cursor: grab;
  transition: background 0.1s;
}

.palette-item:hover {
  background: #252633;
}

.palette-item:active {
  cursor: grabbing;
}

.palette-item-icon {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.palette-item-name {
  font-size: 12px;
  font-weight: 600;
  color: #c4cad4;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.palette-item-desc {
  font-size: 10px;
  color: #64748b;
  margin-top: 1px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}

.palette-empty {
  font-size: 12px;
  color: #64748b;
  text-align: center;
  padding: 24px 12px;
}
</style>
