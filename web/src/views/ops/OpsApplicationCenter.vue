<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteOpsApplication,
  opsAppReleaseInfo,
  opsApplicationInfo,
  queryOpsAppReleaseList,
  queryOpsApplicationList,
  runOpsAppRelease,
  saveOpsApplication
} from '../../api/ops'

const loading = ref(false)
const releaseLoading = ref(false)
const saving = ref(false)
const appDialogVisible = ref(false)
const releaseDialogVisible = ref(false)
const logDrawerVisible = ref(false)
const rows = ref([])
const releases = ref([])
const total = ref(0)
const releaseTotal = ref(0)
const activeApp = ref(null)
const currentLog = ref({})

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', repoType: '', status: '' })
const releaseQuery = reactive({ pageNum: 1, pageSize: 10, appId: undefined, keyword: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  code: '',
  repoType: 'git',
  repoUrl: '',
  branch: 'master',
  workspace: '',
  env: 'prod',
  buildScript: 'npm install\nnpm run build',
  deployScript: '',
  status: 1,
  description: ''
})
const releaseForm = reactive({ appId: undefined, version: '', branch: '' })

const stat = computed(() => {
  const enabled = rows.value.filter((item) => item.status === 1).length
  const running = rows.value.filter((item) => item.lastStatus === 'running').length
  const failed = rows.value.filter((item) => item.lastStatus === 'failed').length
  const success = rows.value.filter((item) => item.lastStatus === 'success').length
  return { total: total.value, enabled, running, failed, success }
})

const latestRelease = computed(() => releases.value[0] || null)

function statusType(status) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'info'
}

function statusText(status) {
  const map = { success: '成功', failed: '失败', running: '执行中', pending: '等待中' }
  return map[status] || '未发布'
}

function stageState(stageName, release) {
  if (!release) return 'idle'
  const order = ['checkout', 'build', 'deploy', 'done']
  const current = order.indexOf(release.stage || '')
  const target = order.indexOf(stageName)
  if (release.status === 'failed' && current === target) return 'failed'
  if (release.status === 'success' || current > target) return 'success'
  if (current === target) return 'running'
  return 'idle'
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    code: '',
    repoType: 'git',
    repoUrl: '',
    branch: 'master',
    workspace: '',
    env: 'prod',
    buildScript: 'npm install\nnpm run build',
    deployScript: '',
    status: 1,
    description: ''
  })
}

async function loadApps() {
  loading.value = true
  try {
    const data = await queryOpsApplicationList(query)
    rows.value = data.list || []
    total.value = data.total || 0
    if (!activeApp.value && rows.value.length) selectApp(rows.value[0])
  } finally {
    loading.value = false
  }
}

async function loadReleases() {
  releaseLoading.value = true
  try {
    const data = await queryOpsAppReleaseList(releaseQuery)
    releases.value = data.list || []
    releaseTotal.value = data.total || 0
  } finally {
    releaseLoading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, pageSize: 10, keyword: '', repoType: '', status: '' })
  activeApp.value = null
  releaseQuery.appId = undefined
  loadApps()
  loadReleases()
}

function selectApp(row) {
  activeApp.value = row
  releaseQuery.appId = row?.id
  releaseQuery.pageNum = 1
  loadReleases()
}

function openCreate() {
  resetForm()
  appDialogVisible.value = true
}

async function openEdit(row) {
  const data = await opsApplicationInfo(row.id)
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    code: data.code || '',
    repoType: data.repoType || 'git',
    repoUrl: data.repoUrl || '',
    branch: data.branch || '',
    workspace: data.workspace || '',
    env: data.env || '',
    buildScript: data.buildScript || '',
    deployScript: data.deployScript || '',
    status: data.status || 1,
    description: data.description || ''
  })
  appDialogVisible.value = true
}

async function submitApp() {
  saving.value = true
  try {
    await saveOpsApplication({ ...form })
    ElMessage.success('保存成功')
    appDialogVisible.value = false
    await loadApps()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除应用 ${row.name}？`, '删除确认', { type: 'warning' })
  await deleteOpsApplication(row.id)
  if (activeApp.value?.id === row.id) {
    activeApp.value = null
    releaseQuery.appId = undefined
  }
  ElMessage.success('删除成功')
  await loadApps()
  await loadReleases()
}

function openRelease(row) {
  const app = row || activeApp.value
  if (!app) {
    ElMessage.warning('请先选择应用')
    return
  }
  Object.assign(releaseForm, {
    appId: app.id,
    version: new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14),
    branch: app.branch || ''
  })
  releaseDialogVisible.value = true
}

async function submitRelease() {
  saving.value = true
  try {
    const data = await runOpsAppRelease({ ...releaseForm })
    ElMessage.success(`发布任务已创建 #${data.releaseId}`)
    releaseDialogVisible.value = false
    await loadApps()
    await loadReleases()
  } finally {
    saving.value = false
  }
}

async function openLog(row) {
  currentLog.value = await opsAppReleaseInfo(row.id)
  logDrawerVisible.value = true
}

onMounted(async () => {
  await Promise.all([loadApps(), loadReleases()])
})
</script>

<template>
  <div class="ops-app-page">
    <section class="delivery-hero">
      <div>
        <p class="eyebrow">DEVOPS DELIVERY</p>
        <h1>应用中心</h1>
        <p>参考 Jenkins 流水线与蓝鲸作业台风格，统一管理 Git / SVN 应用、构建发布和执行日志。</p>
      </div>
      <div class="hero-actions">
        <el-button @click="loadApps">刷新</el-button>
        <el-button type="primary" @click="openCreate">新增应用</el-button>
        <el-button type="success" :disabled="!activeApp" @click="openRelease()">立即发布</el-button>
      </div>
    </section>

    <section class="summary-row">
      <div class="summary-card"><span>应用总数</span><strong>{{ stat.total }}</strong></div>
      <div class="summary-card"><span>启用应用</span><strong>{{ stat.enabled }}</strong></div>
      <div class="summary-card"><span>发布成功</span><strong>{{ stat.success }}</strong></div>
      <div class="summary-card"><span>执行中</span><strong>{{ stat.running }}</strong></div>
      <div class="summary-card danger"><span>失败应用</span><strong>{{ stat.failed }}</strong></div>
    </section>

    <section class="toolbar">
      <el-input v-model="query.keyword" placeholder="搜索应用名称 / 编码 / 仓库地址" clearable @keyup.enter="loadApps" />
      <el-select v-model="query.repoType" placeholder="仓库类型" clearable>
        <el-option label="Git" value="git" />
        <el-option label="SVN" value="svn" />
      </el-select>
      <el-select v-model="query.status" placeholder="应用状态" clearable>
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadApps">查询</el-button>
      <el-button @click="resetQuery">重置</el-button>
    </section>

    <section class="delivery-layout">
      <aside class="app-sidebar">
        <div class="sidebar-title">
          <strong>应用流水线</strong>
          <span>{{ total }} 个</span>
        </div>
        <el-scrollbar height="620px">
          <button
            v-for="item in rows"
            :key="item.id"
            class="pipeline-item"
            :class="{ active: activeApp?.id === item.id }"
            @click="selectApp(item)"
          >
            <span class="repo-badge">{{ item.repoType?.toUpperCase() || 'GIT' }}</span>
            <span class="pipeline-main">
              <strong>{{ item.name }}</strong>
              <small>{{ item.code }} · {{ item.branch || '-' }}</small>
            </span>
            <el-tag size="small" :type="statusType(item.lastStatus)">{{ statusText(item.lastStatus) }}</el-tag>
          </button>
        </el-scrollbar>
      </aside>

      <main class="pipeline-console">
        <div v-if="activeApp" class="console-head">
          <div>
            <p class="eyebrow">PIPELINE</p>
            <h2>{{ activeApp.name }}</h2>
            <div class="meta-line">
              <el-tag>{{ activeApp.repoType?.toUpperCase() }}</el-tag>
              <span>{{ activeApp.env || 'prod' }}</span>
              <span>{{ activeApp.repoUrl }}</span>
            </div>
          </div>
          <div class="console-actions">
            <el-button @click="openEdit(activeApp)">配置</el-button>
            <el-button type="success" @click="openRelease(activeApp)">构建发布</el-button>
          </div>
        </div>
        <el-empty v-else description="请选择或新增一个应用" />

        <div v-if="activeApp" class="stage-board">
          <div class="stage-node" :class="stageState('checkout', latestRelease)">
            <span class="stage-dot"></span>
            <strong>拉取代码</strong>
            <small>Git / SVN Checkout</small>
          </div>
          <div class="stage-line"></div>
          <div class="stage-node" :class="stageState('build', latestRelease)">
            <span class="stage-dot"></span>
            <strong>构建制品</strong>
            <small>Build Script</small>
          </div>
          <div class="stage-line"></div>
          <div class="stage-node" :class="stageState('deploy', latestRelease)">
            <span class="stage-dot"></span>
            <strong>发布部署</strong>
            <small>Deploy Script</small>
          </div>
          <div class="stage-line"></div>
          <div class="stage-node" :class="stageState('done', latestRelease)">
            <span class="stage-dot"></span>
            <strong>完成</strong>
            <small>Result</small>
          </div>
        </div>

        <div v-if="activeApp" class="detail-grid">
          <div class="info-panel">
            <div class="panel-title">应用配置</div>
            <dl>
              <dt>应用编码</dt><dd>{{ activeApp.code }}</dd>
              <dt>默认分支</dt><dd>{{ activeApp.branch || '-' }}</dd>
              <dt>工作目录</dt><dd>{{ activeApp.workspace || 'uploads/apps/{应用编码}' }}</dd>
              <dt>最近状态</dt><dd><el-tag :type="statusType(activeApp.lastStatus)">{{ statusText(activeApp.lastStatus) }}</el-tag></dd>
            </dl>
          </div>
          <div class="info-panel">
            <div class="panel-title">最近发布</div>
            <dl>
              <dt>版本</dt><dd>{{ latestRelease?.version || '-' }}</dd>
              <dt>阶段</dt><dd>{{ latestRelease?.stage || '-' }}</dd>
              <dt>Commit/Revision</dt><dd>{{ latestRelease?.commitId || '-' }}</dd>
              <dt>摘要</dt><dd>{{ latestRelease?.summary || '-' }}</dd>
            </dl>
          </div>
        </div>

        <el-card v-if="activeApp" shadow="never" class="history-card">
          <template #header>
            <div class="card-header">
              <strong>构建发布历史</strong>
              <div>
                <el-input v-model="releaseQuery.keyword" placeholder="搜索版本 / 摘要" clearable @keyup.enter="loadReleases" />
                <el-button @click="loadReleases">刷新</el-button>
              </div>
            </div>
          </template>
          <el-table v-loading="releaseLoading" :data="releases">
            <el-table-column prop="version" label="版本" min-width="130" />
            <el-table-column prop="branch" label="分支" width="120" show-overflow-tooltip />
            <el-table-column prop="commitId" label="Commit/Revision" width="150" show-overflow-tooltip />
            <el-table-column label="状态" width="110">
              <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
            </el-table-column>
            <el-table-column prop="stage" label="阶段" width="110" />
            <el-table-column prop="summary" label="摘要" min-width="220" show-overflow-tooltip />
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }"><el-button link type="primary" @click="openLog(row)">日志</el-button></template>
            </el-table-column>
          </el-table>
          <el-pagination
            v-model:current-page="releaseQuery.pageNum"
            v-model:page-size="releaseQuery.pageSize"
            class="pager"
            layout="total, sizes, prev, pager, next"
            :total="releaseTotal"
            @current-change="loadReleases"
            @size-change="loadReleases"
          />
        </el-card>
      </main>
    </section>

    <el-drawer v-model="appDialogVisible" :title="form.id ? '编辑应用' : '新增应用'" size="760px">
      <el-form label-width="104px">
        <el-form-item label="应用名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="应用编码" required><el-input v-model="form.code" :disabled="!!form.id" /></el-form-item>
        <el-form-item label="仓库类型" required>
          <el-radio-group v-model="form.repoType">
            <el-radio-button label="git">Git</el-radio-button>
            <el-radio-button label="svn">SVN</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="仓库地址" required><el-input v-model="form.repoUrl" placeholder="https://git.example.com/group/app.git" /></el-form-item>
        <el-form-item label="分支/路径"><el-input v-model="form.branch" placeholder="Git 分支，例如 master / main；SVN 可留空" /></el-form-item>
        <el-form-item label="工作目录"><el-input v-model="form.workspace" placeholder="默认 uploads/apps/{应用编码}" /></el-form-item>
        <el-form-item label="运行环境"><el-input v-model="form.env" placeholder="prod / test / dev" /></el-form-item>
        <el-form-item label="构建脚本" required><el-input v-model="form.buildScript" type="textarea" :rows="7" /></el-form-item>
        <el-form-item label="发布脚本"><el-input v-model="form.deployScript" type="textarea" :rows="7" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :label="1">启用</el-radio>
            <el-radio :label="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="appDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitApp">保存</el-button>
      </template>
    </el-drawer>

    <el-dialog v-model="releaseDialogVisible" title="构建发布" width="520px">
      <el-form label-width="96px">
        <el-form-item label="应用"><strong>{{ rows.find((item) => item.id === releaseForm.appId)?.name || '-' }}</strong></el-form-item>
        <el-form-item label="发布版本" required><el-input v-model="releaseForm.version" /></el-form-item>
        <el-form-item label="分支"><el-input v-model="releaseForm.branch" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="releaseDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitRelease">开始发布</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="logDrawerVisible" title="发布日志" size="72%">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="应用">{{ currentLog.appName }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ currentLog.version }}</el-descriptions-item>
        <el-descriptions-item label="状态"><el-tag :type="statusType(currentLog.status)">{{ statusText(currentLog.status) }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="阶段">{{ currentLog.stage }}</el-descriptions-item>
        <el-descriptions-item label="Commit/Revision">{{ currentLog.commitId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ currentLog.durationMs || 0 }} ms</el-descriptions-item>
      </el-descriptions>
      <h3>构建日志</h3>
      <pre class="log-box">{{ currentLog.buildLog || '暂无构建日志' }}</pre>
      <h3>发布日志</h3>
      <pre class="log-box">{{ currentLog.deployLog || '暂无发布日志' }}</pre>
    </el-drawer>
  </div>
</template>

<style scoped>
.ops-app-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.delivery-hero,
.toolbar,
.summary-card,
.app-sidebar,
.pipeline-console,
.info-panel,
.history-card {
  border: 1px solid #dfe8f6;
  border-radius: 8px;
  background: #fff;
}
.delivery-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 24px;
}
.eyebrow {
  margin: 0 0 8px;
  color: #2f6fed;
  font-size: 12px;
  font-weight: 800;
}
.delivery-hero h1,
.console-head h2 {
  margin: 0 0 8px;
  color: #061735;
}
.delivery-hero p {
  margin: 0;
  color: #60708f;
}
.hero-actions,
.console-actions,
.card-header > div {
  display: flex;
  gap: 10px;
}
.summary-row {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}
.summary-card {
  padding: 16px 18px;
}
.summary-card span {
  display: block;
  color: #70809e;
}
.summary-card strong {
  display: block;
  margin-top: 8px;
  color: #061735;
  font-size: 28px;
}
.summary-card.danger strong {
  color: #f56c6c;
}
.toolbar {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) 160px 150px auto auto;
  gap: 12px;
  padding: 14px;
}
.delivery-layout {
  display: grid;
  grid-template-columns: 360px minmax(0, 1fr);
  gap: 16px;
}
.app-sidebar {
  padding: 16px;
}
.sidebar-title,
.card-header,
.console-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.sidebar-title {
  margin-bottom: 12px;
}
.sidebar-title span {
  color: #71809d;
}
.pipeline-item {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 12px;
  padding: 12px;
  margin-bottom: 10px;
  text-align: left;
  border: 1px solid #e2eaf6;
  border-radius: 8px;
  background: #f8fbff;
  cursor: pointer;
}
.pipeline-item.active {
  border-color: #3a7cff;
  background: #eef5ff;
}
.repo-badge {
  flex: 0 0 auto;
  padding: 5px 8px;
  color: #fff;
  background: #1f4f9a;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 800;
}
.pipeline-main {
  flex: 1;
  min-width: 0;
}
.pipeline-main strong,
.pipeline-main small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pipeline-main small {
  margin-top: 4px;
  color: #6f7f9e;
}
.pipeline-console {
  padding: 18px;
}
.meta-line {
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 900px;
  color: #657591;
}
.meta-line span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stage-board {
  display: grid;
  grid-template-columns: 1fr 48px 1fr 48px 1fr 48px 1fr;
  align-items: center;
  margin: 18px 0;
  padding: 18px;
  background: #f5f8fd;
  border-radius: 8px;
}
.stage-node {
  min-height: 96px;
  padding: 16px;
  border: 1px solid #dbe5f4;
  border-radius: 8px;
  background: #fff;
}
.stage-node strong,
.stage-node small {
  display: block;
}
.stage-node small {
  margin-top: 6px;
  color: #7a89a5;
}
.stage-dot {
  display: inline-block;
  width: 12px;
  height: 12px;
  margin-bottom: 12px;
  border-radius: 50%;
  background: #c7d3e6;
}
.stage-node.success .stage-dot {
  background: #30b566;
}
.stage-node.running .stage-dot {
  background: #e6a23c;
  box-shadow: 0 0 0 5px rgba(230, 162, 60, 0.14);
}
.stage-node.failed .stage-dot {
  background: #f56c6c;
}
.stage-line {
  height: 2px;
  background: #cbd8eb;
}
.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-bottom: 16px;
}
.info-panel {
  padding: 16px;
}
.panel-title {
  margin-bottom: 12px;
  color: #061735;
  font-weight: 800;
}
.info-panel dl {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: 10px 14px;
  margin: 0;
}
.info-panel dt {
  color: #7a89a5;
}
.info-panel dd {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: #061735;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pager {
  justify-content: flex-end;
  margin-top: 14px;
}
.log-box {
  min-height: 180px;
  padding: 14px;
  overflow: auto;
  color: #dbe7ff;
  background: #0d1628;
  border-radius: 8px;
  font-family: Consolas, Monaco, monospace;
  white-space: pre-wrap;
}
@media (max-width: 1200px) {
  .delivery-layout,
  .detail-grid,
  .summary-row {
    grid-template-columns: 1fr;
  }
  .toolbar {
    grid-template-columns: 1fr 1fr;
  }
  .stage-board {
    grid-template-columns: 1fr;
    gap: 10px;
  }
  .stage-line {
    display: none;
  }
}
</style>
