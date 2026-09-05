<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import {
  addAssetHost,
  assetHostInfo,
  batchDeleteAssetHosts,
  batchReplaceAssetHostCredential,
  batchSyncAssetHosts,
  deleteAssetHost,
  downloadAssetHostTemplate,
  importAssetHosts,
  queryAssetCloudAccountOptions,
  queryAssetCredentialOptions,
  queryAssetGatewayOptions,
  queryAssetHostGroupList,
  queryAssetHostList,
  removeAssetHostsFromGroup,
  syncAssetHost,
  syncAssetHostsFromCloud,
  updateAssetHost
} from '../../api/asset'

const route = useRoute()
const router = useRouter()
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()
const loading = ref(false)
const syncingId = ref()
const dialogVisible = ref(false)
const importDialogVisible = ref(false)
const cloudSyncDialogVisible = ref(false)
const batchCredentialDialogVisible = ref(false)
const importSubmitting = ref(false)
const cloudSyncSubmitting = ref(false)
const batchCredentialSubmitting = ref(false)
const batchSyncSubmitting = ref(false)
const isEdit = ref(false)
const isCopy = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const groupOptions = ref([])
const credentialOptions = ref([])
const gatewayOptions = ref([])
const cloudAccountOptions = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', ipKeyword: '', status: '', groupId: undefined, environment: '' })
const form = reactive({
  id: undefined,
  hostName: '',
  groupId: undefined,
  groupIds: [],
  sshUser: '',
  sshIp: '',
  sshPort: 22,
  credentialId: undefined,
  connectionMode: 'direct',
  gatewayId: undefined,
  environment: '',
  status: 1,
  description: ''
})
const importForm = reactive({
  groupId: undefined,
  file: null
})
const cloudSyncForm = reactive({
  groupId: undefined,
  provider: 'tencent',
  useExistingAccount: true,
  cloudAccountId: undefined,
	credentialId: undefined,
	connectionMode: 'direct',
	gatewayId: undefined,
	environment: '',
  accessKey: '',
  secretKey: '',
  saveAccount: false,
  accountName: ''
})
const batchCredentialForm = reactive({
  credentialId: undefined
})

const filteredCloudAccounts = computed(() =>
  cloudAccountOptions.value.filter((item) => (item.provider || '').toLowerCase() === cloudSyncForm.provider)
)
const isGroupView = computed(() => Number(query.groupId || 0) > 0)

function resetForm() {
  Object.assign(form, {
    id: undefined,
    hostName: '',
    groupId: undefined,
    groupIds: [],
    sshUser: '',
    sshIp: '',
    sshPort: 22,
    credentialId: undefined,
    connectionMode: 'direct',
    gatewayId: undefined,
    environment: '',
    status: 1,
    description: ''
  })
}

function resetImportForm() {
  Object.assign(importForm, {
    groupId: undefined,
    file: null
  })
}

function resetCloudSyncForm() {
  Object.assign(cloudSyncForm, {
    groupId: undefined,
    provider: 'tencent',
    useExistingAccount: true,
    cloudAccountId: undefined,
	credentialId: undefined,
	connectionMode: 'direct',
	gatewayId: undefined,
	environment: '',
    accessKey: '',
    secretKey: '',
    saveAccount: false,
    accountName: ''
  })
}

function resetBatchCredentialForm() {
  batchCredentialForm.credentialId = undefined
}

async function loadOptions() {
  const [groups, credentials, cloudAccounts, gateways] = await Promise.all([
    queryAssetHostGroupList(),
    queryAssetCredentialOptions(),
    queryAssetCloudAccountOptions(),
    queryAssetGatewayOptions()
  ])
  groupOptions.value = groups.list || []
  credentialOptions.value = credentials || []
  cloudAccountOptions.value = cloudAccounts || []
  gatewayOptions.value = gateways || []
}

async function loadData() {
  loading.value = true
  try {
    const keyword = [query.keyword, query.ipKeyword].filter(Boolean).join(' ')
    const data = await queryAssetHostList({
      pageNum: query.pageNum,
      pageSize: query.pageSize,
      keyword,
      status: query.status,
      groupId: query.groupId,
      environment: query.environment
    })
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.keyword = ''
  query.ipKeyword = ''
  query.status = ''
  query.groupId = undefined
  query.environment = ''
  query.pageNum = 1
  loadData()
}

function applyRouteGroupFilter() {
  const groupId = Number(route.query.groupId || 0)
  query.groupId = groupId > 0 ? groupId : undefined
  query.pageNum = 1
}

function openCreate() {
  isEdit.value = false
  isCopy.value = false
  resetForm()
  dialogVisible.value = true
}

function openImportDialog() {
  resetImportForm()
  importDialogVisible.value = true
}

function openCloudSyncDialog() {
  resetCloudSyncForm()
  cloudSyncDialogVisible.value = true
}

function openBatchCredentialDialog() {
  if (!selectedRows.value.length) {
    ElMessage.warning('먼저 Host를 선택하십시오.')
    return
  }
  resetBatchCredentialForm()
  batchCredentialDialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  isCopy.value = false
  const data = await assetHostInfo(row.id)
  resetForm()
  Object.assign(form, {
    id: data.id,
    hostName: data.hostName,
    groupId: data.groupId,
    groupIds: (data.hostGroups || []).map((item) => item.id).length ? (data.hostGroups || []).map((item) => item.id) : (data.groupId ? [data.groupId] : []),
    sshUser: data.sshUser,
    sshIp: data.sshIp,
    sshPort: data.sshPort || 22,
    credentialId: data.credentialId,
    connectionMode: data.connectionMode || 'direct',
    gatewayId: data.gatewayId || undefined,
    environment: data.environment || '',
    status: data.status || 1,
    description: data.description
  })
  dialogVisible.value = true
}

async function openCopy(row) {
  isEdit.value = false
  isCopy.value = true
  const data = await assetHostInfo(row.id)
  resetForm()
  Object.assign(form, {
    id: undefined,
    hostName: `${data.hostName || row.hostName || ''}-사본`,
    groupId: data.groupId,
    groupIds: (data.hostGroups || []).map((item) => item.id).length ? (data.hostGroups || []).map((item) => item.id) : (data.groupId ? [data.groupId] : []),
    sshUser: data.sshUser,
    sshIp: data.sshIp,
    sshPort: data.sshPort || 22,
    credentialId: data.credentialId,
    connectionMode: data.connectionMode || 'direct',
    gatewayId: data.gatewayId || undefined,
    environment: data.environment || '',
    status: data.status || 1,
    description: data.description
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.hostName || !form.groupIds.length || !form.environment || !form.sshUser || !form.sshIp || !form.credentialId) {
    ElMessage.warning('Host 이름, Host Group, SSH 연결과 Credential을 입력하십시오.')
    return
  }
  if (form.connectionMode === 'gateway' && !form.gatewayId) {
    ElMessage.warning('접속 Gateway를 선택하십시오.')
    return
  }
  form.groupId = form.groupIds[0]

  if (isEdit.value) {
    await updateAssetHost(form)
    ElMessage.success('Host를 수정했습니다.')
  } else {
    await addAssetHost(form)
    ElMessage.success(isCopy.value ? 'Host를 복제했습니다. Public 주소와 Config 정보는 필요에 따라 동기화로 수집하십시오.' : 'Host를 생성했습니다. 동기화를 클릭해 Public 주소와 Config 정보를 수집할 수 있습니다.')
  }
  isCopy.value = false
  dialogVisible.value = false
  await loadData()
}

async function handleSync(row) {
  syncingId.value = row.id
  try {
    await syncAssetHost(row.id)
    ElMessage.success('동기화했습니다.')
    await loadData()
  } finally {
    syncingId.value = undefined
  }
}

async function handleDelete(row) {
  if (isGroupView.value) {
    await handleRemoveFromGroup(row)
    return
  }
  await ElMessageBox.confirm(`Host ${row.hostName}을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteAssetHost(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

async function handleRemoveFromGroup(row) {
  await ElMessageBox.confirm(`Host ${row.hostName}을(를) 현재 Host Group에서 제외하시겠습니까? Host Asset은 삭제되지 않습니다.`, '알림', { type: 'warning' })
  await removeAssetHostsFromGroup({ groupId: query.groupId, hostId: row.id })
  ElMessage.success('현재 Host Group에서 제외했습니다.')
  await loadData()
}

function handleSelectionChange(rows) {
  selectedRows.value = rows
}

function selectedIds() {
  return selectedRows.value.map((item) => item.id)
}

async function handleBatchSync() {
	if (batchSyncSubmitting.value) return
  const ids = selectedIds()
  if (!ids.length) {
    ElMessage.warning('먼저 Host를 선택하십시오.')
    return
  }
	batchSyncSubmitting.value = true
	try {
		const data = await batchSyncAssetHosts(ids)
		ElMessage.success(`일괄 동기화 완료: 성공 ${data.success}대, 실패 ${data.fail}대`)
		await loadData()
	} finally {
		batchSyncSubmitting.value = false
	}
}

async function handleBatchDelete() {
  const ids = selectedIds()
  if (!ids.length) {
    ElMessage.warning('먼저 Host를 선택하십시오.')
    return
  }
  if (isGroupView.value) {
    await ElMessageBox.confirm(`선택한 ${ids.length}대 Host를 현재 Host Group에서 제외하시겠습니까? Host Asset은 삭제되지 않습니다.`, '알림', { type: 'warning' })
    await removeAssetHostsFromGroup({ groupId: query.groupId, hostIds: ids })
    ElMessage.success('현재 Host Group에서 일괄 제외했습니다.')
    selectedRows.value = []
    await loadData()
    return
  }
  await ElMessageBox.confirm(`선택한 ${ids.length}대 Host를 일괄 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await batchDeleteAssetHosts(ids)
  ElMessage.success('일괄 삭제했습니다.')
  selectedRows.value = []
  await loadData()
}

async function submitBatchCredential() {
  const ids = selectedIds()
  if (!ids.length) {
    ElMessage.warning('먼저 Host를 선택하십시오.')
    return
  }
  if (!batchCredentialForm.credentialId) {
    ElMessage.warning('Credential을 선택하십시오.')
    return
  }
  batchCredentialSubmitting.value = true
  try {
    await batchReplaceAssetHostCredential({
      ids,
      credentialId: batchCredentialForm.credentialId
    })
    ElMessage.success('Credential을 일괄 교체했습니다.')
    batchCredentialDialogVisible.value = false
    await loadData()
  } finally {
    batchCredentialSubmitting.value = false
  }
}

async function handleTemplateDownload() {
  const response = await downloadAssetHostTemplate()
  const blob = new Blob([response.data], { type: response.headers['content-type'] })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'asset-host-template.xlsx'
  link.click()
  window.URL.revokeObjectURL(url)
}

function handleFileChange(uploadFile) {
  importForm.file = uploadFile.raw || null
}

function clearImportFile() {
  importForm.file = null
}

async function submitImport() {
  if (!importForm.groupId || !importForm.file) {
    ElMessage.warning('Group을 선택하고 Excel 파일을 업로드하십시오.')
    return
  }
  importSubmitting.value = true
  try {
    const formData = new FormData()
    formData.append('groupId', importForm.groupId)
    formData.append('file', importForm.file)
    const data = await importAssetHosts(formData)
    const failedPreview = (data.failedHosts || []).slice(0, 3).join('; ')
    ElMessage.success(`Import 완료: 성공 ${data.success}대, 실패 ${data.fail}대${failedPreview ? `(${failedPreview})` : ''}`)
    importDialogVisible.value = false
    await loadData()
  } finally {
    importSubmitting.value = false
  }
}

async function submitCloudSync() {
  if (!cloudSyncForm.groupId || !cloudSyncForm.provider) {
    ElMessage.warning('Group과 Cloud Provider를 선택하십시오.')
    return
  }
  if (cloudSyncForm.useExistingAccount && !cloudSyncForm.cloudAccountId) {
    ElMessage.warning('기존 Cloud Account를 선택하십시오.')
    return
  }
	if (!cloudSyncForm.credentialId) {
		ElMessage.warning('Credential을 선택하십시오.')
		return
	}
  if (!cloudSyncForm.environment) {
		ElMessage.warning('소속 Environment를 선택하십시오.')
		return
	}
	if (cloudSyncForm.connectionMode === 'gateway' && !cloudSyncForm.gatewayId) {
		ElMessage.warning('접속 Gateway를 선택하십시오.')
		return
	}
  if (!cloudSyncForm.useExistingAccount && (!cloudSyncForm.accessKey || !cloudSyncForm.secretKey)) {
    ElMessage.warning('AccessKey와 SecretKey를 입력하십시오.')
    return
  }
  cloudSyncSubmitting.value = true
  try {
    const data = await syncAssetHostsFromCloud(cloudSyncForm)
    const addedNames = (data.addedHosts || []).slice(0, 3).join(', ')
    const updatedNames = (data.updatedHosts || []).slice(0, 3).join(', ')
    const details = [
      addedNames ? `추가: ${addedNames}` : '',
      updatedNames ? `업데이트: ${updatedNames}` : '',
      Object.keys(data.regionCounts || {}).length ? `Region: ${Object.entries(data.regionCounts).map(([region, count]) => `${region} ${count}대`).join(', ')}` : ''
    ].filter(Boolean).join('; ')
    ElMessage.success(`Cloud에서 ${data.total || 0}대 발견: 추가 ${data.added}대, 업데이트 ${data.updated}대, 건너뜀 ${data.skipped}대${details ? ` (${details})` : ''}`)
    cloudSyncDialogVisible.value = false
    await loadData()
  } finally {
    cloudSyncSubmitting.value = false
  }
}

function groupName(row) {
  const groups = row.hostGroups || []
  if (groups.length) {
    return groups.map((item) => item.name).join(' / ')
  }
  return row.group?.name || '-'
}

function statusText(value, onlineText, offlineText, unknownText = '미점검') {
  if (value === 1) return onlineText
  if (value === 2) return offlineText
  return unknownText
}

function statusType(value) {
  if (value === 1) return 'success'
  if (value === 2) return 'danger'
  return 'info'
}

function configText(row) {
  const parts = [row.cpu, row.memory, row.disk].filter(Boolean)
  return parts.length ? parts.join(' / ') : '동기화 대기'
}

function goCredential() {
  router.push('/assets/server/credentials')
}

function goTerminal() {
  router.push('/assets/terminal')
}

function goDetail(row) {
  router.push(`/assets/server/hosts/${row.id}/detail`)
}

function handleCreateCommand(command) {
  if (command === 'create') openCreate()
  if (command === 'excel') openImportDialog()
  if (command === 'cloud') openCloudSyncDialog()
}

function handleMoreCommand(command) {
  if (command === 'batch-sync') handleBatchSync()
  if (command === 'batch-delete') handleBatchDelete()
  if (command === 'batch-credential') openBatchCredentialDialog()
}

onMounted(async () => {
  applyRouteGroupFilter()
  await loadOptions()
  await loadData()
})

watch(
  () => route.query.groupId,
  async () => {
    applyRouteGroupFilter()
    await loadData()
  }
)
</script>

<template>
  <div class="asset-host-page">
    <header class="host-page-header">
      <div>
        <p class="host-page-kicker">ASSET INVENTORY</p>
        <h2>Host 관리</h2>
        <p>Host 연결 상태, 인증 상태와 용량 Metric을 통합 조회하고 Offline 및 인증 이상 Resource를 우선 처리합니다.</p>
      </div>
      <div class="host-page-summary">
        <span>Resource Workbench</span>
        <small>일괄 동기화와 Credential 교체 지원</small>
      </div>
    </header>
    <section class="query-panel">
      <el-form inline>
        <el-form-item label="Host 이름">
          <el-input v-model="query.keyword" clearable placeholder="Host 이름을 입력하십시오." style="width: 160px" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="IP 주소">
          <el-input v-model="query.ipKeyword" clearable placeholder="IP 주소를 입력하십시오." style="width: 160px" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="Host 상태">
          <el-select v-model="query.status" clearable placeholder="상태 선택" style="width: 140px">
            <el-option label="온라인" value="1" />
            <el-option label="오프라인" value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="Host Group">
          <el-select v-model="query.groupId" clearable filterable placeholder="Host Group 선택" style="width: 180px">
            <el-option v-for="item in groupOptions" :key="item.id" :value="item.id" :label="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Environment">
          <el-select v-model="query.environment" clearable placeholder="전체 Environment" style="width: 150px">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="query-actions">
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button color="#f0a43a" @click="resetQuery">초기화</el-button>
        <el-dropdown split-button type="success" @click="openCreate" @command="handleCreateCommand">
          추가
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="create">Host Import</el-dropdown-item>
              <el-dropdown-item command="excel">Excel Import</el-dropdown-item>
              <el-dropdown-item command="cloud">Cloud Host 동기화</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button color="#6f58c9" @click="goTerminal">Terminal</el-button>
        <el-dropdown
          placement="bottom-end"
          popper-class="host-more-dropdown"
          :disabled="!selectedRows.length"
          @command="handleMoreCommand"
        >
          <el-button class="more-action-trigger" :disabled="!selectedRows.length">
            더 보기
            <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
			  <el-dropdown-item command="batch-sync" :disabled="batchSyncSubmitting">
				<el-icon v-if="batchSyncSubmitting" class="is-loading"><Loading /></el-icon>
				{{ batchSyncSubmitting ? '동기화 중' : '일괄 동기화' }}
			  </el-dropdown-item>
              <el-dropdown-item command="batch-delete">{{ isGroupView ? '현재 Group에서 일괄 제외' : '일괄 삭제' }}</el-dropdown-item>
              <el-dropdown-item command="batch-credential">Credential 일괄 교체</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </section>

    <el-table v-loading="loading" :data="tableData" class="host-table" @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" />
      <el-table-column label="Host 이름" min-width="180">
        <template #default="{ row }">
          <div class="host-name">
            <span class="linux-icon">L</span>
            <el-button link type="primary" @click="goDetail(row)">{{ row.hostName }}</el-button>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="IP 주소" min-width="170">
        <template #default="{ row }">
          <div class="ip-list">
            <span v-if="row.publicIp" class="ip public">공인 {{ row.publicIp }}</span>
            <span v-if="row.privateIp || row.sshIp" class="ip private">내부 {{ row.privateIp || row.sshIp }}</span>
            <span v-if="!row.publicIp && !row.privateIp && !row.sshIp">-</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="CPU 사용" width="110">
        <template #default="{ row }"><span :class="{ 'metric-unavailable': !row.cpuUsage }" :title="row.metricsStatus === 'not_configured' ? 'Prometheus/VictoriaMetrics Datasource 미구성' : '로컬 Monitoring Datasource 기준'">{{ row.cpuUsage || '-' }}</span></template>
      </el-table-column>
      <el-table-column label="메모리 사용" width="120">
        <template #default="{ row }"><span :class="{ 'metric-unavailable': !row.memoryUsage }" :title="row.metricsStatus === 'not_configured' ? 'Prometheus/VictoriaMetrics Datasource 미구성' : '로컬 Monitoring Datasource 기준'">{{ row.memoryUsage || '-' }}</span></template>
      </el-table-column>
      <el-table-column label="디스크 사용" width="120">
        <template #default="{ row }"><span :class="{ 'metric-unavailable': !row.diskUsage }" :title="row.metricsStatus === 'not_configured' ? 'Prometheus/VictoriaMetrics Datasource 미구성' : '로컬 Monitoring Datasource 기준'">{{ row.diskUsage || '-' }}</span></template>
      </el-table-column>
      <el-table-column label="Config 정보" min-width="170">
        <template #default="{ row }">
          <div class="config-info">
            <span>{{ configText(row) }}</span>
            <small v-if="row.os">{{ row.os }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="온라인 상태" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.aliveStatus)" effect="light">
            {{ statusText(row.aliveStatus, '온라인', '오프라인') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="인증 상태" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.authStatus)" effect="light">
            {{ statusText(row.authStatus, '인증 성공', '인증 실패') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="접속 방식" min-width="140">
        <template #default="{ row }">
          <span v-if="row.connectionMode === 'gateway'">Gateway: {{ row.gateway?.name || '-' }}</span>
          <span v-else>직접 연결</span>
        </template>
      </el-table-column>
      <el-table-column label="Host 유형" width="100">
        <template #default="{ row }">{{ row.provider || '자체 구축' }}</template>
      </el-table-column>
      <el-table-column label="Environment" width="110">
        <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.environment) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="소속 Group" min-width="120">
        <template #default="{ row }">{{ groupName(row) }}</template>
      </el-table-column>
      <el-table-column label="작업" width="230" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="primary" @click="openCopy(row)">복제</el-button>
          <el-button link type="success" :loading="syncingId === row.id" @click="handleSync(row)">동기화</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ isGroupView ? 'Group에서 제외' : '삭제' }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        layout="total, sizes, prev, pager, next, jumper"
        :total="total"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Host 수정' : (isCopy ? 'Host 복제' : 'Host 추가')" width="640px">
      <el-form label-width="96px">
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item label="Host 이름" required>
              <el-input v-model="form.hostName" placeholder="Host 이름을 입력하십시오." />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="소속 Host Group" required>
              <el-select v-model="form.groupIds" multiple collapse-tags collapse-tags-tooltip filterable placeholder="Host Group 선택" style="width: 100%">
                <el-option v-for="item in groupOptions" :key="item.id" :value="item.id" :label="item.name" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="SSH 연결" required>
              <div class="ssh-line">
                <el-input v-model="form.sshUser" placeholder="사용자명" />
                <span>@</span>
                <el-input v-model="form.sshIp" placeholder="Host 주소" />
                <span>-p</span>
                <el-input-number v-model="form.sshPort" :min="1" :max="65535" controls-position="right" />
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="인증 Credential" required>
              <div class="credential-line">
                <el-select v-model="form.credentialId" clearable filterable placeholder="Credential 선택">
                  <el-option v-for="item in credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
                </el-select>
                <el-button color="#f59e0b" @click="goCredential">+ Credential 생성</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="소속 Environment" required>
              <el-select v-model="form.environment" :loading="environmentLoading" placeholder="Environment 선택" style="width: 100%">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="연결 방식">
              <el-radio-group v-model="form.connectionMode">
                <el-radio-button label="direct">직접 연결</el-radio-button>
                <el-radio-button label="gateway">Gateway 경유</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col v-if="form.connectionMode === 'gateway'" :span="12">
            <el-form-item label="접속 Gateway" required>
              <el-select v-model="form.gatewayId" filterable placeholder="Gateway 선택" style="width: 100%">
                <el-option v-for="item in gatewayOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="비고">
              <el-input v-model="form.description" type="textarea" :rows="3" placeholder="비고를 입력하십시오." />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submit">확인</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="Excel Import" width="520px">
      <el-form label-width="92px">
        <el-form-item label="Template Download">
          <el-button type="primary" @click="handleTemplateDownload">Template 다운로드</el-button>
        </el-form-item>
        <el-form-item label="Group 선택">
          <el-select v-model="importForm.groupId" filterable placeholder="Group 선택" style="width: 100%">
            <el-option v-for="item in groupOptions" :key="item.id" :value="item.id" :label="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Excel Upload">
          <el-upload
            :auto-upload="false"
            :show-file-list="true"
            :limit="1"
            accept=".xlsx,.xls"
            :on-change="handleFileChange"
            :on-remove="clearImportFile"
          >
            <el-button type="primary">파일 선택</el-button>
          </el-upload>
        </el-form-item>
        <div class="dialog-tip">Target Group은 이 대화상자에서 일괄 지정합니다. Template 필수 열: Host 이름, SSH 주소, SSH 사용자, 인증 Credential, 연결 방식, 소속 Environment. Gateway 모드는 접속 Gateway 이름도 입력해야 합니다. SSH 주소는 내부 IP 입력을 권장합니다.</div>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="importSubmitting" @click="submitImport">Host Import</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="cloudSyncDialogVisible" title="Cloud Host 동기화" width="620px">
      <el-form label-width="108px">
        <el-form-item label="Target Group">
          <el-select v-model="cloudSyncForm.groupId" filterable placeholder="동기화 Group 선택" style="width: 100%">
            <el-option v-for="item in groupOptions" :key="item.id" :value="item.id" :label="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Cloud Provider">
          <el-select v-model="cloudSyncForm.provider" placeholder="Cloud Provider 선택" style="width: 100%">
            <el-option label="Tencent Cloud" value="tencent" />
            <el-option label="Alibaba Cloud" value="aliyun" />
          </el-select>
        </el-form-item>
        <el-form-item label="Cloud Account">
          <el-select v-model="cloudSyncForm.cloudAccountId" filterable clearable placeholder="Cloud Account 선택" style="width: 100%">
            <el-option
              v-for="item in filteredCloudAccounts"
              :key="item.id"
              :value="item.id"
              :label="`${item.name} (${(item.regions?.length ? item.regions : (item.region ? item.region.split(/[,，;；\s]+/).filter(Boolean) : [])).join(', ')})`"
            />
          </el-select>
        </el-form-item>
		<el-form-item label="인증 Credential" required>
		  <div class="credential-line">
			<el-select v-model="cloudSyncForm.credentialId" clearable filterable placeholder="Credential 선택">
			  <el-option v-for="item in credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
			</el-select>
			<el-button color="#f59e0b" @click="goCredential">+ Credential 생성</el-button>
		  </div>
		</el-form-item>
        <el-form-item label="연결 방식">
		  <el-radio-group v-model="cloudSyncForm.connectionMode">
			<el-radio value="direct">직접 연결</el-radio>
			<el-radio value="gateway">Gateway 경유</el-radio>
		  </el-radio-group>
		</el-form-item>
		<el-form-item v-if="cloudSyncForm.connectionMode === 'gateway'" label="접속 Gateway">
		  <el-select v-model="cloudSyncForm.gatewayId" filterable placeholder="Gateway 선택" style="width: 100%">
			<el-option v-for="item in gatewayOptions" :key="item.id" :value="item.id" :label="item.name" />
		  </el-select>
		</el-form-item>
		<el-form-item label="소속 Environment">
		  <el-select v-model="cloudSyncForm.environment" filterable placeholder="소속 Environment 선택" :loading="environmentLoading" style="width: 100%">
			<el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
		  </el-select>
		</el-form-item>
        <div class="dialog-tip">동기화는 Cloud Account 관리에서 구성한 Region을 엄격히 사용합니다. 먼저 Cloud Account 관리에서 Region을 하나 이상 등록하십시오. Alibaba Cloud와 Tencent Cloud 모두 다중 Region Cloud Host 동기화를 지원합니다.</div>
      </el-form>
      <template #footer>
        <el-button @click="cloudSyncDialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="cloudSyncSubmitting" @click="submitCloudSync">동기화 시작</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="batchCredentialDialogVisible" title="인증 Credential 일괄 교체" width="480px">
      <el-form label-width="96px">
        <el-form-item label="선택된 Host">
          <span>{{ selectedRows.length }}대</span>
        </el-form-item>
        <el-form-item label="인증 Credential">
          <el-select v-model="batchCredentialForm.credentialId" filterable placeholder="Credential 선택" style="width: 100%">
            <el-option v-for="item in credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchCredentialDialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="batchCredentialSubmitting" @click="submitBatchCredential">교체 확인</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.asset-host-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.query-panel {
  padding: 16px;
  border: 1px solid #e3e8f0;
  border-radius: 10px;
  background: #f9fafc;
}

.query-actions {
  display: flex;
  gap: 10px;
  margin-top: 10px;
  align-items: center;
}

.host-table {
  overflow: hidden;
  border-radius: 10px;
  border: 1px solid #e3e8f0;
  box-shadow: 0 2px 5px rgba(20, 34, 58, 0.035);
}

.host-name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #2787ff;
  font-weight: 700;
}

.linux-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #111827;
  color: #facc15;
  font-size: 12px;
  font-weight: 800;
}

.ip-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: #2787ff;
  font-size: 13px;
}

.ip {
  line-height: 1.2;
}

.config-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.config-info small {
  color: #8190ad;
}

.host-page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 22px;
  border: 1px solid #dfe7f3;
  border-radius: 12px;
  background: linear-gradient(110deg, #ffffff 0%, #f3f7fd 100%);
}

.host-page-kicker {
  margin: 0 0 6px;
  color: #356ae6;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: .12em;
}

.host-page-header h2 {
  margin: 0;
  color: #18243a;
  font-size: 22px;
  font-weight: 650;
  letter-spacing: -.02em;
}

.host-page-header > div > p:last-child {
  margin: 7px 0 0;
  color: #66758d;
  font-size: 13px;
}

.host-page-summary {
  display: grid;
  gap: 4px;
  min-width: 180px;
  padding: 10px 12px;
  border-left: 3px solid #356ae6;
  color: #304b78;
  background: rgba(53, 106, 230, .06);
}

.host-page-summary span { font-size: 13px; font-weight: 650; }
.host-page-summary small { color: #71809a; font-size: 12px; }

.pager {
  display: flex;
  justify-content: flex-end;
}

.ssh-line,
.credential-line {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.ssh-line .el-input:first-child {
  width: 130px;
}

.ssh-line .el-input:nth-child(3) {
  flex: 1;
}

.ssh-line .el-input-number {
  width: 96px;
}

.credential-line .el-select {
  flex: 1;
}

.dialog-tip {
  color: #7c87a6;
  font-size: 13px;
  line-height: 1.7;
}

.more-action-trigger {
  min-width: 110px;
  height: 34px;
  border: 1px solid #dce3ed;
  border-radius: 8px;
  background: #fff;
  color: #52647e;
  box-shadow: none;
}

.more-action-trigger:hover,
.more-action-trigger:focus {
  color: #4e67a2;
  border-color: #9fbdf7;
  background: #edf4ff;
}

.more-action-trigger.is-disabled {
  color: #aeb8cf;
  border-color: #dce5f6;
  background: #f7f9fd;
}

:deep(.host-more-dropdown.el-popper) {
  border: 1px solid #dbe6ff;
  border-radius: 10px;
  box-shadow: 0 16px 32px rgba(42, 68, 132, 0.14);
  overflow: hidden;
}

:deep(.host-more-dropdown .el-dropdown-menu) {
  padding: 6px 0;
}

:deep(.host-more-dropdown .el-dropdown-menu__item) {
  min-width: 152px;
  padding: 10px 14px;
  color: #53688f;
  font-size: 13px;
}

:deep(.host-more-dropdown .el-dropdown-menu__item:hover) {
  background: #eef4ff;
  color: #4669c9;
}

@media (max-width: 900px) {
  .host-page-header { align-items: flex-start; flex-direction: column; }
  .host-page-summary { width: 100%; }
}

.metric-unavailable {
  color: #a6b1c2;
}
</style>
