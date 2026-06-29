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

const flatGroups = computed(() => flattenGroups(props.groupOptions))

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
  for (const row of props.hostOptions) {
    if (tempHostIds.value.includes(row.id)) {
      table.toggleRowSelection(row, true)
    }
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
  tempHostIds.value = rows.map((item) => item.id)
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
      <el-table ref="tableRef" :data="hostOptions" border @selection-change="handleHostSelectionChange">
        <el-table-column type="selection" width="48" />
        <el-table-column prop="hostName" label="主机名称" min-width="180" />
        <el-table-column prop="sshIp" label="SSH IP" min-width="140">
          <template #default="{ row }">{{ row.sshIp || row.privateIp || '-' }}</template>
        </el-table-column>
        <el-table-column prop="group.name" label="主机组" min-width="140">
          <template #default="{ row }">{{ row.group?.name || '-' }}</template>
        </el-table-column>
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
</style>
