<template>
  <div class="infra-providers">
    <div class="page-header">
      <div><h2 class="page-title">{{ inft('providersTitle') }}</h2><p class="page-desc">{{ inft('providersDesc') }}</p></div>
      <el-button @click="loadData">{{ inft('refresh') }}</el-button>
    </div>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input v-model="keyword" clearable :placeholder="inft('keywordPlaceholder')" style="width:320px" @keyup.enter="loadData" />
        <el-button type="primary" @click="loadData">{{ inft('search') }}</el-button>
        <el-button @click="resetQuery">{{ inft('reset') }}</el-button>
      </div>
      <el-table v-loading="loading" :data="filteredConnections" size="small">
        <el-table-column prop="name" :label="inft('connectionName')" min-width="160" />
        <el-table-column prop="uid" :label="inft('uidCol')" min-width="180" show-overflow-tooltip />
        <el-table-column prop="providerType" :label="inft('typeCol')" width="130" />
        <el-table-column prop="endpoint" :label="inft('endpointCol')" min-width="200" show-overflow-tooltip />
        <el-table-column :label="inft('statusCol')" width="110">
          <template #default="{ row }"><el-tag :type="statusTag(row.status).type" size="small">{{ statusTag(row.status).label }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="version" :label="inft('versionCol')" width="110" />
        <el-table-column prop="secretRefUid" :label="inft('secretRefCol')" min-width="180" show-overflow-tooltip />
        <el-table-column prop="updatedAt" :label="inft('updatedAt')" width="170" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listInfraProviderConnections } from '../../api/infra'
import { inft } from '../../utils/infra-i18n'

const connections = ref([])
const keyword = ref('')
const loading = ref(false)

const filteredConnections = computed(() => {
  const needle = keyword.value.trim().toLowerCase()
  if (!needle) return connections.value
  return connections.value.filter((row) => [row.name, row.uid, row.endpoint].some((field) => String(field || '').toLowerCase().includes(needle)))
})

function statusTag(status) {
  const map = { active: { key: 'statusActive', type: 'success' }, inactive: { key: 'statusInactive', type: 'info' } }
  const entry = map[status] || { key: 'statusUnknown', type: 'info' }
  return { label: inft(entry.key), type: entry.type }
}

function resetQuery() {
  keyword.value = ''
  loadData()
}

async function loadData() {
  loading.value = true
  try {
    const response = await listInfraProviderConnections()
    connections.value = response?.data?.items || []
  } catch (error) {
    ElMessage.error(inft('loadFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-title { margin: 0 0 4px; font-size: 20px; }
.page-desc { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
</style>
