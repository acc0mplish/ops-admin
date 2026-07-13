<script setup>
import { computed, nextTick, ref, watch } from 'vue'

const props = defineProps({
  hostOptions: { type: Array, default: () => [] },
  groupOptions: { type: Array, default: () => [] },
  hostIds: { type: Array, default: () => [] },
  groupId: { type: [Number, String], default: undefined }
})

const emit = defineEmits(['update:hostIds', 'update:groupId'])

const dialogVisible = ref(false)
const tableRef = ref()
const tempHostIds = ref([])
const environmentFilter = ref('')
const tagFilter = ref('')

const flatGroups = computed(() => flattenGroups(props.groupOptions))
const environmentOptions = computed(() => [...new Set(props.hostOptions.map((item) => item.environment).filter(Boolean))])
const tagOptions = computed(() => [...new Set(props.hostOptions.flatMap((item) => item.tags || []))])
const filteredHostOptions = computed(() => props.hostOptions.filter((host) => {
  if (environmentFilter.value && host.environment !== environmentFilter.value) return false
  if (tagFilter.value && !(host.tags || []).includes(tagFilter.value)) return false
  return true
}))

const selectedGroupHosts = computed(() => {
  const groupId = Number(props.groupId || 0)
  if (!groupId) return []
  return props.hostOptions.filter((host) => {
    if (Number(host.groupId) === groupId) return true
    return (host.hostGroups || []).some((group) => Number(group.id) === groupId)
  })
})

const selectedHosts = computed(() => {
  if (Number(props.groupId || 0)) {
    return selectedGroupHosts.value
  }
  const selectedSet = new Set((props.hostIds || []).map((id) => Number(id)))
  return props.hostOptions.filter((host) => selectedSet.has(Number(host.id)))
})

watch(dialogVisible, async (visible) => {
  if (!visible) return
  tempHostIds.value = [...(props.hostIds || [])]
  await nextTick()
  const table = tableRef.value
  if (!table) return
  table.clearSelection()
  for (const row of filteredHostOptions.value) {
    if (tempHostIds.value.includes(row.id)) {
      table.toggleRowSelection(row, true)
    }
  }
})

watch([environmentFilter, tagFilter], async () => {
  if (!dialogVisible.value) return
  await nextTick()
  const table = tableRef.value
  if (!table) return
  table.clearSelection()
  const selectedSet = new Set(tempHostIds.value.map(Number))
  for (const row of filteredHostOptions.value) {
    if (selectedSet.has(Number(row.id))) table.toggleRowSelection(row, true)
  }
})

function flattenGroups(nodes = [], prefix = '') {
  return nodes.flatMap((item) => {
    const label = prefix ? `${prefix} / ${item.name}` : item.name
    return [{ label, value: item.id }, ...flattenGroups(item.children || [], label)]
  })
}

function openHostDialog() {
  dialogVisible.value = true
}

function handleHostSelectionChange(rows) {
  const visibleIds = new Set(filteredHostOptions.value.map((item) => Number(item.id)))
  const hiddenSelections = tempHostIds.value.filter((id) => !visibleIds.has(Number(id)))
  tempHostIds.value = [...hiddenSelections, ...rows.map((item) => item.id)]
}

function confirmHosts() {
  emit('update:hostIds', tempHostIds.value)
  emit('update:groupId', undefined)
  dialogVisible.value = false
}

function clearHosts() {
  emit('update:hostIds', [])
}

function handleGroupChange(value) {
  emit('update:groupId', value || undefined)
  emit('update:hostIds', [])
}

function clearGroup() {
  emit('update:groupId', undefined)
}
</script>

<template>
  <div class="target-selector">
    <el-form-item label="目标主机">
      <div class="target-field">
        <el-button @click="openHostDialog" :disabled="Boolean(groupId)">选择主机</el-button>
        <el-button v-if="hostIds?.length" link type="danger" @click="clearHosts">清空</el-button>
      </div>
    </el-form-item>

    <el-form-item label="主机组">
      <div class="target-field">
        <el-select
          :model-value="groupId"
          clearable
          filterable
          placeholder="选择主机组"
          style="width: 100%"
          :disabled="Boolean(hostIds?.length)"
          @change="handleGroupChange"
          @clear="clearGroup"
        >
          <el-option v-for="item in flatGroups" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
      </div>
    </el-form-item>

    <el-form-item label="目标预览">
      <div class="target-tags">
        <el-empty v-if="!selectedHosts.length" :image-size="60" description="当前还没有选择目标主机" />
        <el-tag v-for="item in selectedHosts" :key="item.id" class="target-tag" effect="light">
          {{ item.hostName }} ({{ item.sshIp || item.privateIp || '-' }})
        </el-tag>
      </div>
    </el-form-item>

    <el-dialog v-model="dialogVisible" title="选择目标主机" width="860px">
      <div class="target-filter-bar">
        <el-select v-model="environmentFilter" clearable placeholder="全部环境" style="width: 180px">
          <el-option v-for="env in environmentOptions" :key="env" :label="env" :value="env" />
        </el-select>
        <el-select v-model="tagFilter" clearable filterable placeholder="全部标签" style="width: 180px">
          <el-option v-for="tag in tagOptions" :key="tag" :label="tag" :value="tag" />
        </el-select>
        <span>命中 {{ filteredHostOptions.length }} 台主机</span>
      </div>
      <el-table ref="tableRef" :data="filteredHostOptions" border @selection-change="handleHostSelectionChange">
        <el-table-column type="selection" width="48" />
        <el-table-column prop="hostName" label="主机名称" min-width="180" />
        <el-table-column prop="sshIp" label="SSH IP" min-width="140">
          <template #default="{ row }">{{ row.sshIp || row.privateIp || '-' }}</template>
        </el-table-column>
        <el-table-column prop="group.name" label="主机组" min-width="140">
          <template #default="{ row }">{{ row.group?.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="environment" label="环境" width="100" />
        <el-table-column label="标签" min-width="150"><template #default="{ row }">{{ (row.tags || []).join('、') || '-' }}</template></el-table-column>
        <el-table-column prop="sshUser" label="SSH 用户" min-width="100" />
      </el-table>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmHosts">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.target-field {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.target-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  min-height: 44px;
  width: 100%;
}

.target-tag {
  max-width: 100%;
}
.target-filter-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; color: #7282a0; }
</style>
