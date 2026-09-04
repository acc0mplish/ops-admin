<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
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
    ElMessage.warning('Gateway 이름, 주소, Credential을 입력하십시오.')
    return
  }
  if (isEdit.value) {
    await updateAssetGateway(form)
    ElMessage.success('Gateway를 수정했습니다.')
  } else {
    await addAssetGateway(form)
    ElMessage.success('Gateway를 생성했습니다.')
  }
  dialogVisible.value = false
  loadData()
}

async function handleTest(row) {
  await testAssetGateway(row.id)
  ElMessage.success('Gateway 연결이 정상입니다.')
  loadData()
}

async function toggleStatus(row) {
  await updateAssetGatewayStatus({ id: row.id, status: row.status === 1 ? 2 : 1 })
  ElMessage.success(row.status === 1 ? 'Gateway를 비활성화했습니다.' : 'Gateway를 활성화했습니다.')
  loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Gateway “${row.name}”을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' })
  await deleteAssetGateway(row.id)
  ElMessage.success('Gateway를 삭제했습니다.')
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
        <h1>Gateway 관리</h1>
        <p>SSH 점프 Gateway를 관리하여 내부 Host, Database, K8s Cluster에 통일된 접속 창구를 제공합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">Gateway 추가</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" placeholder="이름 / 주소 / Network Zone 검색" clearable style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" placeholder="상태" clearable style="width: 140px">
        <el-option label="활성화" value="1" />
        <el-option label="비활성화" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
      <el-button @click="Object.assign(query, { keyword: '', status: '', pageNum: 1 }); loadData()">초기화</el-button>
    </div>

    <el-table v-loading="loading" :data="list" class="gateway-table">
      <el-table-column prop="name" label="Gateway 이름" min-width="160" />
      <el-table-column prop="host" label="Gateway 주소" min-width="170">
        <template #default="{ row }">{{ row.host }}:{{ row.port || 22 }}</template>
      </el-table-column>
      <el-table-column label="Credential" min-width="150">
        <template #default="{ row }">{{ row.credential?.name || '-' }}</template>
      </el-table-column>
      <el-table-column prop="networkZone" label="Network Zone" min-width="140" />
      <el-table-column label="참조 Asset" min-width="210">
        <template #default="{ row }">
          <span>Host {{ row.hostCount || 0 }}</span>
          <el-divider direction="vertical" />
          <span>Database {{ row.databaseCount || 0 }}</span>
          <el-divider direction="vertical" />
          <span>K8s {{ row.clusterCount || 0 }}</span>
        </template>
      </el-table-column>
      <el-table-column label="상태" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="연결 상태" width="120">
        <template #default="{ row }">
          <el-tag :type="row.connectStatus === 1 ? 'success' : row.connectStatus === 2 ? 'danger' : 'info'">
            {{ row.connectStatus === 1 ? '정상' : row.connectStatus === 2 ? '실패' : '미검사' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="최근 검사" min-width="190"><template #default="{ row }">{{ formatDateTime(row.lastCheckTime) }}</template></el-table-column>
      <el-table-column label="작업" fixed="right" width="260">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">테스트</el-button>
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link :type="row.status === 1 ? 'warning' : 'success'" @click="toggleStatus(row)">
            {{ row.status === 1 ? '비활성화' : '활성화' }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Gateway 수정' : 'Gateway 추가'" width="720px">
      <el-form label-width="110px">
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item label="Gateway 이름" required>
              <el-input v-model="form.name" placeholder="예: prod-bastion" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Gateway Code">
              <el-input v-model="form.code" placeholder="선택 사항. 예: prod-vpc-a" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Gateway 주소" required>
              <el-input v-model="form.host" placeholder="IP 또는 Domain" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="SSH 포트" required>
              <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Login Credential" required>
              <el-select v-model="form.credentialId" filterable placeholder="Gateway Credential 선택" style="width: 100%">
                <el-option v-for="item in credentials" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Network Zone">
              <el-input v-model="form.networkZone" placeholder="예: prod-vpc / idc-a" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="비고">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" @click="saveGateway">저장</el-button>
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
