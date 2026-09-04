<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsApplication, queryOpsApplicationList, saveOpsApplication } from '../../api/ops'
import { queryAssetCredentialOptions } from '../../api/asset'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const rows = ref([])
const total = ref(0)
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()
const credentialOptions = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  serviceType: '',
  env: ''
})

const form = reactive({
  id: undefined,
  name: '',
  code: '',
  serviceType: '백엔드 서비스',
  repoType: 'git',
  repoUrl: '',
  repoCredentialId: undefined,
  branch: 'master',
  env: 'test',
  status: 1,
  description: ''
})

function isSVNRepository() { return form.repoType === 'svn' }

function handleRepoTypeChange(repoType) {
  if (repoType === 'svn' && (!form.branch || form.branch === 'master' || form.branch === 'main')) form.branch = 'HEAD'
  if (repoType === 'git' && (!form.branch || form.branch === 'HEAD')) form.branch = 'master'
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    code: '',
    serviceType: '백엔드 서비스',
    repoType: 'git',
    repoUrl: '',
    repoCredentialId: undefined,
    branch: 'master',
    env: 'test',
    status: 1,
    description: ''
  })
}

async function loadCredentialOptions() {
  const credentials = await queryAssetCredentialOptions()
  credentialOptions.value = credentials || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsApplicationList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  Object.assign(form, {
    id: row.id,
    name: row.name || '',
    code: row.code || '',
    serviceType: row.serviceType || row.env || '백엔드 서비스',
    repoType: row.repoType || 'git',
    repoUrl: row.repoUrl || '',
    repoCredentialId: row.repoCredentialId || undefined,
    branch: row.branch || (row.repoType === 'svn' ? 'HEAD' : 'master'),
    env: row.env || 'test',
    status: row.status || 1,
    description: row.description || ''
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name || !form.code || !form.repoUrl) {
    ElMessage.warning('Application 이름, Application Code와 Repository 주소를 입력하십시오.')
    return
  }
  if (isSVNRepository() && form.branch && !/^(HEAD|\d+)$/i.test(form.branch.trim())) {
    ElMessage.warning('SVN Version은 HEAD 또는 숫자 Revision만 지원합니다')
    return
  }
  saving.value = true
  try {
    await saveOpsApplication({ ...form })
    ElMessage.success('저장했습니다.')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`Application "${row.name}"을(를) 삭제하시겠습니까?`, 'Application 삭제', { type: 'warning' })
  await deleteOpsApplication(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

function statusType(status) {
  return Number(status) === 1 ? 'success' : 'info'
}

onMounted(async () => { await Promise.all([loadCredentialOptions(), loadData()]) })
</script>

<template>
  <div class="app-page">
    <div class="app-header">
      <div>
        <h1>Application 관리</h1>
        <p>Application Repository를 통합 관리하고 Environment별로 Host, K8s, Database, Gateway, Monitoring Resource를 바인딩합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">+ 새 Application</el-button>
    </div>

    <div class="filter-panel">
      <el-form inline>
        <el-form-item label="Application 이름">
          <el-input v-model="query.keyword" clearable placeholder="Application 이름 / Repository 주소를 입력하십시오" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="서비스 유형">
          <el-select v-model="query.serviceType" clearable placeholder="서비스 유형을 선택하십시오">
            <el-option label="프론트엔드 서비스" value="프론트엔드 서비스" />
            <el-option label="백엔드 서비스" value="백엔드 서비스" />
            <el-option label="미들웨어" value="미들웨어" />
          </el-select>
        </el-form-item>
        <el-form-item label="Environment">
          <el-select v-model="query.env" clearable placeholder="전체 Environment" @change="loadData">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">검색</el-button>
          <el-button @click="Object.assign(query, { keyword: '', serviceType: '', env: '', pageNum: 1 }); loadData()">초기화</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column prop="name" label="Application 이름" min-width="170">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.code }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="비즈니스 기능" min-width="170" show-overflow-tooltip />
        <el-table-column prop="serviceType" label="서비스 유형" width="120">
          <template #default="{ row }">{{ row.serviceType || row.env || '-' }}</template>
        </el-table-column>
        <el-table-column label="기본 Environment" width="120">
          <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.env) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="repoUrl" label="Repository 주소" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.repoType || 'git' }}</el-tag>
            <span class="repo-url">{{ row.repoUrl }}</span>
          </template>
        </el-table-column>
        <el-table-column label="상태" width="90">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ Number(row.status) === 1 ? '정상' : '비활성화' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="생성자" width="100">관리자</el-table-column>
        <el-table-column prop="createTime" label="생성 시각" min-width="170" />
        <el-table-column label="작업" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">조회</el-button>
            <el-button link type="primary" @click="openEdit(row)">수정</el-button>
            <el-button link type="danger" @click="remove(row)">삭제</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? 'Application 수정' : '새 Application'" width="min(1280px, 94vw)" top="4vh" class="app-project-dialog">
      <el-form :model="form" label-width="96px">
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="Application 이름" required><el-input v-model="form.name" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Application Code" required><el-input v-model="form.code" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="서비스 유형"><el-input v-model="form.serviceType" placeholder="예: 프론트엔드 서비스 / 백엔드 서비스" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Repository 유형">
              <el-radio-group v-model="form.repoType" @change="handleRepoTypeChange">
                <el-radio-button label="git">Git</el-radio-button>
                <el-radio-button label="svn">SVN</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="Repository 주소" required><el-input v-model="form.repoUrl" :placeholder="isSVNRepository() ? 'https://svn.example.com/svn/team/app' : 'https://git.example.com/team/app.git'" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Repository Credential">
              <el-select v-model="form.repoCredentialId" clearable filterable style="width: 100%" :placeholder="isSVNRepository() ? 'Public SVN은 선택하지 않아도 되며 HTTP(S) 인증을 지원합니다' : 'Public Repository는 선택하지 않아도 됩니다'">
                <el-option v-for="item in credentialOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="isSVNRepository() ? 'SVN Version' : '기본 Branch'">
              <el-input v-model="form.branch" :placeholder="isSVNRepository() ? 'HEAD(최신 Version) 또는 숫자 Revision' : '예: main / master / release/1.0'" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="기본 Environment" required>
              <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" placeholder="Environment를 선택하십시오">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="비즈니스 기능"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="예: 사용자 Login, Order 처리 또는 Game Gateway 등 비즈니스 기능 제공" /></el-form-item>
          </el-col>
          <el-col :span="12">
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
        <el-button type="primary" :loading="saving" @click="submit">저장</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.app-page { padding: 24px; }
.app-header, .filter-panel, .table-card { background: #fff; border: 1px solid #e5edf8; border-radius: 12px; }
.app-header { display: flex; justify-content: space-between; align-items: center; padding: 24px; margin-bottom: 16px; }
.app-header h1 { margin: 0; font-size: 28px; color: #071b3d; }
.app-header p { margin: 8px 0 0; color: #6b7c9b; }
.filter-panel { padding: 18px 24px 0; margin-bottom: 16px; }
:deep(.filter-panel .el-select) { width: 220px; }
:deep(.filter-panel .el-input) { width: 280px; }
.table-card { border-radius: 12px; }
.name-cell { display: flex; flex-direction: column; gap: 4px; }
.name-cell strong { color: #1677ff; }
.name-cell span, .repo-url { color: #697b99; }
.repo-url { margin-left: 8px; }
.pager { display: flex; justify-content: flex-end; padding-top: 16px; }
:deep(.app-project-dialog) { overflow: hidden; border: 1px solid #d8e5f6; border-radius: 16px; box-shadow: 0 20px 48px rgba(20, 55, 105, .18); }
:deep(.app-project-dialog .el-dialog__header) { position: relative; padding: 20px 24px 16px !important; border-bottom: 1px solid #e2eaf6; background: linear-gradient(118deg, #fff, #f3f8ff); }
:deep(.app-project-dialog .el-dialog__header::before) { position: absolute; top: 0; left: 0; width: 100%; height: 3px; content: ''; background: linear-gradient(90deg, #2563eb, #4b86f2 58%, #ea580c 58%, #ea580c 66%, transparent 66%); }
:deep(.app-project-dialog .el-dialog__title) { color: #183962; font-size: 18px; font-weight: 700; }
:deep(.app-project-dialog .el-dialog__body) { max-height: calc(92vh - 150px); overflow: auto; padding: 20px 24px !important; background: #fbfdff; }
:deep(.app-project-dialog .el-form > .el-row) { padding: 18px 18px 4px; border: 1px solid #dfe9f7; border-radius: 12px; background: #fff; }
:deep(.app-project-dialog .el-form-item__label) { color: #526985; font-weight: 600; }
:deep(.app-project-dialog .el-radio-button__inner) { min-width: 54px; border-radius: 7px !important; }
:deep(.app-project-dialog .el-dialog__footer) { padding: 14px 24px 18px !important; border-top: 1px solid #e2eaf6; background: #fff; }
:deep(.app-project-dialog .el-dialog__footer .el-button--primary) { min-width: 88px; }
</style>
