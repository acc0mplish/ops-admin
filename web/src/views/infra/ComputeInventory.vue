<template>
  <div class="compute-inventory">
    <div class="page-header">
      <div><h2 class="page-title">{{ inft('computeTitle') }}</h2><p class="page-desc">{{ inft('computeDesc') }}</p></div>
      <el-button @click="loadData">{{ inft('refresh') }}</el-button>
    </div>

    <el-card shadow="never">
      <div class="toolbar">
        <span class="toolbar-label">{{ inft('kindFilterLabel') }}</span>
        <el-select v-model="kind" clearable :placeholder="inft('kindAll')" style="width:260px" @change="onKindChange">
          <el-option v-for="item in kindOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <span class="toolbar-total">{{ inft('paginationTotal', { count: total }) }}</span>
      </div>
      <el-table v-loading="loading" :data="resources" size="small" @row-click="openDetail">
        <el-table-column prop="displayName" :label="inft('resourceDisplayName')" min-width="170" show-overflow-tooltip />
        <el-table-column prop="kind" :label="inft('kindCol')" min-width="150" />
        <el-table-column prop="subtype" :label="inft('subtypeCol')" width="130" />
        <el-table-column prop="lifecycleState" :label="inft('lifecycleCol')" width="110" />
        <el-table-column prop="healthState" :label="inft('healthCol')" width="100" />
        <el-table-column prop="managedState" :label="inft('managedCol')" width="120" />
        <el-table-column prop="externalUrn" :label="inft('urnCol')" min-width="230" show-overflow-tooltip />
        <el-table-column prop="lastSeenAt" :label="inft('lastSeenCol')" width="170" />
      </el-table>
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        class="pager"
        @current-change="loadData"
      />
    </el-card>

    <el-drawer v-model="detailVisible" :title="inft('detailTitle')" size="55%">
      <template v-if="detail">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item :label="inft('resourceDisplayName')">{{ detail.resource.displayName }}</el-descriptions-item>
          <el-descriptions-item :label="inft('urnCol')">{{ detail.resource.externalUrn }}</el-descriptions-item>
          <el-descriptions-item :label="inft('kindCol')">{{ detail.resource.kind }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.observation" :label="inft('detailGeneration')">{{ detail.observation.generationUid }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.observation" :label="inft('detailObservedAt')">{{ detail.observation.observedAt }}</el-descriptions-item>
        </el-descriptions>
        <template v-if="detail.observation">
          <h4 class="json-title">{{ inft('detailNormalized') }}</h4>
          <pre class="json-block">{{ pretty(detail.observation.normalized) }}</pre>
          <h4 class="json-title">{{ inft('detailRaw') }}</h4>
          <pre class="json-block">{{ pretty(detail.observation.raw) }}</pre>
        </template>
        <el-button class="drawer-close" @click="detailVisible = false">{{ inft('close') }}</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getInfraResource, listInfraComputeResources } from '../../api/infra'
import { inft } from '../../utils/infra-i18n'

const kindOptions = ['compute.vm', 'compute.volume', 'compute.snapshot', 'compute.image']

const resources = ref([])
const kind = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const detailVisible = ref(false)
const detail = ref(null)

function onKindChange() {
  page.value = 1
  loadData()
}

function pretty(value) {
  return JSON.stringify(value ?? {}, null, 2)
}

async function loadData() {
  loading.value = true
  try {
    const params = { page: page.value, pageSize: pageSize.value }
    if (kind.value) params.kind = kind.value
    const response = await listInfraComputeResources(params)
    resources.value = response?.items || []
    total.value = response?.total || 0
  } catch (error) {
    ElMessage.error(inft('loadFailed'))
  } finally {
    loading.value = false
  }
}

async function openDetail(row) {
  try {
    const response = await getInfraResource(row.uid)
    detail.value = response || null
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(inft('loadFailed'))
  }
}

onMounted(loadData)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-title { margin: 0 0 4px; font-size: 20px; }
.page-desc { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.toolbar-label { color: var(--el-text-color-secondary); font-size: 13px; }
.toolbar-total { margin-left: auto; color: var(--el-text-color-secondary); font-size: 13px; }
.pager { margin-top: 12px; justify-content: flex-end; }
.json-title { margin: 14px 0 6px; font-size: 14px; }
.json-block { margin: 0; padding: 10px; background: var(--el-fill-color-light); border-radius: 6px; font-size: 12px; overflow: auto; max-height: 260px; }
.drawer-close { margin-top: 16px; }
</style>
