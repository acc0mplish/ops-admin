<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteMonitorDatasource,
  monitorDatasourceInfo,
  queryMonitorDatasourceList,
  saveMonitorDatasource,
  testMonitorDatasource
} from '../../api/monitor'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import { mt } from '../../utils/monitor-i18n'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', type: '', status: '', env: '' })
const form = reactive({
  id: undefined,
  name: '',
  type: 'prometheus',
  env: 'test',
  url: '',
  authType: 'none',
  username: '',
  password: '',
  token: '',
  isDefault: false,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    type: 'prometheus',
    env: 'test',
    url: '',
    authType: 'none',
    username: '',
    password: '',
    token: '',
    isDefault: false,
    status: 1,
    description: ''
  })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorDatasourceList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await monitorDatasourceInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.url.trim()) {
    ElMessage.warning(mt('enterDsRequired'))
    return
  }
  saving.value = true
  try {
    await saveMonitorDatasource(form)
    ElMessage.success(mt('savedMsg'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleTest(payload = form) {
  testing.value = true
  try {
    await testMonitorDatasource(payload)
    ElMessage.success(mt('dsConnectOk'))
    if (payload.id) await loadData()
  } finally {
    testing.value = false
  }
}

function healthType(status) {
  return ({ healthy: 'success', unhealthy: 'danger', unknown: 'info' }[status] || 'info')
}

function healthText(status) {
  return ({ healthy: mt('healthOk'), unhealthy: mt('healthBad'), unknown: mt('healthUnknown') }[status] || mt('healthUnknown'))
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

async function handleDelete(row) {
  await ElMessageBox.confirm(mt('dsDeleteConfirm', { name: row.name }), mt('noticeTitle'), { type: 'warning' })
  await deleteMonitorDatasource(row.id)
  ElMessage.success(mt('deletedMsg'))
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-page monitor-datasource-page">
    <div class="page-header">
      <div>
        <h2>{{ mt('dsManageTitle') }}</h2>
        <p>{{ mt('dsPageDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ mt('addDatasource') }}</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="mt('dsSearchPlaceholder')" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.type" clearable :placeholder="mt('typePlaceholder')" style="width: 170px">
        <el-option label="Prometheus" value="prometheus" />
        <el-option label="VictoriaMetrics" value="victoriametrics" />
        <el-option label="VictoriaLogs" value="victorialogs" />
        <el-option label="Elasticsearch" value="elasticsearch" />
        <el-option label="Jaeger" value="jaeger" />
      </el-select>
      <el-select v-model="query.status" clearable :placeholder="mt('status')" style="width: 130px">
        <el-option :label="mt('enabledOption')" value="1" />
        <el-option :label="mt('disabledOption')" value="2" />
      </el-select>
      <el-select v-model="query.env" clearable :placeholder="mt('allEnvPlaceholder')" style="width: 150px" @change="loadData">
        <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ mt('searchLabel') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" :label="mt('nameLabel')" min-width="180" />
      <el-table-column prop="type" :label="mt('typeCol')" width="150" />
      <el-table-column label="Environment" width="120">
        <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.env) }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="url" :label="mt('urlCol')" min-width="260" show-overflow-tooltip />
      <el-table-column :label="mt('authCol')" width="120">
        <template #default="{ row }">{{ row.authType || 'none' }}</template>
      </el-table-column>
      <el-table-column :label="mt('healthStatusCol')" width="120">
        <template #default="{ row }"><el-tag :type="healthType(row.healthStatus)" effect="light">{{ healthText(row.healthStatus) }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="mt('queryLatencyCol')" width="110"><template #default="{ row }">{{ row.lastCheckAt ? `${row.latencyMs || 0} ms` : '-' }}</template></el-table-column>
      <el-table-column :label="mt('lastCheckCol')" width="180"><template #default="{ row }">{{ formatTime(row.lastCheckAt) }}</template></el-table-column>
      <el-table-column prop="lastError" :label="mt('lastErrorCol')" min-width="180" show-overflow-tooltip />
      <el-table-column :label="mt('status')" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? mt('enabledOption') : mt('disabledOption') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="mt('actions')" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">{{ mt('testLabel') }}</el-button>
          <el-button link type="primary" @click="openEdit(row)">{{ mt('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ mt('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? mt('editDatasource') : mt('addDatasource')" width="760px">
      <el-form label-width="110px">
        <el-form-item :label="mt('nameLabel')" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="mt('typeCol')">
          <el-radio-group v-model="form.type">
            <el-radio-button label="prometheus">Prometheus</el-radio-button>
            <el-radio-button label="victoriametrics">VictoriaMetrics</el-radio-button>
            <el-radio-button label="victorialogs">VictoriaLogs</el-radio-button>
            <el-radio-button label="elasticsearch">Elasticsearch</el-radio-button>
            <el-radio-button label="jaeger">Jaeger</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="mt('envLabel')" required>
          <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" :placeholder="mt('selectEnv')">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item :label="mt('urlCol')" required><el-input v-model="form.url" :placeholder="form.type === 'elasticsearch' ? 'http://elasticsearch:9200' : (form.type === 'victorialogs' ? 'http://victorialogs:9428' : (form.type === 'jaeger' ? 'http://jaeger-query:16686' : 'http://prometheus:9090'))" /></el-form-item>
        <el-alert v-if="form.type === 'elasticsearch'" :title="mt('esDsAlert')" type="info" :closable="false" style="margin-bottom: 18px" />
        <el-alert v-else-if="form.type === 'victorialogs'" :title="mt('vlDsAlert')" type="info" :closable="false" style="margin-bottom: 18px" />
        <el-alert v-else-if="form.type === 'jaeger'" :title="mt('jaegerDsAlert')" type="info" :closable="false" style="margin-bottom: 18px" />
        <el-form-item :label="mt('authMethod')">
          <el-select v-model="form.authType" style="width: 100%">
            <el-option :label="mt('noAuth')" value="none" />
            <el-option label="Basic" value="basic" />
            <el-option label="Bearer Token" value="bearer" />
            <el-option label="Elasticsearch API Key" value="apikey" />
          </el-select>
        </el-form-item>
        <template v-if="form.authType === 'basic'">
          <el-form-item :label="mt('usernameLabel')"><el-input v-model="form.username" /></el-form-item>
          <el-form-item :label="mt('passwordLabel')"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        </template>
        <el-form-item v-if="form.authType === 'bearer' || form.authType === 'apikey'" :label="form.authType === 'apikey' ? 'API Key' : 'Token'">
          <el-input v-model="form.token" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item :label="mt('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ mt('enabledOption') }}</el-radio>
            <el-radio :value="2">{{ mt('disabledOption') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button :loading="testing" @click="handleTest()">{{ mt('testConnection') }}</el-button>
        <el-button @click="dialogVisible = false">{{ mt('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ mt('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.page-header p { margin: 0; color: #7282a0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }
.pager { display: flex; justify-content: flex-end; }
</style>
