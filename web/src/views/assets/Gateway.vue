<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { at } from '../../utils/asset-i18n'
import {
  addAssetGateway,
  assetGatewayInfo,
  deleteAssetGateway,
  queryAssetCredentialOptions,
  queryAssetGatewayList,
  testAssetGateway,
  updateAssetGateway,
  updateAssetGatewayStatus
} from '../../api/asset'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const list = ref([])
const total = ref(0)
const credentials = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  status: ''
})

const form = reactive({
  id: undefined,
  name: '',
  code: '',
  gatewayType: 'ssh',
  host: '',
  port: 22,
  credentialId: undefined,
  networkZone: '',
  status: 1,
  description: ''
})

function formatDateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false }).replaceAll('/', '-')
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    code: '',
    gatewayType: 'ssh',
    host: '',
    port: 22,
    credentialId: undefined,
    networkZone: '',
    status: 1,
    description: ''
  })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetGatewayList(query)
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadCredentials() {
  credentials.value = await queryAssetCredentialOptions()
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  resetForm()
  const data = await assetGatewayInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function saveGateway() {
  if (!form.name.trim() || !form.host.trim() || !form.credentialId) {
    ElMessage.warning(at('enterGatewayFields'))
    return
  }
  if (isEdit.value) {
    await updateAssetGateway(form)
    ElMessage.success(at('gatewayUpdated'))
  } else {
    await addAssetGateway(form)
    ElMessage.success(at('gatewayCreated'))
  }
  dialogVisible.value = false
  loadData()
}

async function handleTest(row) {
  await testAssetGateway(row.id)
  ElMessage.success(at('gatewayAlive'))
  loadData()
}

async function toggleStatus(row) {
  await updateAssetGatewayStatus({ id: row.id, status: row.status === 1 ? 2 : 1 })
  ElMessage.success(row.status === 1 ? at('gatewayDeactivated') : at('gatewayActivated'))
  loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(at('deleteGatewayConfirm', { name: row.name }), at('confirmDeleteTitle'), { type: 'warning' })
  await deleteAssetGateway(row.id)
  ElMessage.success(at('gatewayDeleted'))
  loadData()
}

onMounted(() => {
  loadCredentials()
  loadData()
})
</script>

<template>
  <div class="gateway-page">
    <div class="page-hero">
      <div>
        <h1>{{ at('gatewayManageTitle') }}</h1>
        <p>{{ at('gatewayManageDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ at('addGatewayButton') }}</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" :placeholder="at('gatewaySearchPlaceholder')" clearable style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" :placeholder="at('statusSelect')" clearable style="width: 140px">
        <el-option :label="at('enabled')" value="1" />
        <el-option :label="at('disabled')" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ at('search') }}</el-button>
      <el-button @click="Object.assign(query, { keyword: '', status: '', pageNum: 1 }); loadData()">{{ at('reset') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="list" class="gateway-table">
      <el-table-column prop="name" :label="at('gatewayNameColumn')" min-width="160" />
      <el-table-column prop="host" :label="at('gatewayAddrColumn')" min-width="170">
        <template #default="{ row }">{{ row.host }}:{{ row.port || 22 }}</template>
      </el-table-column>
      <el-table-column label="Credential" min-width="150">
        <template #default="{ row }">{{ row.credential?.name || '-' }}</template>
      </el-table-column>
      <el-table-column prop="networkZone" label="Network Zone" min-width="140" />
      <el-table-column :label="at('refAssetsColumn')" min-width="210">
        <template #default="{ row }">
          <span>Host {{ row.hostCount || 0 }}</span>
          <el-divider direction="vertical" />
          <span>Database {{ row.databaseCount || 0 }}</span>
          <el-divider direction="vertical" />
          <span>K8s {{ row.clusterCount || 0 }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="at('status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? at('enabled') : at('disabled') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="at('connStatusColumn')" width="120">
        <template #default="{ row }">
          <el-tag :type="row.connectStatus === 1 ? 'success' : row.connectStatus === 2 ? 'danger' : 'info'">
            {{ row.connectStatus === 1 ? at('groupNormal') : row.connectStatus === 2 ? at('statusFailed') : at('notInspected') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="at('lastCheckColumn')" min-width="190"><template #default="{ row }">{{ formatDateTime(row.lastCheckTime) }}</template></el-table-column>
      <el-table-column :label="at('actions')" fixed="right" width="260">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">{{ at('testButton') }}</el-button>
          <el-button link type="primary" @click="openEdit(row)">{{ at('edit') }}</el-button>
          <el-button link :type="row.status === 1 ? 'warning' : 'success'" @click="toggleStatus(row)">
            {{ row.status === 1 ? at('disabled') : at('enabled') }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ at('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        layout="total, prev, pager, next"
        :total="total"
        @current-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? at('editGatewayTitle') : at('addGatewayButton')" width="720px">
      <el-form label-width="110px">
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item :label="at('gatewayNameColumn')" required>
              <el-input v-model="form.name" :placeholder="at('gatewayNamePlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Gateway Code">
              <el-input v-model="form.code" :placeholder="at('codeOptionalPlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('gatewayAddrColumn')" required>
              <el-input v-model="form.host" :placeholder="at('ipOrDomainPlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('sshPortLabel')" required>
              <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Login Credential" required>
              <el-select v-model="form.credentialId" filterable :placeholder="at('selectGatewayCredential')" style="width: 100%">
                <el-option v-for="item in credentials" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Network Zone">
              <el-input v-model="form.networkZone" :placeholder="at('networkZonePlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="at('noteLabel')">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" @click="saveGateway">{{ at('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.gateway-page {
  padding: 0;
}

.page-hero,
.toolbar,
.gateway-table {
  background: #fff;
  border: 1px solid #e4ebf7;
  border-radius: 10px;
}

.page-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  margin-bottom: 16px;
}

.page-hero h1 {
  margin: 0 0 8px;
  font-size: 22px;
  font-weight: 650;
}

.page-hero p {
  margin: 0;
  color: #6b7a99;
}

.toolbar {
  display: flex;
  gap: 12px;
  padding: 12px;
  background: #f9fafc;
  margin-bottom: 16px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  padding: 16px 0;
}
</style>
