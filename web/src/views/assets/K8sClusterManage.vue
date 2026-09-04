<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Refresh } from '@element-plus/icons-vue'
import {
  addK8sCluster,
  deleteK8sCluster,
  queryK8sClusterInfo,
  queryK8sClusterList,
  updateK8sCluster
} from '../../api/k8s'
import { queryAssetGatewayOptions } from '../../api/asset'
import { queryMonitorDatasourceOptions, saveMonitorDatasource } from '../../api/monitor'
import { t } from '../../utils/i18n'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'

const loading = ref(false)
const router = useRouter()
const submitting = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const clusterList = ref([])
const gatewayOptions = ref([])
const datasourceOptions = ref([])
const datasourceSaving = ref(false)
const datasourceDialogVisible = ref(false)
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()
const selectedEnv = ref('')
const filteredClusters = computed(() => {
  return clusterList.value.filter((item) => {
    if (selectedEnv.value && item.env !== selectedEnv.value) return false
    return true
  })
})

const form = reactive({
  id: undefined,
  name: '',
  env: 'test',
  description: '',
  kubeConfig: '',
  connectionMode: 'direct',
  gatewayId: undefined,
  monitorDatasourceId: undefined
})
const datasourceForm = reactive({ name: '', type: 'prometheus', url: '', authType: 'none', username: '', password: '', token: '', env: 'test', status: 1, description: '' })

const dialogTitle = computed(() => (isEdit.value ? t('k8sEditCluster') : t('k8sCreateCluster')))

function clusterStatusText(status) {
  const map = {
    running: 'k8sStatusRunning',
    warning: 'k8sStatusWarning',
    offline: 'k8sStatusOffline'
  }
  return t(map[status] || 'k8sStatusWarning')
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    env: 'test',
    description: '',
    kubeConfig: '',
    connectionMode: 'direct',
    gatewayId: undefined,
    monitorDatasourceId: undefined
  })
}

async function loadClusters() {
  loading.value = true
  try {
    clusterList.value = await queryK8sClusterList()
  } finally {
    loading.value = false
  }
}

async function loadGateways() {
  gatewayOptions.value = await queryAssetGatewayOptions()
}

async function loadDatasourceOptions() {
  const options = await queryMonitorDatasourceOptions()
  datasourceOptions.value = (options || []).filter((item) => ['prometheus', 'victoriametrics'].includes(item.type))
}

function openDatasourceCreate() {
  Object.assign(datasourceForm, { name: '', type: 'prometheus', url: '', authType: 'none', username: '', password: '', token: '', env: form.env || 'test', status: 1, description: '' })
  datasourceDialogVisible.value = true
}

async function saveDatasource() {
  if (!datasourceForm.name.trim() || !datasourceForm.url.trim()) {
    ElMessage.warning('Datasource 이름과 주소를 입력하십시오.')
    return
  }
  datasourceSaving.value = true
  try {
    await saveMonitorDatasource(datasourceForm)
    await loadDatasourceOptions()
    const created = datasourceOptions.value.find((item) => item.name === datasourceForm.name.trim() && item.url === datasourceForm.url.trim().replace(/\/$/, ''))
    form.monitorDatasourceId = created?.id
    datasourceDialogVisible.value = false
    ElMessage.success('Datasource를 생성하고 현재 Cluster에 바인딩했습니다.')
  } finally {
    datasourceSaving.value = false
  }
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  const data = await queryK8sClusterInfo(row.id)
  isEdit.value = true
  Object.assign(form, {
    id: data.id,
    name: data.name,
    env: data.env || 'test',
    description: data.description || '',
    kubeConfig: data.kubeConfig || '',
    connectionMode: data.connectionMode || 'direct',
    gatewayId: data.gatewayId || undefined,
    monitorDatasourceId: data.monitorDatasourceId || undefined
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning(t('k8sPleaseInputClusterName'))
    return
  }
  if (!form.kubeConfig.trim()) {
    ElMessage.warning(t('k8sPleaseInputKubeconfig'))
    return
  }
  if (form.connectionMode === 'gateway' && !form.gatewayId) {
    ElMessage.warning('접속 Gateway를 선택하십시오.')
    return
  }

  submitting.value = true
  try {
    if (isEdit.value) {
      await updateK8sCluster({ ...form })
      ElMessage.success(t('k8sClusterUpdated'))
    } else {
      await addK8sCluster({ ...form })
      ElMessage.success(t('k8sClusterCreated'))
    }
    dialogVisible.value = false
    await loadClusters()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('k8sDeleteClusterConfirm', { name: row.name }), t('k8sPrompt'), { type: 'warning' })
  await deleteK8sCluster(row.id)
  ElMessage.success(t('k8sClusterDeleted'))
  await loadClusters()
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

onMounted(async () => {
  await loadGateways()
  await loadDatasourceOptions()
  await loadClusters()
})
</script>

<template>
  <div class="cluster-manage-page">
    <section class="page-header">
      <div>
        <h2>{{ t('k8sManageTitle') }}</h2>
        <p>{{ t('k8sManageDesc') }}</p>
      </div>
      <div class="header-actions">
        <el-select v-model="selectedEnv" clearable placeholder="전체 Environment" style="width: 160px">
          <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
        </el-select>
        <el-button :icon="Refresh" @click="loadClusters">{{ t('k8sRefresh') }}</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">{{ t('k8sNewCluster') }}</el-button>
      </div>
    </section>

    <section v-loading="loading" class="table-panel">
      <el-table :data="filteredClusters" class="cluster-table">
        <el-table-column :label="t('k8sClusterName')" min-width="180"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">{{ row.name }}</el-button></template></el-table-column>
        <el-table-column label="Environment" width="120">
          <template #default="{ row }">
            <el-tag effect="plain">{{ environmentName(row.env) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('k8sStatus')" width="120">
          <template #default="{ row }">
            <el-tag :type="tagType(row.status)" effect="light">{{ clusterStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="apiServer" :label="t('k8sApiServer')" min-width="260" />
        <el-table-column prop="version" :label="t('k8sVersion')" width="140" />
        <el-table-column prop="nodeCount" :label="t('k8sNodeCount')" width="100" />
        <el-table-column label="Monitoring Datasource" min-width="160">
          <template #default="{ row }">{{ row.monitorDatasourceName || '미바인딩' }}</template>
        </el-table-column>
        <el-table-column label="접속 방식" min-width="150">
          <template #default="{ row }">
            <span v-if="row.connectionMode === 'gateway'">Gateway: {{ row.gatewayName || row.gatewayId || '-' }}</span>
            <span v-else>직접 연결</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="t('k8sDescription')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="t('k8sOperations')" width="160" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <el-button link type="primary" :icon="Edit" @click="openEdit(row)">{{ t('k8sEdit') }}</el-button>
              <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">{{ t('k8sDelete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && !filteredClusters.length" :description="t('k8sNoClustersRecorded')" />
    </section>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="760px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item :label="t('k8sClusterName')" required>
          <el-input v-model="form.name" :placeholder="t('k8sClusterNamePlaceholder')" />
        </el-form-item>
        <el-form-item label="소속 Environment" required>
          <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" placeholder="Environment를 선택하십시오">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('k8sDescription')">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            :placeholder="t('k8sClusterDescriptionPlaceholder')"
          />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="연결 방식">
              <el-radio-group v-model="form.connectionMode">
                <el-radio-button label="direct">직접 연결</el-radio-button>
                <el-radio-button label="gateway">Gateway 사용</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col v-if="form.connectionMode === 'gateway'" :span="12">
            <el-form-item label="접속 Gateway" required>
              <el-select v-model="form.gatewayId" filterable placeholder="Gateway를 선택하십시오" style="width: 100%">
                <el-option v-for="item in gatewayOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="Monitoring Datasource">
          <div class="datasource-binding-control">
            <el-select v-model="form.monitorDatasourceId" clearable filterable placeholder="Prometheus / VictoriaMetrics 선택" style="flex: 1">
              <el-option v-for="item in datasourceOptions" :key="item.id" :label="`${item.name} · ${item.type}`" :value="item.id" />
            </el-select>
            <el-button plain @click="openDatasourceCreate">새 사용자 정의 Datasource</el-button>
          </div>
          <div class="form-hint">Pod CPU, Memory 모니터링은 이 Cluster에 바인딩된 Datasource만 사용합니다. 바인딩하지 않으면 전역 기본 Datasource를 조회하지 않습니다.</div>
        </el-form-item>
        <el-form-item :label="t('k8sKubeConfig')" required>
          <el-input
            v-model="form.kubeConfig"
            type="textarea"
            :rows="14"
            :placeholder="t('k8sKubeConfigPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">{{ t('save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="datasourceDialogVisible" title="새 사용자 정의 Monitoring Datasource" width="620px" append-to-body>
      <el-form label-width="110px">
        <el-form-item label="이름" required><el-input v-model="datasourceForm.name" placeholder="예: 테스트 Cluster Prometheus" /></el-form-item>
        <el-form-item label="Type"><el-radio-group v-model="datasourceForm.type"><el-radio-button label="prometheus">Prometheus</el-radio-button><el-radio-button label="victoriametrics">VictoriaMetrics</el-radio-button></el-radio-group></el-form-item>
        <el-form-item label="주소" required><el-input v-model="datasourceForm.url" placeholder="http://prometheus:9090" /></el-form-item>
        <el-form-item label="인증 방식"><el-select v-model="datasourceForm.authType" style="width: 100%"><el-option label="인증 없음" value="none" /><el-option label="Basic Auth" value="basic" /><el-option label="Bearer Token" value="bearer" /></el-select></el-form-item>
        <el-form-item v-if="datasourceForm.authType === 'basic'" label="사용자 이름"><el-input v-model="datasourceForm.username" /></el-form-item>
        <el-form-item v-if="datasourceForm.authType === 'basic'" label="비밀번호"><el-input v-model="datasourceForm.password" type="password" show-password /></el-form-item>
        <el-form-item v-if="datasourceForm.authType === 'bearer'" label="Token"><el-input v-model="datasourceForm.token" type="password" show-password /></el-form-item>
      </el-form>
      <template #footer><el-button @click="datasourceDialogVisible = false">취소</el-button><el-button type="primary" :loading="datasourceSaving" @click="saveDatasource">생성 후 바인딩</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.cluster-manage-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.datasource-binding-control { display: flex; width: 100%; gap: 10px; }
.form-hint { margin-top: 6px; color: #7d8ca3; font-size: 12px; line-height: 1.5; }

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

.row-actions {
  display: flex;
  gap: 12px;
}

@media (max-width: 860px) {
  .page-header {
    flex-direction: column;
  }
}
</style>
