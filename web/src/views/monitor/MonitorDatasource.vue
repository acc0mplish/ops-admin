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
    ElMessage.warning('Datasource 이름과 주소를 입력하십시오.')
    return
  }
  saving.value = true
  try {
    await saveMonitorDatasource(form)
    ElMessage.success('저장했습니다.')
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
    ElMessage.success('Datasource 연결에 성공했습니다.')
    if (payload.id) await loadData()
  } finally {
    testing.value = false
  }
}

function healthType(status) {
  return ({ healthy: 'success', unhealthy: 'danger', unknown: 'info' }[status] || 'info')
}

function healthText(status) {
  return ({ healthy: '정상', unhealthy: '비정상', unknown: '미검사' }[status] || '미검사')
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Datasource “${row.name}”을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteMonitorDatasource(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-page monitor-datasource-page">
    <div class="page-header">
      <div>
        <h2>Datasource 관리</h2>
        <p>Prometheus, VictoriaMetrics, Elasticsearch, VictoriaLogs 또는 Jaeger를 연결해 Metric, 로그, Trace 접점을 통합 관리합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">Datasource 추가</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="이름 / 주소 검색" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.type" clearable placeholder="유형" style="width: 170px">
        <el-option label="Prometheus" value="prometheus" />
        <el-option label="VictoriaMetrics" value="victoriametrics" />
        <el-option label="VictoriaLogs" value="victorialogs" />
        <el-option label="Elasticsearch" value="elasticsearch" />
        <el-option label="Jaeger" value="jaeger" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="상태" style="width: 130px">
        <el-option label="활성화" value="1" />
        <el-option label="비활성화" value="2" />
      </el-select>
      <el-select v-model="query.env" clearable placeholder="전체 Environment" style="width: 150px" @change="loadData">
        <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="이름" min-width="180" />
      <el-table-column prop="type" label="유형" width="150" />
      <el-table-column label="Environment" width="120">
        <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.env) }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="url" label="주소" min-width="260" show-overflow-tooltip />
      <el-table-column label="인증" width="120">
        <template #default="{ row }">{{ row.authType || 'none' }}</template>
      </el-table-column>
      <el-table-column label="Health 상태" width="120">
        <template #default="{ row }"><el-tag :type="healthType(row.healthStatus)" effect="light">{{ healthText(row.healthStatus) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="Query 지연" width="110"><template #default="{ row }">{{ row.lastCheckAt ? `${row.latencyMs || 0} ms` : '-' }}</template></el-table-column>
      <el-table-column label="최근 검사" width="180"><template #default="{ row }">{{ formatTime(row.lastCheckAt) }}</template></el-table-column>
      <el-table-column prop="lastError" label="최근 오류" min-width="180" show-overflow-tooltip />
      <el-table-column label="상태" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="작업" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">테스트</el-button>
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Datasource 수정' : 'Datasource 추가'" width="760px">
      <el-form label-width="110px">
        <el-form-item label="이름" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="유형">
          <el-radio-group v-model="form.type">
            <el-radio-button label="prometheus">Prometheus</el-radio-button>
            <el-radio-button label="victoriametrics">VictoriaMetrics</el-radio-button>
            <el-radio-button label="victorialogs">VictoriaLogs</el-radio-button>
            <el-radio-button label="elasticsearch">Elasticsearch</el-radio-button>
            <el-radio-button label="jaeger">Jaeger</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="소속 Environment" required>
          <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" placeholder="Environment 선택">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item label="주소" required><el-input v-model="form.url" :placeholder="form.type === 'elasticsearch' ? 'http://elasticsearch:9200' : (form.type === 'victorialogs' ? 'http://victorialogs:9428' : (form.type === 'jaeger' ? 'http://jaeger-query:16686' : 'http://prometheus:9090'))" /></el-form-item>
        <el-alert v-if="form.type === 'elasticsearch'" title="Elasticsearch Datasource는 로그 Query와 로그 Alert에 사용됩니다. PromQL Instant Query, 모니터링 Dashboard, Inspection Dashboard에서는 여전히 Prometheus 또는 VictoriaMetrics를 선택해야 합니다." type="info" :closable="false" style="margin-bottom: 18px" />
        <el-alert v-else-if="form.type === 'victorialogs'" title="VictoriaLogs Datasource는 LogsQL 로그 Query에 사용됩니다. PromQL Instant Query, 모니터링 Dashboard, Inspection Dashboard에서는 여전히 Prometheus 또는 VictoriaMetrics를 선택해야 합니다." type="info" :closable="false" style="margin-bottom: 18px" />
        <el-alert v-else-if="form.type === 'jaeger'" title="Jaeger Datasource는 Trace 연결 관리와 Health Check에 사용됩니다. 현재 PromQL, 로그 Query, 모니터링 Dashboard에서는 이 Datasource를 사용하지 않습니다. Jaeger Query 서비스 주소(예: http://jaeger-query:16686)를 입력하십시오." type="info" :closable="false" style="margin-bottom: 18px" />
        <el-form-item label="인증 방식">
          <el-select v-model="form.authType" style="width: 100%">
            <el-option label="인증 없음" value="none" />
            <el-option label="Basic" value="basic" />
            <el-option label="Bearer Token" value="bearer" />
            <el-option label="Elasticsearch API Key" value="apikey" />
          </el-select>
        </el-form-item>
        <template v-if="form.authType === 'basic'">
          <el-form-item label="사용자 이름"><el-input v-model="form.username" /></el-form-item>
          <el-form-item label="비밀번호"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        </template>
        <el-form-item v-if="form.authType === 'bearer' || form.authType === 'apikey'" :label="form.authType === 'apikey' ? 'API Key' : 'Token'">
          <el-input v-model="form.token" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">활성화</el-radio>
            <el-radio :value="2">비활성화</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="설명"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button :loading="testing" @click="handleTest()">연결 테스트</el-button>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="saving" @click="submit">저장</el-button>
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
