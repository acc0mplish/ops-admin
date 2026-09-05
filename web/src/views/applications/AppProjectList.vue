<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsApplication, queryOpsApplicationList, saveOpsApplication } from '../../api/ops'
import { queryAssetCredentialOptions } from '../../api/asset'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import { apt } from '../../utils/application-i18n'

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
    ElMessage.warning(apt('projectRequired'))
    return
  }
  if (isSVNRepository() && form.branch && !/^(HEAD|\d+)$/i.test(form.branch.trim())) {
    ElMessage.warning(apt('svnVersionInvalid'))
    return
  }
  saving.value = true
  try {
    await saveOpsApplication({ ...form })
    ElMessage.success(apt('saved'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(apt('projectDeleteConfirm', { name: row.name }), apt('projectDeleteTitle'), { type: 'warning' })
  await deleteOpsApplication(row.id)
  ElMessage.success(apt('deleted'))
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
        <h1>{{ apt('projectManagementTitle') }}</h1>
        <p>{{ apt('projectHeroDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">+ {{ apt('newApplication') }}</el-button>
    </div>

    <div class="filter-panel">
      <el-form inline>
        <el-form-item :label="apt('applicationNameLabel')">
          <el-input v-model="query.keyword" clearable :placeholder="apt('projectSearchPlaceholder')" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item :label="apt('serviceTypeLabel')">
          <el-select v-model="query.serviceType" clearable :placeholder="apt('serviceTypePlaceholder')">
            <el-option :label="apt('serviceTypeFrontend')" value="프론트엔드 서비스" />
            <el-option :label="apt('serviceTypeBackend')" value="백엔드 서비스" />
            <el-option :label="apt('serviceTypeMiddleware')" value="미들웨어" />
          </el-select>
        </el-form-item>
        <el-form-item label="Environment">
          <el-select v-model="query.env" clearable :placeholder="apt('allEnvironmentsPlaceholder')" @change="loadData">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">{{ apt('search') }}</el-button>
          <el-button @click="Object.assign(query, { keyword: '', serviceType: '', env: '', pageNum: 1 }); loadData()">{{ apt('reset') }}</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column prop="name" :label="apt('applicationNameLabel')" min-width="170">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.code }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="apt('businessCapability')" min-width="170" show-overflow-tooltip />
        <el-table-column prop="serviceType" :label="apt('serviceTypeLabel')" width="120">
          <template #default="{ row }">{{ row.serviceType || row.env || '-' }}</template>
        </el-table-column>
        <el-table-column :label="apt('defaultEnvironmentLabel')" width="120">
          <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.env) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="repoUrl" :label="apt('repositoryAddressLabel')" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.repoType || 'git' }}</el-tag>
            <span class="repo-url">{{ row.repoUrl }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="apt('status')" width="90">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ Number(row.status) === 1 ? apt('statusHealthy') : apt('deactivate') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="apt('creatorLabel')" width="100">{{ apt('creatorAdmin') }}</el-table-column>
        <el-table-column prop="createTime" :label="apt('createdAtCol')" min-width="170" />
        <el-table-column :label="apt('actions')" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">{{ apt('viewAction') }}</el-button>
            <el-button link type="primary" @click="openEdit(row)">{{ apt('edit') }}</el-button>
            <el-button link type="danger" @click="remove(row)">{{ apt('delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? apt('editApplicationTitle') : apt('newApplication')" width="min(1280px, 94vw)" top="4vh" class="app-project-dialog">
      <el-form :model="form" label-width="96px">
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item :label="apt('applicationNameLabel')" required><el-input v-model="form.name" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Application Code" required><el-input v-model="form.code" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="apt('serviceTypeLabel')"><el-input v-model="form.serviceType" :placeholder="apt('serviceTypeExamplePlaceholder')" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="apt('repositoryTypeLabel')">
              <el-radio-group v-model="form.repoType" @change="handleRepoTypeChange">
                <el-radio-button label="git">Git</el-radio-button>
                <el-radio-button label="svn">SVN</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="apt('repositoryAddressLabel')" required><el-input v-model="form.repoUrl" :placeholder="isSVNRepository() ? 'https://svn.example.com/svn/team/app' : 'https://git.example.com/team/app.git'" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Repository Credential">
              <el-select v-model="form.repoCredentialId" clearable filterable style="width: 100%" :placeholder="isSVNRepository() ? apt('credentialSvnPlaceholder') : apt('credentialGitPlaceholder')">
                <el-option v-for="item in credentialOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="isSVNRepository() ? apt('svnVersionLabel') : apt('defaultBranchCol')">
              <el-input v-model="form.branch" :placeholder="isSVNRepository() ? apt('svnVersionPlaceholder') : apt('branchExamplePlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="apt('defaultEnvironmentLabel')" required>
              <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" :placeholder="apt('environmentSelectPlaceholder')">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="apt('businessCapability')"><el-input v-model="form.description" type="textarea" :rows="3" :placeholder="apt('businessCapabilityPlaceholder')" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="apt('status')">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">{{ apt('activate') }}</el-radio>
                <el-radio :value="2">{{ apt('deactivate') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ apt('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ apt('save') }}</el-button>
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
