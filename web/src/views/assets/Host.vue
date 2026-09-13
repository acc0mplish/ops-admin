<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import { at } from '../../utils/asset-i18n'
import {
  deleteAssetHost,
  queryAssetCloudAccountOptions,
  queryAssetCredentialOptions,
  queryAssetGatewayOptions,
  queryAssetHostGroupList,
  queryAssetHostList,
  removeAssetHostsFromGroup,
  syncAssetHost
} from '../../api/asset'
// I5-J (i5-plan §3.8·CW-4) — 1,070행 분할 잔류본. 호스트 폼 다이얼로그는 host/
// HostFormDialog.vue, 가져오기·클라우드 동기화·일괄 자격증명 다이얼로그는
// HostImportDialogs.vue로 이동(원본 좌표는 커밋 메시지·자식 파일 헤더). 자식은
// :page 주입(§5 #11 승계)·open* 진입은 defineExpose 위임으로 보존한다.
import HostFormDialog from './host/HostFormDialog.vue'
import HostImportDialogs from './host/HostImportDialogs.vue'

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
const formDialogRef = ref(null)
const importDialogsRef = ref(null)

const isGroupView = computed(() => Number(query.groupId || 0) > 0)

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

// 원본 :187-192 openCreate·:213-234 openEdit·:236-257 openCopy는 자식
// (HostFormDialog)으로 이동 — 테이블·툴바 진입점은 위임 호출로 대체했다.
function openCreate() {
  formDialogRef.value?.openCreate()
}

function openEdit(row) {
  formDialogRef.value?.openEdit(row)
}

function openCopy(row) {
  formDialogRef.value?.openCopy(row)
}

// 원본 :194-197 openImportDialog·:199-202 openCloudSyncDialog·:204-211
// openBatchCredentialDialog는 자식(HostImportDialogs)으로 이동 — 위임 호출.
function openImportDialog() {
  importDialogsRef.value?.openImportDialog()
}

function openCloudSyncDialog() {
  importDialogsRef.value?.openCloudSyncDialog()
}

function openBatchCredentialDialog() {
  importDialogsRef.value?.openBatchCredentialDialog()
}

async function handleSync(row) {
  syncingId.value = row.id
  try {
    await syncAssetHost(row.id)
    ElMessage.success(at('synced'))
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
  await ElMessageBox.confirm(at('deleteHostConfirm', { name: row.hostName }), at('notice'), { type: 'warning' })
  await deleteAssetHost(row.id)
  ElMessage.success(at('rowDeleted'))
  await loadData()
}

async function handleRemoveFromGroup(row) {
  await ElMessageBox.confirm(at('removeFromGroupConfirm', { name: row.hostName }), at('notice'), { type: 'warning' })
  await removeAssetHostsFromGroup({ groupId: query.groupId, hostId: row.id })
  ElMessage.success(at('removedFromGroup'))
  await loadData()
}

function handleSelectionChange(rows) {
  selectedRows.value = rows
}

// 원본 :506-510 handleMoreCommand(+그 대상 :315-317 selectedIds·:319-334
// handleBatchSync·:336-355 handleBatchDelete)는 자식(HostImportDialogs)으로
// 이동 — More 드롭다운 3개 명령 전부가 자식 소속이므로 위임 호출로 대체.
function handleMoreCommand(command) {
  importDialogsRef.value?.handleMoreCommand(command)
}

function groupName(row) {
  const groups = row.hostGroups || []
  if (groups.length) {
    return groups.map((item) => item.name).join(' / ')
  }
  return row.group?.name || '-'
}

function statusText(value, onlineText, offlineText, unknownText = at('notChecked')) {
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
  return parts.length ? parts.join(' / ') : at('syncPending')
}

// 원본 :488-490 — 폼·클라우드 동기화 자식 2종이 공유(goCredential)
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

// I5-J 자식 주입 번들 — reactive 래핑으로 ref/reactive가 언랩되어
// 자식 템플릿의 page.x / v-model="page.x" 재배선이 동작한다(§5 #11).
const page = reactive({
  dialogVisible,
  isEdit,
  isCopy,
  form,
  query,
  isGroupView,
  batchSyncSubmitting,
  importDialogVisible,
  importForm,
  importSubmitting,
  cloudSyncDialogVisible,
  cloudSyncForm,
  cloudSyncSubmitting,
  batchCredentialDialogVisible,
  batchCredentialForm,
  batchCredentialSubmitting,
  selectedRows,
  groupOptions,
  credentialOptions,
  gatewayOptions,
  cloudAccountOptions,
  environmentOptions,
  environmentLoading,
  goCredential,
  loadData
})

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
        <h2>{{ at('hostManageTitle') }}</h2>
        <p>{{ at('hostManageDesc') }}</p>
      </div>
      <div class="host-page-summary">
        <span>Resource Workbench</span>
        <small>{{ at('batchSyncSummary') }}</small>
      </div>
    </header>
    <section class="query-panel">
      <el-form inline>
        <el-form-item :label="at('hostNameLabel')">
          <el-input v-model="query.keyword" clearable :placeholder="at('enterHostName')" style="width: 160px" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item :label="at('ipLabel')">
          <el-input v-model="query.ipKeyword" clearable :placeholder="at('enterIp')" style="width: 160px" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item :label="at('hostStatusLabel')">
          <el-select v-model="query.status" clearable :placeholder="at('selectStatus')" style="width: 140px">
            <el-option :label="at('online')" value="1" />
            <el-option :label="at('offline')" value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="Host Group">
          <el-select v-model="query.groupId" clearable filterable :placeholder="at('selectGroup')" style="width: 180px">
            <el-option v-for="item in groupOptions" :key="item.id" :value="item.id" :label="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Environment">
          <el-select v-model="query.environment" clearable :placeholder="at('allEnvironments')" style="width: 150px">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="query-actions">
        <el-button type="primary" @click="loadData">{{ at('search') }}</el-button>
        <el-button color="#f0a43a" @click="resetQuery">{{ at('reset') }}</el-button>
        <el-dropdown split-button type="success" @click="openCreate" @command="handleCreateCommand">
          {{ at('addAction') }}
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="create">Host Import</el-dropdown-item>
              <el-dropdown-item command="excel">Excel Import</el-dropdown-item>
              <el-dropdown-item command="cloud">{{ at('syncCloudHosts') }}</el-dropdown-item>
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
            {{ at('more') }}
            <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
			  <el-dropdown-item command="batch-sync" :disabled="batchSyncSubmitting">
				<el-icon v-if="batchSyncSubmitting" class="is-loading"><Loading /></el-icon>
				{{ batchSyncSubmitting ? at('syncing') : at('batchSync') }}
			  </el-dropdown-item>
              <el-dropdown-item command="batch-delete">{{ isGroupView ? at('batchRemoveFromGroup') : at('batchDelete') }}</el-dropdown-item>
              <el-dropdown-item command="batch-credential">{{ at('batchReplaceCredential') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </section>

    <el-table v-loading="loading" :data="tableData" class="host-table" @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" />
      <el-table-column :label="at('hostNameLabel')" min-width="180">
        <template #default="{ row }">
          <div class="host-name">
            <span class="linux-icon">L</span>
            <el-button link type="primary" @click="goDetail(row)">{{ row.hostName }}</el-button>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="at('ipLabel')" min-width="170">
        <template #default="{ row }">
          <div class="ip-list">
            <span v-if="row.publicIp" class="ip public">{{ at('publicIpLabel', { ip: row.publicIp }) }}</span>
            <span v-if="row.privateIp || row.sshIp" class="ip private">{{ at('privateIpLabel', { ip: row.privateIp || row.sshIp }) }}</span>
            <span v-if="!row.publicIp && !row.privateIp && !row.sshIp">-</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="at('cpuUsageLabel')" width="110">
        <template #default="{ row }"><span :class="{ 'metric-unavailable': !row.cpuUsage }" :title="row.metricsStatus === 'not_configured' ? at('metricsNotConfigured') : at('localMetricsBasis')">{{ row.cpuUsage || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="at('memoryUsageLabel')" width="120">
        <template #default="{ row }"><span :class="{ 'metric-unavailable': !row.memoryUsage }" :title="row.metricsStatus === 'not_configured' ? at('metricsNotConfigured') : at('localMetricsBasis')">{{ row.memoryUsage || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="at('diskUsageLabel')" width="120">
        <template #default="{ row }"><span :class="{ 'metric-unavailable': !row.diskUsage }" :title="row.metricsStatus === 'not_configured' ? at('metricsNotConfigured') : at('localMetricsBasis')">{{ row.diskUsage || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="at('configInfoLabel')" min-width="170">
        <template #default="{ row }">
          <div class="config-info">
            <span>{{ configText(row) }}</span>
            <small v-if="row.os">{{ row.os }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="at('onlineStatusLabel')" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.aliveStatus)" effect="light">
            {{ statusText(row.aliveStatus, at('online'), at('offline')) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="at('authStatusLabel')" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.authStatus)" effect="light">
            {{ statusText(row.authStatus, at('authSuccess'), at('authFailed')) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="at('accessModeLabel')" min-width="140">
        <template #default="{ row }">
          <span v-if="row.connectionMode === 'gateway'">Gateway: {{ row.gateway?.name || '-' }}</span>
          <span v-else>{{ at('directConnection') }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="at('hostTypeLabel')" width="100">
        <template #default="{ row }">{{ row.provider || at('selfHosted') }}</template>
      </el-table-column>
      <el-table-column label="Environment" width="110">
        <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.environment) }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="at('groupColumnLabel')" min-width="120">
        <template #default="{ row }">{{ groupName(row) }}</template>
      </el-table-column>
      <el-table-column :label="at('actions')" width="230" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ at('edit') }}</el-button>
          <el-button link type="primary" @click="openCopy(row)">{{ at('clone') }}</el-button>
          <el-button link type="success" :loading="syncingId === row.id" @click="handleSync(row)">{{ at('sync') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ isGroupView ? at('removeFromGroup') : at('delete') }}</el-button>
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

    <HostFormDialog ref="formDialogRef" :page="page" />

    <HostImportDialogs ref="importDialogsRef" :page="page" />
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
