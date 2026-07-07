<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteOpsAppBuildTask,
  queryOpsAppBuildTaskList,
  queryOpsApplicationOptions,
  runOpsAppBuildTask,
  saveOpsAppBuildTask,
  updateOpsAppBuildTaskStatus
} from '../../api/ops'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const rows = ref([])
const total = ref(0)
const appOptions = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  appId: undefined,
  env: '',
  keyword: ''
})

const form = reactive({
  id: undefined,
  name: '',
  appId: undefined,
  env: 'test',
  branch: '',
  buildScript: 'npm install\nnpm run build',
  deployScript: '',
  timeoutSeconds: 1800,
  status: 1,
  description: ''
})

const currentApp = computed(() => appOptions.value.find((item) => Number(item.id) === Number(form.appId)))

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    appId: undefined,
    env: 'test',
    branch: '',
    buildScript: 'npm install\nnpm run build',
    deployScript: '',
    timeoutSeconds: 1800,
    status: 1,
    description: ''
  })
}

async function loadApps() {
  appOptions.value = await queryOpsApplicationOptions()
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsAppBuildTaskList(query)
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

function assignForm(row, copy = false) {
  Object.assign(form, {
    id: copy ? undefined : row.id,
    name: copy ? `${row.name || '构建任务'}-copy` : row.name || '',
    appId: row.appId,
    env: row.env || 'test',
    branch: row.branch || '',
    buildScript: row.buildScript || '',
    deployScript: row.deployScript || '',
    timeoutSeconds: row.timeoutSeconds || 1800,
    status: copy ? 1 : row.status || 1,
    description: row.description || ''
  })
  dialogVisible.value = true
}

function fillFromApp() {
  if (!currentApp.value) return
  if (!form.branch) form.branch = currentApp.value.branch || 'master'
  if (!form.env) form.env = currentApp.value.env || 'test'
}

async function submit() {
  if (!form.name || !form.appId || !form.buildScript) {
    ElMessage.warning('请填写任务名称、所属项目和构建脚本')
    return
  }
  saving.value = true
  try {
    await saveOpsAppBuildTask({ ...form })
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function runBuild(row) {
  await ElMessageBox.confirm(`确认立即构建「${row.name}」？`, '立即构建', { type: 'warning' })
  const data = await runOpsAppBuildTask({ taskId: row.id })
  ElMessage.success(`构建任务已提交：#${data.releaseId}`)
  await loadData()
}

async function toggleStatus(row) {
  const next = Number(row.status) === 1 ? 2 : 1
  await updateOpsAppBuildTaskStatus({ id: row.id, status: next })
  ElMessage.success(next === 1 ? '已启用' : '已禁用')
  await loadData()
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除构建任务「${row.name}」？`, '删除构建任务', { type: 'warning' })
  await deleteOpsAppBuildTask(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function goHistory(row) {
  router.push({ path: '/applications/build-history', query: { appId: row.appId, env: row.env || '', keyword: row.name } })
}

function statusText(status) {
  return Number(status) === 1 ? '正常' : '已禁用'
}

function statusType(status) {
  return Number(status) === 1 ? 'success' : 'danger'
}

function buildStats(row) {
  const totalCount = (row.successCount || 0) + (row.failedCount || 0)
  return `${totalCount} 次 / 成功 ${row.successCount || 0} / 失败 ${row.failedCount || 0}`
}

onMounted(async () => {
  await loadApps()
  await loadData()
})
</script>

<template>
  <div class="app-page">
    <div class="app-header">
      <div>
        <h1>构建任务</h1>
        <p>为项目创建构建与发布任务，任务会使用项目中配置的 Git/SVN 仓库地址拉取代码。</p>
      </div>
      <el-button type="primary" @click="openCreate">+ 新建构建任务</el-button>
    </div>

    <div class="filter-panel">
      <el-form inline>
        <el-form-item label="项目">
          <el-select v-model="query.appId" clearable filterable placeholder="全部项目">
            <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="环境">
          <el-select v-model="query.env" clearable placeholder="全部环境">
            <el-option label="dev" value="dev" />
            <el-option label="test" value="test" />
            <el-option label="prod" value="prod" />
          </el-select>
        </el-form-item>
        <el-form-item label="任务名称">
          <el-input v-model="query.keyword" clearable placeholder="请输入任务名称" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">搜索</el-button>
          <el-button @click="Object.assign(query, { appId: undefined, env: '', keyword: '', pageNum: 1 }); loadData()">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column prop="name" label="任务名称" min-width="170">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.description || '暂无描述' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="appName" label="所属项目" min-width="150" />
        <el-table-column prop="env" label="构建环境" width="110">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.env || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="branch" label="默认分支" width="130" />
        <el-table-column label="任务状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近构建" min-width="150">
          <template #default="{ row }">
            <div class="history-cell">
              <el-link v-if="row.lastReleaseId" type="primary" @click="goHistory(row)">#{{ row.lastReleaseId }}</el-link>
              <span v-else>-</span>
              <el-tag v-if="row.lastStatus" size="small" :type="row.lastStatus === 'success' ? 'success' : row.lastStatus === 'running' ? 'warning' : 'danger'">
                {{ row.lastStatus }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="构建统计" min-width="160">
          <template #default="{ row }">{{ buildStats(row) }}</template>
        </el-table-column>
        <el-table-column label="创建者" width="100">管理员</el-table-column>
        <el-table-column prop="createTime" label="创建时间" min-width="170" />
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="Number(row.status) !== 1" @click="runBuild(row)">立即构建</el-button>
            <el-dropdown trigger="click">
              <el-button link type="primary">更多</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="goHistory(row)">日志</el-dropdown-item>
                  <el-dropdown-item @click="assignForm(row)">编辑</el-dropdown-item>
                  <el-dropdown-item @click="assignForm(row, true)">复制</el-dropdown-item>
                  <el-dropdown-item @click="toggleStatus(row)">{{ Number(row.status) === 1 ? '禁用' : '启用' }}</el-dropdown-item>
                  <el-dropdown-item divided class="danger-item" @click="remove(row)">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
      </div>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="form.id ? '编辑构建任务' : '新建构建任务'"
      width="min(1280px, 92vw)"
      top="4vh"
      class="build-task-dialog"
    >
      <div class="dialog-layout">
        <section class="dialog-section">
          <div class="section-title">
            <strong>基础信息</strong>
            <span>构建任务会继承所选项目的仓库地址，分支可按任务覆盖。</span>
          </div>
          <el-form :model="form" label-width="88px">
            <el-row :gutter="18">
              <el-col :span="8">
                <el-form-item label="任务名称" required>
                  <el-input v-model="form.name" placeholder="例如：test-web-prod" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="所属项目" required>
                  <el-select v-model="form.appId" filterable placeholder="请选择项目" @change="fillFromApp">
                    <el-option v-for="item in appOptions" :key="item.id" :label="`${item.name} (${item.repoType})`" :value="item.id" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="4">
                <el-form-item label="环境">
                  <el-input v-model="form.env" />
                </el-form-item>
              </el-col>
              <el-col :span="4">
                <el-form-item label="状态">
                  <el-switch v-model="form.status" :active-value="1" :inactive-value="2" active-text="启用" inactive-text="禁用" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="构建分支">
                  <el-input v-model="form.branch" placeholder="默认使用项目分支" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="超时时间">
                  <el-input-number v-model="form.timeoutSeconds" :min="60" :max="7200" :step="60" controls-position="right" />
                  <span class="unit">秒</span>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="任务描述">
                  <el-input v-model="form.description" placeholder="描述构建用途或发布范围" />
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
          <div v-if="currentApp" class="repo-preview">
            <div>
              <span>仓库地址</span>
              <strong>{{ currentApp.repoUrl }}</strong>
            </div>
            <el-tag size="small" effect="plain">{{ currentApp.repoType || 'git' }}</el-tag>
          </div>
        </section>

        <section class="script-grid">
          <div class="script-card">
            <div class="script-head">
              <div>
                <strong>构建脚本</strong>
                <span>必填，通常用于依赖安装、编译、镜像构建等步骤。</span>
              </div>
              <el-tag size="small" type="primary" effect="dark">Build</el-tag>
            </div>
            <el-input v-model="form.buildScript" class="script-editor" type="textarea" :rows="18" spellcheck="false" />
          </div>
          <div class="script-card">
            <div class="script-head">
              <div>
                <strong>发布脚本</strong>
                <span>可选，适合 kubectl apply、rsync、重启服务或健康检查。</span>
              </div>
              <el-tag size="small" type="success" effect="dark">Deploy</el-tag>
            </div>
            <el-input v-model="form.deployScript" class="script-editor" type="textarea" :rows="18" spellcheck="false" placeholder="# 可选，例如：\nkubectl apply -f deploy.yaml\nkubectl rollout status deployment/app" />
          </div>
        </section>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
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
.name-cell { display: flex; flex-direction: column; gap: 4px; }
.name-cell strong { color: #071b3d; }
.name-cell span { color: #7d8ba6; }
.history-cell { display: flex; align-items: center; gap: 8px; }
.unit { margin-left: 8px; color: #6b7c9b; }
.pager { display: flex; justify-content: flex-end; padding-top: 16px; }
.dialog-layout { max-height: 72vh; overflow: auto; padding-right: 4px; }
.dialog-section { padding: 18px; border: 1px solid #e5edf8; border-radius: 12px; background: #f8fbff; }
.section-title { display: flex; align-items: baseline; gap: 12px; margin-bottom: 18px; }
.section-title strong { font-size: 16px; color: #071b3d; }
.section-title span { color: #73839f; }
.repo-preview { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 14px; border: 1px solid #d8e5f6; border-radius: 8px; background: #fff; color: #667895; }
.repo-preview div { min-width: 0; display: flex; align-items: center; gap: 10px; }
.repo-preview strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #0b1f42; }
.script-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 18px; margin-top: 18px; }
.script-card { overflow: hidden; border: 1px solid #dce7f6; border-radius: 12px; background: #101827; }
.script-head { display: flex; justify-content: space-between; gap: 12px; padding: 14px 16px; background: #172033; border-bottom: 1px solid #29364f; }
.script-head div { display: flex; flex-direction: column; gap: 4px; }
.script-head strong { color: #f8fafc; font-size: 15px; }
.script-head span { color: #91a1ba; font-size: 12px; }
:deep(.script-editor .el-textarea__inner) {
  min-height: 430px !important;
  resize: vertical;
  border: 0;
  border-radius: 0;
  background: #0f172a;
  color: #e5edf8;
  box-shadow: none;
  font-family: Consolas, Monaco, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.7;
  tab-size: 2;
}
:deep(.script-editor .el-textarea__inner::placeholder) { color: #64748b; }
:deep(.build-task-dialog .el-dialog__body) { padding-top: 8px; }
:deep(.danger-item) { color: #f56c6c; }
@media (max-width: 980px) {
  .script-grid { grid-template-columns: 1fr; }
}
</style>
