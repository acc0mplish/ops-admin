<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { queryK8sClusterList } from '../../api/k8s'
import { kt } from '../../utils/k8s-extra-i18n'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'

const loading = ref(false)
const router = useRouter()
const cliGuideVisible = ref(false)
const clusterList = ref([])
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()
const selectedEnv = ref('')
const filteredClusters = computed(() => {
  return clusterList.value.filter((item) => {
    if (selectedEnv.value && item.env !== selectedEnv.value) return false
    return true
  })
})

// H1 — 클러스터 등록·편집·삭제는 register-k8s CLI로 이전됐다(§14.2-2 사용자 확정,
// plan §3.2 H1). v1 CRUD 라우트는 H2에서 삭제되고 읽기(list)는 v1 존치다.

function clusterStatusText(status) {
  const map = {
    running: 'k8sStatusRunning',
    warning: 'k8sStatusWarning',
    offline: 'k8sStatusOffline'
  }
  return kt(map[status] || 'k8sStatusWarning')
}

async function loadClusters() {
  loading.value = true
  try {
    clusterList.value = await queryK8sClusterList()
  } finally {
    loading.value = false
  }
}

function tagType(status) {
  switch (status) {
    case 'running':
      return 'success'
    case 'warning':
      return 'warning'
    case 'offline':
      return 'danger'
    default:
      return 'info'
  }
}

function openDetail(row) {
  router.push({ name: 'K8sClusterDetail', params: { id: row.id } })
}

onMounted(() => {
  loadClusters()
})
</script>

<template>
  <div class="cluster-manage-page">
    <section class="page-header">
      <div>
        <h2>{{ kt('k8sManageTitle') }}</h2>
        <p>{{ kt('k8sManageDesc') }}</p>
      </div>
      <div class="header-actions">
        <el-select v-model="selectedEnv" clearable :placeholder="kt('allEnvironments')" style="width: 160px">
          <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
        </el-select>
        <el-button :icon="Refresh" @click="loadClusters">{{ kt('k8sRefresh') }}</el-button>
        <el-button type="primary" @click="cliGuideVisible = true">{{ kt('k8sNewCluster') }}</el-button>
      </div>
    </section>

    <section v-loading="loading" class="table-panel">
      <el-table :data="filteredClusters" class="cluster-table">
        <el-table-column :label="kt('k8sClusterName')" min-width="180"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">{{ row.name }}</el-button></template></el-table-column>
        <el-table-column label="Environment" width="120">
          <template #default="{ row }">
            <el-tag effect="plain">{{ environmentName(row.env) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="kt('k8sStatus')" width="120">
          <template #default="{ row }">
            <el-tag :type="tagType(row.status)" effect="light">{{ clusterStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="apiServer" :label="kt('k8sApiServer')" min-width="260" />
        <el-table-column prop="version" :label="kt('k8sVersion')" width="140" />
        <el-table-column prop="nodeCount" :label="kt('k8sNodeCount')" width="100" />
        <el-table-column label="Monitoring Datasource" min-width="160">
          <template #default="{ row }">{{ row.monitorDatasourceName || kt('unboundLabel') }}</template>
        </el-table-column>
        <el-table-column :label="kt('accessModeLabel')" min-width="150">
          <template #default="{ row }">
            <span v-if="row.connectionMode === 'gateway'">Gateway: {{ row.gatewayName || row.gatewayId || '-' }}</span>
            <span v-else>{{ kt('directConnection') }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="kt('k8sDescription')" min-width="180" show-overflow-tooltip />
      </el-table>

      <el-empty v-if="!loading && !filteredClusters.length" :description="kt('k8sNoClustersRecorded')" />
    </section>

    <el-dialog v-model="cliGuideVisible" :title="kt('k8sNewCluster')" width="760px" destroy-on-close>
      <el-alert type="info" :closable="false" show-icon :title="kt('k8sClusterCRUDMovedToCLI')">
        <p>{{ kt('k8sClusterCRUDMovedToCLIDesc') }}</p>
      </el-alert>
      <pre class="cli-command">{{ kt('k8sRegisterCommandExample') }}</pre>
      <p class="cli-note">{{ kt('k8sRegisterKubeconfigSealedHint') }}</p>
    </el-dialog>
  </div>
</template>

<style scoped>
.cluster-manage-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cli-command {
  margin: 14px 0 8px;
  padding: 12px 14px;
  border: 1px solid #dbe4f5;
  border-radius: 6px;
  background: #f7f9fd;
  color: #0f172a;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
.cli-note { margin: 0; color: #6b7a93; font-size: 12px; }

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 20px;
  border: 1px solid #dbe4f5;
  border-radius: 8px;
  background: #f7f9fd;
}

.page-header h2 {
  margin: 0;
  font-size: 22px;
  color: #0f172a;
}

.page-header p {
  margin: 8px 0 0;
  color: #6b7a93;
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.table-panel {
  padding: 16px;
  border: 1px solid #e1e8f5;
  border-radius: 8px;
  background: #fff;
}

.cluster-table {
  width: 100%;
}

@media (max-width: 860px) {
  .page-header {
    flex-direction: column;
  }
}
</style>
