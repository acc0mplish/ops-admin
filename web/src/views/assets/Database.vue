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
  if (row.connectStatus === 1) return '연결됨'
  if (row.connectStatus === 2) return '연결 오류'
  return '미검사'
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
    ElMessage.success(`연결에 성공했습니다. ${databaseTypeLabel(data.dbType || form.dbType)} 버전 ${data.version || '-'}`)
  } finally {
    testing.value = false
  }
}

async function submit() {
  if (form.connectionMode === 'gateway' && !form.gatewayId) {
    ElMessage.warning('접속 Gateway를 선택하십시오.')
    return
  }
  if (isEdit.value) {
    await updateAssetDatabase(form)
    ElMessage.success('Database Asset을 업데이트했습니다.')
  } else {
    await addAssetDatabase(form)
    ElMessage.success('Database Asset을 생성했습니다.')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Database Asset “${row.name}”을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteAssetDatabase(row.id)
  ElMessage.success('삭제했습니다.')
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
        <h2 class="page-title">Database 관리</h2>
        <p class="page-desc">MySQL, PostgreSQL, MongoDB, Redis Asset을 통합 관리하고 Database Type별로 연결, 구조, Workbench 기능을 제공합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">Database 추가</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="Database 이름 / Host / DB 이름 검색"
          style="width: 260px"
          @keyup.enter="loadData"
        />
        <el-select v-model="query.dbType" clearable style="width: 140px" placeholder="Database Type">
          <el-option label="전체 Type" value="" />
          <el-option label="MySQL" value="mysql" />
          <el-option label="PostgreSQL" value="postgresql" />
          <el-option label="MongoDB" value="mongodb" />
          <el-option label="Redis" value="redis" />
        </el-select>
        <el-select v-model="query.status" clearable style="width: 140px" placeholder="상태">
          <el-option label="활성화" value="1" />
          <el-option label="비활성화" value="2" />
        </el-select>
        <el-select v-model="query.env" clearable style="width: 160px" placeholder="전체 Environment" @change="loadData">
          <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
        </el-select>
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button @click="Object.assign(query, { pageNum: 1, pageSize: 10, keyword: '', dbType: '', status: '', env: '' }); loadData()">초기화</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border class="data-table">
      <el-table-column label="Database 이름" min-width="180">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ row.name }}</el-button>
        </template>
      </el-table-column>
      <el-table-column label="Type" width="120">
        <template #default="{ row }"><el-tag effect="plain">{{ databaseTypeLabel(row.dbType) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="접속 모드" width="110">
        <template #default="{ row }">
          <el-tag :type="row.accessMode === 'readonly' ? 'warning' : 'success'" effect="plain">
            {{ row.accessMode === 'readonly' ? '읽기 전용' : '읽기/쓰기' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Environment" width="120">
        <template #default="{ row }">
          <el-tag effect="plain">{{ environmentName(row.env) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="연결 주소" min-width="220">
        <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
      </el-table-column>
      <el-table-column label="접속 방식" min-width="150">
        <template #default="{ row }">
          <span v-if="row.connectionMode === 'gateway'">Gateway: {{ row.gateway?.name || '-' }}</span>
          <span v-else>직접 연결</span>
        </template>
      </el-table-column>
      <el-table-column prop="dbName" label="기본 DB" min-width="140" />
      <el-table-column prop="username" label="계정" min-width="120" />
      <el-table-column prop="version" label="버전" min-width="140" />
      <el-table-column label="연결 상태" width="110">
        <template #default="{ row }">
          <el-tag :type="connectionStatusType(row)" effect="light">{{ connectionStatusText(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="비고" min-width="180" />
      <el-table-column label="작업" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openWorkbench(row)">Workbench</el-button>
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Database Asset 수정' : 'Database Asset 추가'" width="720px">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Database 이름"><el-input v-model="form.name" placeholder="예: order-prod" /></el-form-item>
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
            <el-form-item label="소속 Environment" required>
              <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" placeholder="Environment를 선택하십시오.">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="접속 모드" required>
              <el-radio-group v-model="form.accessMode">
                <el-radio-button value="readonly">읽기 전용</el-radio-button>
                <el-radio-button value="readwrite">읽기/쓰기</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="모니터링 Dashboard">
              <el-switch v-model="form.monitorEnabled" active-text="활성화" inactive-text="비활성화" />
              <div class="form-tip">활성화하면 상세 Page에서 Database Native Query로 운영 Metric을 수집합니다.</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Host 주소"><el-input v-model="form.host" placeholder="예: 10.0.0.12" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Port"><el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="form.dbType === 'redis' || form.dbType === 'mongodb' ? '사용자 이름 (선택)' : '사용자 이름'">
              <el-input v-model="form.username" :placeholder="form.dbType === 'redis' || form.dbType === 'mongodb' ? '인증을 사용하지 않으면 비워 둘 수 있습니다' : ''" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="비밀번호"><el-input v-model="form.password" show-password :placeholder="isEdit ? '비워 두면 변경되지 않습니다' : ''" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="form.dbType === 'redis' ? '논리 DB 번호' : '기본 DB'">
              <el-input v-model="form.dbName" :placeholder="form.dbType === 'redis' ? '예: 0' : form.dbType === 'mongodb' ? '예: admin' : '예: app_db'" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Charset">
              <el-input v-model="form.charset" :disabled="form.dbType === 'mongodb' || form.dbType === 'redis'" placeholder="기본값 utf8mb4" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="연결 방식">
              <el-radio-group v-model="form.connectionMode">
                <el-radio-button label="direct">직접 연결</el-radio-button>
                <el-radio-button label="gateway">Gateway를 통해 연결</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col v-if="form.connectionMode === 'gateway'" :span="12">
            <el-form-item label="접속 Gateway" required>
              <el-select v-model="form.gatewayId" filterable placeholder="Gateway를 선택하십시오." style="width: 100%">
                <el-option v-for="item in gatewayOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="비고"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="상태">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">활성화</el-radio>
                <el-radio :value="2">비활성화</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button :loading="testing" @click="handleTest">연결 테스트</el-button>
        <el-button type="primary" @click="submit">저장</el-button>
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
