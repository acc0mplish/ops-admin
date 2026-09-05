<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addAssetDatabase,
  assetDatabaseInfo,
  deleteAssetDatabase,
  queryAssetDatabaseList,
  queryAssetGatewayOptions,
  testAssetDatabase,
  updateAssetDatabase
} from '../../api/asset'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import { at } from '../../utils/asset-i18n'

const router = useRouter()

const loading = ref(false)
const testing = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const total = ref(0)
const gatewayOptions = ref([])
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  dbType: '',
  status: '',
  env: ''
})

const form = reactive({
  id: undefined,
  name: '',
  dbType: 'mysql',
  host: '',
  port: 3306,
  username: '',
  password: '',
  connectionMode: 'direct',
  gatewayId: undefined,
  dbName: '',
  charset: 'utf8mb4',
  env: 'test',
  accessMode: 'readwrite',
  monitorEnabled: false,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    dbType: 'mysql',
    host: '',
    port: 3306,
    username: '',
    password: '',
    connectionMode: 'direct',
    gatewayId: undefined,
    dbName: '',
    charset: 'utf8mb4',
    env: 'test',
    accessMode: 'readwrite',
    monitorEnabled: false,
    status: 1,
    description: ''
  })
}

const databaseTypeOptions = [
  { label: 'MySQL', value: 'mysql', port: 3306, charset: 'utf8mb4', dbName: 'mysql' },
  { label: 'PostgreSQL', value: 'postgresql', port: 5432, charset: 'UTF8', dbName: 'postgres' },
  { label: 'MongoDB', value: 'mongodb', port: 27017, charset: '', dbName: 'admin' },
  { label: 'Redis', value: 'redis', port: 6379, charset: '', dbName: '0' }
]

function databaseTypeLabel(value) {
  return databaseTypeOptions.find((item) => item.value === value)?.label || value || '-'
}

function onDatabaseTypeChange(value) {
  const option = databaseTypeOptions.find((item) => item.value === value)
  if (!option) return
  form.port = option.port
  form.charset = option.charset
  if (!form.dbName || ['mysql', 'postgres', 'admin', '0'].includes(form.dbName)) form.dbName = option.dbName
}

function connectionStatusText(row) {
  if (row.connectStatus === 1) return at('dbConnected')
  if (row.connectStatus === 2) return at('dbConnectError')
  return at('notInspected')
}

function connectionStatusType(row) {
  if (row.connectStatus === 1) return 'success'
  if (row.connectStatus === 2) return 'danger'
  return 'info'
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetDatabaseList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadGateways() {
  gatewayOptions.value = await queryAssetGatewayOptions()
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await assetDatabaseInfo(row.id)
  Object.assign(form, data, { password: '' })
  dialogVisible.value = true
}

async function handleTest() {
  testing.value = true
  try {
    const data = await testAssetDatabase(form)
    ElMessage.success(at('testSuccess', { type: databaseTypeLabel(data.dbType || form.dbType), version: data.version || '-' }))
  } finally {
    testing.value = false
  }
}

async function submit() {
  if (form.connectionMode === 'gateway' && !form.gatewayId) {
    ElMessage.warning(at('selectGatewayWarning'))
    return
  }
  if (isEdit.value) {
    await updateAssetDatabase(form)
    ElMessage.success(at('dbAssetUpdated'))
  } else {
    await addAssetDatabase(form)
    ElMessage.success(at('dbAssetCreated'))
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(at('deleteDbConfirm', { name: row.name }), at('notice'), { type: 'warning' })
  await deleteAssetDatabase(row.id)
  ElMessage.success(at('rowDeleted'))
  await loadData()
}

function openWorkbench(row) {
  router.push({ name: 'DatabaseWorkbench', params: { id: row.id } })
}

function openDetail(row) {
  router.push({ name: 'AssetDatabaseDetail', params: { id: row.id } })
}

function formatDateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false }).replaceAll('/', '-')
}

onMounted(() => {
  loadGateways()
  loadData()
})
</script>

<template>
  <div class="database-page page-card asset-card-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ at('dbManageTitle') }}</h2>
        <p class="page-desc">{{ at('dbManageDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ at('addDbButton') }}</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="query.keyword"
          clearable
          :placeholder="at('dbSearchPlaceholder')"
          style="width: 260px"
          @keyup.enter="loadData"
        />
        <el-select v-model="query.dbType" clearable style="width: 140px" placeholder="Database Type">
          <el-option :label="at('allTypesOption')" value="" />
          <el-option label="MySQL" value="mysql" />
          <el-option label="PostgreSQL" value="postgresql" />
          <el-option label="MongoDB" value="mongodb" />
          <el-option label="Redis" value="redis" />
        </el-select>
        <el-select v-model="query.status" clearable style="width: 140px" :placeholder="at('statusSelect')">
          <el-option :label="at('enabled')" value="1" />
          <el-option :label="at('disabled')" value="2" />
        </el-select>
        <el-select v-model="query.env" clearable style="width: 160px" :placeholder="at('allEnvironments')" @change="loadData">
          <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ at('search') }}</el-button>
        <el-button @click="Object.assign(query, { pageNum: 1, pageSize: 10, keyword: '', dbType: '', status: '', env: '' }); loadData()">{{ at('reset') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border class="data-table">
      <el-table-column :label="at('dbTableName')" min-width="180">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ row.name }}</el-button>
        </template>
      </el-table-column>
      <el-table-column label="Type" width="120">
        <template #default="{ row }"><el-tag effect="plain">{{ databaseTypeLabel(row.dbType) }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="at('dbAccessModeColumn')" width="110">
        <template #default="{ row }">
          <el-tag :type="row.accessMode === 'readonly' ? 'warning' : 'success'" effect="plain">
            {{ row.accessMode === 'readonly' ? at('readOnly') : at('readWrite') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Environment" width="120">
        <template #default="{ row }">
          <el-tag effect="plain">{{ environmentName(row.env) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="at('connAddrColumn')" min-width="220">
        <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
      </el-table-column>
      <el-table-column :label="at('accessModeLabel')" min-width="150">
        <template #default="{ row }">
          <span v-if="row.connectionMode === 'gateway'">Gateway: {{ row.gateway?.name || '-' }}</span>
          <span v-else>{{ at('directConnection') }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="dbName" :label="at('defaultDbColumn')" min-width="140" />
      <el-table-column prop="username" :label="at('accountColumn')" min-width="120" />
      <el-table-column prop="version" :label="at('versionColumn')" min-width="140" />
      <el-table-column :label="at('connStatusColumn')" width="110">
        <template #default="{ row }">
          <el-tag :type="connectionStatusType(row)" effect="light">{{ connectionStatusText(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" :label="at('noteLabel')" min-width="180" />
      <el-table-column :label="at('actions')" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openWorkbench(row)">Workbench</el-button>
          <el-button link type="primary" @click="openEdit(row)">{{ at('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ at('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        :total="total"
        layout="total, sizes, prev, pager, next"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? at('editDbTitle') : at('addDbTitle')" width="720px">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="at('dbTableName')"><el-input v-model="form.name" :placeholder="at('dbNamePlaceholder')" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Database Type">
              <el-select v-model="form.dbType" style="width: 100%" @change="onDatabaseTypeChange">
                <el-option label="MySQL" value="mysql" />
                <el-option label="PostgreSQL" value="postgresql" />
                <el-option label="MongoDB" value="mongodb" />
                <el-option label="Redis" value="redis" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('environmentLabel')" required>
              <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" :placeholder="at('selectEnvironmentFull')">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('dbAccessModeColumn')" required>
              <el-radio-group v-model="form.accessMode">
                <el-radio-button value="readonly">{{ at('readOnly') }}</el-radio-button>
                <el-radio-button value="readwrite">{{ at('readWrite') }}</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('monitoringDashboardLabel')">
              <el-switch v-model="form.monitorEnabled" :active-text="at('enabled')" :inactive-text="at('disabled')" />
              <div class="form-tip">{{ at('monitorTip') }}</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('hostAddressPlaceholder')"><el-input v-model="form.host" :placeholder="at('dbHostPlaceholder')" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Port"><el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="form.dbType === 'redis' || form.dbType === 'mongodb' ? at('usernameOptionalLabel') : at('usernameLabel')">
              <el-input v-model="form.username" :placeholder="form.dbType === 'redis' || form.dbType === 'mongodb' ? at('usernameHint') : ''" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('passwordLabel')"><el-input v-model="form.password" show-password :placeholder="isEdit ? at('passwordPlaceholder') : ''" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="form.dbType === 'redis' ? at('logicalDbLabel') : at('defaultDbColumn')">
              <el-input v-model="form.dbName" :placeholder="form.dbType === 'redis' ? at('redisDbPlaceholder') : form.dbType === 'mongodb' ? at('mongoDbPlaceholder') : at('defaultDbPlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Charset">
              <el-input v-model="form.charset" :disabled="form.dbType === 'mongodb' || form.dbType === 'redis'" :placeholder="at('charsetPlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="at('connectionModeLabel')">
              <el-radio-group v-model="form.connectionMode">
                <el-radio-button label="direct">{{ at('directConnection') }}</el-radio-button>
                <el-radio-button label="gateway">{{ at('viaGatewayLine') }}</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col v-if="form.connectionMode === 'gateway'" :span="12">
            <el-form-item :label="at('accessGatewayLabel')" required>
              <el-select v-model="form.gatewayId" filterable :placeholder="at('selectGatewayFull')" style="width: 100%">
                <el-option v-for="item in gatewayOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="at('noteLabel')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="at('status')">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">{{ at('enabled') }}</el-radio>
                <el-radio :value="2">{{ at('disabled') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button :loading="testing" @click="handleTest">{{ at('testConnection') }}</el-button>
        <el-button type="primary" @click="submit">{{ at('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.database-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 650;
}

.page-desc {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
}

.toolbar,
.toolbar-left {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar { padding: 12px; border: 1px solid #e8edf3; border-radius: 9px; background: #f9fafc; }

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid #edf0f5;
}

.form-tip { margin-top: 6px; color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.35; }
</style>
