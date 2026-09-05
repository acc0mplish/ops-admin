<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetHostList } from '../../api/asset'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import { apt } from '../../utils/application-i18n'
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
const runVisible = ref(false)
const systemVariablesVisible = ref(false)
const currentRunTask = ref(null)
const rows = ref([])
const total = ref(0)
const appOptions = ref([])
const hostOptions = ref([])
const { environmentOptions } = useEnvironmentOptions()
const systemVariables = [
  ['BUILD_NUMBER', apt('sysVarBuildNumber')], ['VERSION', apt('sysVarBuildVersion')], ['COMMIT_ID', apt('sysVarCommitVersion')],
  ['BRANCH', apt('sysVarBranch')], ['PROJECT_NAME', apt('sysVarProjectName')], ['PROJECT_ID', apt('sysVarProjectId')],
  ['PROJECT_REPO', apt('sysVarProjectRepo')], ['TASK_NAME', apt('sysVarTaskName')], ['TASK_ID', apt('sysVarTaskId')],
  ['ENVIRONMENT', apt('sysVarEnvironment')], ['ENVIRONMENT_TYPE', apt('sysVarEnvironmentType')], ['BUILD_PATH', apt('sysVarBuildPath')]
]

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
  buildParams: [],
  runnerType: 'local',
  runnerHostId: undefined,
  executionPath: '',
  timeoutSeconds: 1800,
  status: 1,
  description: ''
})

const currentApp = computed(() => appOptions.value.find((item) => Number(item.id) === Number(form.appId)))

const dockerComposeBuildScript = `set -eu
if docker info >/dev/null 2>&1; then
  docker_cmd() { docker "$@"; }
elif sudo -n docker info >/dev/null 2>&1; then
  docker_cmd() { sudo -n docker "$@"; }
else
  echo "Cannot access Docker. The build user needs Docker socket access or passwordless sudo for docker."
  exit 1
fi
docker_cmd compose version
if [ ! -f deploy/.env ]; then
  umask 077
  MYSQL_PASSWORD="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
  MYSQL_ROOT_PASSWORD="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
  cat > deploy/.env <<EOF
TZ=Asia/Shanghai
MYSQL_DATABASE=ops_admin
MYSQL_USER=ops_admin
MYSQL_PASSWORD=$MYSQL_PASSWORD
MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD
OPS_ADMIN_INITIAL_USERNAME=admin
OPS_ADMIN_INITIAL_PASSWORD=admin@123
EOF
fi
if grep -q '^OPS_ADMIN_INITIAL_PASSWORD=$' deploy/.env; then
  sed -i 's/^OPS_ADMIN_INITIAL_PASSWORD=$/OPS_ADMIN_INITIAL_PASSWORD=admin@123/' deploy/.env
  echo "Filled an empty OPS_ADMIN_INITIAL_PASSWORD in deploy/.env."
elif ! grep -q '^OPS_ADMIN_INITIAL_PASSWORD=.' deploy/.env; then
  printf '\nOPS_ADMIN_INITIAL_PASSWORD=admin@123\n' >> deploy/.env
  echo "Added OPS_ADMIN_INITIAL_PASSWORD to deploy/.env."
fi
if grep -q '^OPS_ADMIN_INITIAL_USERNAME=$' deploy/.env; then
  sed -i 's/^OPS_ADMIN_INITIAL_USERNAME=$/OPS_ADMIN_INITIAL_USERNAME=admin/' deploy/.env
elif ! grep -q '^OPS_ADMIN_INITIAL_USERNAME=.' deploy/.env; then
  printf '\nOPS_ADMIN_INITIAL_USERNAME=admin\n' >> deploy/.env
fi
if [ ! -f deploy/config.yaml ]; then cp deploy/config.yaml.example deploy/config.yaml; fi
MYSQL_PASSWORD="$(sed -n 's/^MYSQL_PASSWORD=//p' deploy/.env | head -n 1)"
test -n "$MYSQL_PASSWORD" || { echo "MYSQL_PASSWORD is missing"; exit 1; }
OPS_ADMIN_INITIAL_PASSWORD="$(sed -n 's/^OPS_ADMIN_INITIAL_PASSWORD=//p' deploy/.env | head -n 1)"
test -n "$OPS_ADMIN_INITIAL_PASSWORD" || { echo "OPS_ADMIN_INITIAL_PASSWORD is missing"; exit 1; }
sed -i "s|^  password: .*|  password: $MYSQL_PASSWORD|" deploy/config.yaml
chmod 600 deploy/.env deploy/config.yaml
docker_cmd compose --env-file deploy/.env build`

const dockerComposeDeployScript = `set -eu
if docker info >/dev/null 2>&1; then
  docker_cmd() { docker "$@"; }
elif sudo -n docker info >/dev/null 2>&1; then
  docker_cmd() { sudo -n docker "$@"; }
else
  echo "Cannot access Docker. The build user needs Docker socket access or passwordless sudo for docker."
  exit 1
fi
test -f deploy/.env || { echo "deploy/.env was not created"; exit 1; }
test -f deploy/config.yaml || { echo "deploy/config.yaml was not created"; exit 1; }
docker_cmd compose --env-file deploy/.env up -d --remove-orphans
docker_cmd compose --env-file deploy/.env ps`

function applyDockerComposePreset() {
  form.buildScript = dockerComposeBuildScript
  form.deployScript = dockerComposeDeployScript
  form.timeoutSeconds = Math.max(Number(form.timeoutSeconds) || 0, 1800)
  ElMessage.success(apt('composePresetApplied'))
}

function formatDateTime(value) {
  const raw = String(value || '').trim()
  if (!raw) return '-'
  const match = raw.match(/^(\d{4}-\d{2}-\d{2})[T\s](\d{2}:\d{2}:\d{2})/)
  if (match) return `${match[1]} ${match[2]}`
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  const pad = (number) => String(number).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    appId: undefined,
    env: 'test',
    branch: '',
    buildScript: 'npm install\nnpm run build',
    deployScript: '',
    buildParams: [],
    runnerType: 'local',
    runnerHostId: undefined,
    executionPath: '',
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
    name: copy ? `${row.name || 'Build Task'}-copy` : row.name || '',
    appId: row.appId,
    env: row.env || 'test',
    branch: row.branch || '',
    buildScript: row.buildScript || '',
    deployScript: row.deployScript || '',
    buildParams: parseBuildParams(row.buildParamsJson).map((item) => ({ ...item, optionsText: (item.options || []).join('\n') })),
    runnerType: row.runnerType || 'local',
    runnerHostId: row.runnerHostId || undefined,
    executionPath: row.executionPath || row.workspace || '',
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
  if (!form.executionPath) form.executionPath = currentApp.value.workspace || `uploads/apps/${currentApp.value.code}`
}

async function submit() {
  if (!form.name || !form.appId || !form.buildScript) {
    ElMessage.warning(apt('taskRequired'))
    return
  }
  if (form.runnerType === 'host' && !form.runnerHostId) {
    ElMessage.warning(apt('buildHostRequired'))
    return
  }
  if (!form.executionPath) {
    ElMessage.warning(apt('executionPathRequired'))
    return
  }
  saving.value = true
  try {
    const buildParams = form.buildParams.map(({ optionsText, ...item }) => ({
      ...item,
      name: String(item.name || '').trim().toUpperCase(),
      options: String(optionsText || '').split(/[\n,]/).map((value) => value.trim()).filter(Boolean)
    }))
    await saveOpsAppBuildTask({ ...form, buildParams })
    ElMessage.success(apt('saved'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

function parseBuildParams(raw) {
  if (!raw) return []
  try { return JSON.parse(raw) || [] } catch { return [] }
}

function addBuildParam() {
  form.buildParams.push({ name: '', label: '', type: 'text', default: '', options: [], optionsText: '', required: false, description: '' })
}

function removeBuildParam(index) {
  form.buildParams.splice(index, 1)
}

function openRun(row) {
  const definitions = parseBuildParams(row.buildParamsJson)
  const params = {}
  definitions.forEach((item) => {
    if (item.type === 'multiSelect') {
      params[item.name] = Array.isArray(item.default) ? item.default : String(item.default || '').split(',').map((value) => value.trim()).filter(Boolean)
    } else if (item.type === 'boolean') {
      params[item.name] = item.default === true || item.default === 'true'
    } else {
      params[item.name] = item.default ?? ''
    }
  })
  currentRunTask.value = { ...row, buildParams: definitions }
  Object.assign(runForm, { version: '', branch: row.branch || '', params })
  runVisible.value = true
}

const runForm = reactive({ version: '', branch: '', params: {} })

async function submitRun() {
  const task = currentRunTask.value
  if (!task) return
  const missing = task.buildParams.find((item) => item.required && (runForm.params[item.name] === '' || runForm.params[item.name] === undefined || runForm.params[item.name] === null || (Array.isArray(runForm.params[item.name]) && !runForm.params[item.name].length)))
  if (missing) {
    ElMessage.warning(apt('buildParamRequired', { name: missing.label || missing.name }))
    return
  }
  const data = await runOpsAppBuildTask({ taskId: task.id, version: runForm.version, branch: runForm.branch, params: runForm.params })
  ElMessage.success(apt('buildTaskSubmitted', { releaseId: data.releaseId }))
  runVisible.value = false
  await loadData()
}

async function toggleStatus(row) {
  const next = Number(row.status) === 1 ? 2 : 1
  await updateOpsAppBuildTaskStatus({ id: row.id, status: next })
  ElMessage.success(next === 1 ? apt('enabledMessage') : apt('disabledMessage'))
  await loadData()
}

async function remove(row) {
  await ElMessageBox.confirm(apt('buildTaskDeleteConfirm', { name: row.name }), apt('buildTaskDeleteTitle'), { type: 'warning' })
  await deleteOpsAppBuildTask(row.id)
  ElMessage.success(apt('deleted'))
  await loadData()
}

function goHistory(row) {
  router.push({ path: '/applications/build-history', query: { appId: row.appId, env: row.env || '', keyword: row.name } })
}

function statusText(status) {
  return Number(status) === 1 ? apt('statusHealthy') : apt('disabledMessage')
}

function statusType(status) {
  return Number(status) === 1 ? 'success' : 'danger'
}

function buildStats(row) {
  const totalCount = (row.successCount || 0) + (row.failedCount || 0)
  return apt('buildStatsText', { total: totalCount, success: row.successCount || 0, failed: row.failedCount || 0 })
}

function runnerName(row) {
  if (row.runnerType !== 'host') return apt('runnerLocal')
  const host = hostOptions.value.find((item) => Number(item.id) === Number(row.runnerHostId))
  if (!host) return row.runnerHostId ? `Asset Host #${row.runnerHostId}` : apt('buildHostNotSelected')
  const name = host.hostName || `Asset Host #${host.id}`
  const address = host.sshIp || host.privateIp || host.publicIp || '-'
  return `${name} (${address})`
}

onMounted(async () => {
  const hosts = await queryAssetHostList({ pageNum: 1, pageSize: 1000 })
  hostOptions.value = hosts.list || []
  await loadApps()
  await loadData()
})
</script>

<template>
  <div class="app-page">
    <div class="app-header">
      <div>
        <h1>Build Task</h1>
        <p>{{ apt('buildTaskHeroDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">+ {{ apt('newBuildTask') }}</el-button>
    </div>

    <div class="filter-panel">
      <el-form inline>
        <el-form-item label="Application">
          <el-select v-model="query.appId" clearable filterable :placeholder="apt('allApplicationsPlaceholder')">
            <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Environment">
          <el-select v-model="query.env" clearable :placeholder="apt('allEnvironmentsPlaceholder')">
            <el-option label="dev" value="dev" />
            <el-option label="test" value="test" />
            <el-option label="prod" value="prod" />
          </el-select>
        </el-form-item>
        <el-form-item :label="apt('taskNameLabel')">
          <el-input v-model="query.keyword" clearable :placeholder="apt('taskNameSearchPlaceholder')" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">{{ apt('search') }}</el-button>
          <el-button @click="Object.assign(query, { appId: undefined, env: '', keyword: '', pageNum: 1 }); loadData()">{{ apt('reset') }}</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column prop="name" :label="apt('taskNameLabel')" min-width="150">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.description || apt('noDescription') }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="appName" :label="apt('belongingApplication')" min-width="150" />
        <el-table-column prop="env" label="Build Environment" width="110">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.env || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="branch" :label="apt('defaultBranchCol')" width="130" />
        <el-table-column :label="apt('executionNode')" min-width="150">
          <template #default="{ row }">{{ runnerName(row) }}</template>
        </el-table-column>
        <el-table-column :label="apt('executionPathLabel')" min-width="190" show-overflow-tooltip>
          <template #default="{ row }"><code class="path-code">{{ row.executionPath || '-' }}</code></template>
        </el-table-column>
        <el-table-column :label="apt('taskStatusLabel')" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="apt('lastBuild')" min-width="150">
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
        <el-table-column :label="apt('buildStatsLabel')" min-width="160">
          <template #default="{ row }">{{ buildStats(row) }}</template>
        </el-table-column>
        <el-table-column :label="apt('creatorLabel')" width="96" fixed="right">{{ apt('creatorAdmin') }}</el-table-column>
        <el-table-column :label="apt('createdAtCol')" width="150" fixed="right">
          <template #default="{ row }">{{ formatDateTime(row.createTime) }}</template>
        </el-table-column>
        <el-table-column :label="apt('actions')" width="152" fixed="right">
          <template #default="{ row }">
            <div class="action-tools">
              <el-button link type="primary" :disabled="Number(row.status) !== 1" @click="openRun(row)">{{ apt('buildNow') }}</el-button>
              <el-dropdown trigger="click">
                <el-button link class="more-action">{{ apt('moreActions') }}</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item @click="goHistory(row)">Log</el-dropdown-item>
                    <el-dropdown-item @click="assignForm(row)">{{ apt('edit') }}</el-dropdown-item>
                    <el-dropdown-item @click="assignForm(row, true)">{{ apt('duplicate') }}</el-dropdown-item>
                    <el-dropdown-item @click="toggleStatus(row)">{{ Number(row.status) === 1 ? apt('deactivate') : apt('activate') }}</el-dropdown-item>
                    <el-dropdown-item divided class="danger-item" @click="remove(row)">{{ apt('delete') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
      </div>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="form.id ? apt('editBuildTaskTitle') : apt('newBuildTaskTitle')"
      width="min(1180px, 94vw)"
      top="3vh"
      class="build-task-dialog"
    >
      <div class="dialog-layout">
        <section class="dialog-section">
          <div class="section-title">
            <strong>{{ apt('basicInfo') }}</strong>
            <span>{{ apt('basicInfoDesc') }}</span>
          </div>
          <el-form :model="form" label-width="88px">
            <el-row :gutter="18">
              <el-col :span="7">
                <el-form-item :label="apt('taskNameLabel')" required>
                  <el-input v-model="form.name" :placeholder="apt('taskNamePlaceholder')" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item :label="apt('belongingApplication')" required>
                  <el-select v-model="form.appId" filterable :placeholder="apt('applicationSelectPlaceholder')" @change="fillFromApp">
                    <el-option v-for="item in appOptions" :key="item.id" :label="`${item.name} (${item.repoType})`" :value="item.id" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="5">
                <el-form-item label="Environment">
                  <el-select v-model="form.env"><el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" /></el-select>
                </el-form-item>
              </el-col>
              <el-col :span="4">
                <el-form-item :label="apt('status')">
                  <div class="status-control">
                    <el-switch v-model="form.status" :active-value="1" :inactive-value="2" />
                    <span>{{ Number(form.status) === 1 ? apt('enabledMessage') : apt('disabledMessage') }}</span>
                  </div>
                </el-form-item>
              </el-col>
              <el-col :span="7">
                <el-form-item label="Build Branch">
                  <el-input v-model="form.branch" :placeholder="apt('defaultBranchPlaceholder')" />
                </el-form-item>
              </el-col>
              <el-col :span="5">
                <el-form-item label="Timeout">
                  <el-input-number v-model="form.timeoutSeconds" :min="60" :max="7200" :step="60" controls-position="right" />
                  <span class="unit">{{ apt('secondsUnit') }}</span>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item :label="apt('taskDescriptionLabel')">
                  <el-input v-model="form.description" :placeholder="apt('taskDescriptionPlaceholder')" />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item :label="apt('executionNode')">
                  <el-select v-model="form.runnerType"><el-option :label="apt('runnerLocal')" value="local" /><el-option :label="apt('assetHostOption')" value="host" /></el-select>
                </el-form-item>
              </el-col>
              <el-col v-if="form.runnerType === 'host'" :span="8">
                <el-form-item label="Build Host" required>
                  <el-select v-model="form.runnerHostId" filterable :placeholder="apt('buildHostRequired')"><el-option v-for="item in hostOptions" :key="item.id" :label="`${item.hostName} (${item.sshIp || item.privateIp || '-'})`" :value="item.id" /></el-select>
                </el-form-item>
              </el-col>
              <el-col :span="form.runnerType === 'host' ? 10 : 18">
                <el-form-item :label="apt('executionPathLabel')" required class="execution-path-item">
                  <el-input v-model="form.executionPath" :placeholder="apt('executionPathPlaceholder')" />
                  <div class="field-help">{{ apt('executionPathHelp') }}</div>
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
          <div v-if="currentApp" class="repo-preview">
            <div>
              <span>{{ apt('repositoryAddressLabel') }}</span>
              <strong>{{ currentApp.repoUrl }}</strong>
            </div>
            <el-tag size="small" effect="plain">{{ currentApp.repoType || 'git' }}</el-tag>
          </div>
        </section>

        <section class="dialog-section params-section">
          <div class="section-title params-title">
            <div><strong>{{ apt('buildParamSectionTitle') }}</strong><span>{{ apt('buildParamSectionDesc') }}</span></div>
            <div class="section-actions">
              <el-button @click="systemVariablesVisible = true">{{ apt('systemVariablesButton') }}</el-button>
              <el-button type="primary" @click="addBuildParam">{{ apt('newParameter') }}</el-button>
            </div>
          </div>
          <div v-if="!form.buildParams.length" class="empty-params">{{ apt('emptyParamsNotice') }}</div>
          <div v-if="form.buildParams.length" class="param-header" aria-hidden="true">
            <span>{{ apt('variableNameCol') }}</span><span>{{ apt('displayNameCol') }}</span><span>{{ apt('typeCol') }}</span><span>{{ apt('defaultValueCol') }}</span><span>{{ apt('optionsDescCol') }}</span><span>{{ apt('requiredCol') }}</span><span>{{ apt('actions') }}</span>
          </div>
          <div v-for="(item, index) in form.buildParams" :key="index" class="param-row">
            <el-input v-model="item.name" :placeholder="apt('variableNamePlaceholder')" @input="item.name = item.name.toUpperCase()" />
            <el-input v-model="item.label" :placeholder="apt('displayNameCol')" />
            <el-select v-model="item.type" :placeholder="apt('paramTypePlaceholder')">
              <el-option :label="apt('paramTypeText')" value="text" /><el-option :label="apt('paramTypeSelect')" value="select" />
              <el-option :label="apt('paramTypeMultiSelect')" value="multiSelect" /><el-option :label="apt('paramTypeBoolean')" value="boolean" />
            </el-select>
            <el-input v-if="item.type !== 'boolean' && item.type !== 'multiSelect'" v-model="item.default" :placeholder="apt('defaultValueCol')" />
            <el-switch v-else-if="item.type === 'boolean'" v-model="item.default" />
            <el-input v-else v-model="item.default" :placeholder="apt('defaultValueJoinPlaceholder')" />
            <el-input v-if="['select', 'multiSelect'].includes(item.type)" v-model="item.optionsText" type="textarea" :rows="2" :placeholder="apt('optionsTextPlaceholder')" />
            <el-input v-else v-model="item.description" :placeholder="apt('paramDescriptionPlaceholder')" />
            <el-checkbox v-model="item.required">{{ apt('requiredCol') }}</el-checkbox>
            <el-button link type="danger" @click="removeBuildParam(index)">{{ apt('delete') }}</el-button>
          </div>
        </section>

        <section class="script-grid">
          <div class="docker-compose-tip">
            <div>
              <strong>Docker Compose Deploy Template</strong>
              <span>{{ apt('dockerComposeHint') }}</span>
            </div>
            <el-button type="primary" plain @click="applyDockerComposePreset">{{ apt('applyTemplate') }}</el-button>
          </div>
          <div class="script-card">
            <div class="script-head">
              <div>
                <strong>Build Script</strong>
                <span>{{ apt('buildScriptHint') }}</span>
              </div>
              <el-tag size="small" type="primary" effect="dark">Build</el-tag>
            </div>
            <el-input v-model="form.buildScript" class="script-editor" type="textarea" :rows="18" spellcheck="false" />
          </div>
          <div class="script-card">
            <div class="script-head">
              <div><strong>{{ apt('postBuildTitle') }}</strong><span>{{ apt('postBuildDesc') }}</span></div>
              <el-tag size="small" type="success" effect="dark">Post Build</el-tag>
            </div>
            <el-input v-model="form.deployScript" class="script-editor" type="textarea" :rows="18" spellcheck="false" :placeholder="apt('deployScriptPlaceholder')" />
          </div>
        </section>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ apt('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ apt('save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="runVisible" :title="apt('runBuildTitle', { name: currentRunTask?.name || '' })" width="720px">
      <el-alert type="info" :closable="false" show-icon :title="apt('runParamsAlert')" />
      <el-form class="run-form" label-width="120px">
        <el-form-item label="Build Branch"><el-input v-model="runForm.branch" :placeholder="apt('runBranchPlaceholder')" /></el-form-item>
        <el-form-item label="Build Version"><el-input v-model="runForm.version" :placeholder="apt('versionPlaceholder')" /></el-form-item>
        <el-form-item v-for="item in currentRunTask?.buildParams || []" :key="item.name" :label="item.label || item.name" :required="item.required">
          <el-select v-if="item.type === 'select'" v-model="runForm.params[item.name]" clearable style="width: 100%"><el-option v-for="option in item.options" :key="option" :label="option" :value="option" /></el-select>
          <el-select v-else-if="item.type === 'multiSelect'" v-model="runForm.params[item.name]" multiple clearable collapse-tags style="width: 100%"><el-option v-for="option in item.options" :key="option" :label="option" :value="option" /></el-select>
          <el-switch v-else-if="item.type === 'boolean'" v-model="runForm.params[item.name]" />
          <el-input v-else v-model="runForm.params[item.name]" :placeholder="item.description || apt('paramInputPlaceholder', { name: item.label || item.name })" />
          <div v-if="item.description" class="param-help">{{ item.description }} · Environment Variable: ${{ item.name }}</div>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="runVisible = false">{{ apt('cancel') }}</el-button><el-button type="primary" @click="submitRun">{{ apt('startBuild') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="systemVariablesVisible" :title="apt('systemEnvVariablesTitle')" width="760px">
      <el-alert type="info" :closable="false" show-icon :title="apt('systemEnvAlert')" />
      <el-table :data="systemVariables.map(([name, description]) => ({ name, description }))" class="variable-table">
        <el-table-column prop="name" :label="apt('variableNameCol')" width="220"><template #default="{ row }"><code>{{ row.name }}</code></template></el-table-column>
        <el-table-column prop="description" :label="apt('description')" />
      </el-table>
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
.path-code { color: #29466f; font-family: Consolas, Monaco, "Courier New", monospace; font-size: 12px; }
.history-cell { display: flex; align-items: center; gap: 8px; }
.action-tools { display: flex; align-items: center; gap: 14px; white-space: nowrap; }
.more-action { color: #6b7c9b; }
.more-action:hover, .more-action:focus-visible { color: #355b93; }
.unit { margin-left: 8px; color: #6b7c9b; }
.docker-compose-tip { grid-column: 1 / -1; display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 16px; border: 1px solid #bfdbfe; border-radius: 10px; background: #f0f7ff; color: #315b89; }
.docker-compose-tip strong { display: block; margin-bottom: 4px; color: #173d69; }
.docker-compose-tip span { font-size: 13px; }
.pager { display: flex; justify-content: flex-end; padding-top: 16px; }
.dialog-layout { max-height: 78vh; overflow: auto; padding: 2px 6px 2px 2px; }
.dialog-section { padding: 18px 20px; border: 1px solid #dce7f5; border-radius: 8px; background: #fff; }
.params-section { margin-top: 18px; }
.section-title { display: flex; align-items: baseline; gap: 12px; margin-bottom: 18px; }
.params-title { justify-content: space-between; align-items: center; }
.params-title > div:first-child { display: flex; flex-direction: column; gap: 5px; }
.section-actions { display: flex; gap: 8px; }
.section-title strong { font-size: 16px; color: #071b3d; }
.section-title span { color: #73839f; }
.empty-params { padding: 13px 14px; border: 1px dashed #cad8eb; border-radius: 6px; background: #f8fbff; color: #7d8ca5; font-size: 13px; }
.status-control { display: flex; align-items: center; gap: 8px; white-space: nowrap; color: #536784; }
.field-help { width: 100%; margin-top: 5px; color: #8492a8; font-size: 12px; line-height: 1.4; }
:deep(.execution-path-item .el-form-item__content) { align-items: flex-start; }
.repo-preview { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 14px; border: 1px solid #d8e5f6; border-radius: 8px; background: #fff; color: #667895; }
.repo-preview div { min-width: 0; display: flex; align-items: center; gap: 10px; }
.repo-preview strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #0b1f42; }
.script-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 18px; margin-top: 18px; }
.script-grid.single { grid-template-columns: 1fr; }
.param-header, .param-row { display: grid; grid-template-columns: 1.25fr 1.1fr .9fr 1fr 1.4fr 52px 44px; gap: 10px; align-items: center; }
.param-header { padding: 9px 10px; border-radius: 6px; background: #f2f6fc; color: #667895; font-size: 12px; }
.param-row { padding: 12px 10px; border-bottom: 1px solid #e8eef7; }
.param-row:last-child { border-bottom: 0; }
.script-card { overflow: hidden; border: 1px solid #dce7f6; border-radius: 8px; background: #101827; }
.script-head { display: flex; justify-content: space-between; gap: 12px; padding: 14px 16px; background: #172033; border-bottom: 1px solid #29364f; }
.script-head div { display: flex; flex-direction: column; gap: 4px; }
.script-head strong { color: #f8fafc; font-size: 15px; }
.script-head span { color: #91a1ba; font-size: 12px; }
:deep(.script-editor .el-textarea__inner) {
  min-height: 300px !important;
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
.run-form { margin-top: 20px; }
.param-help { margin-top: 4px; color: #7b8ba6; font-size: 12px; }
.variable-table { margin-top: 14px; }
.variable-table code { padding: 3px 7px; border-radius: 4px; background: #eef4ff; color: #2459a9; font-family: Consolas, Monaco, monospace; }
@media (max-width: 1100px) {
  .script-grid { grid-template-columns: 1fr; }
  .param-header { display: none; }
  .param-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
:deep(.script-editor .el-textarea__inner::placeholder) { color: #64748b; }
:deep(.build-task-dialog .el-dialog__body) { padding: 8px 18px 14px; }
:deep(.build-task-dialog .el-dialog__footer) { padding: 12px 18px 16px; border-top: 1px solid #e8eef7; }
:deep(.danger-item) { color: #f56c6c; }
@media (max-width: 980px) {
  .script-grid { grid-template-columns: 1fr; }
}
</style>
