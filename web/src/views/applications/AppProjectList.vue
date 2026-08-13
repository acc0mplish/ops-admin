<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsApplication, queryOpsApplicationBindings, queryOpsApplicationList, saveOpsApplication } from '../../api/ops'
import { queryAssetCredentialOptions, queryAssetDatabaseList, queryAssetGatewayOptions, queryAssetHostGroupList } from '../../api/asset'
import { queryK8sClusterList } from '../../api/k8s'
import { queryMonitorDatasourceOptions } from '../../api/monitor'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const rows = ref([])
const total = ref(0)
const { environmentOptions, environmentLoading, environmentName } = useEnvironmentOptions()
const credentialOptions = ref([])
const hostGroupOptions = ref([])
const databaseOptions = ref([])
const gatewayOptions = ref([])
const clusterOptions = ref([])
const datasourceOptions = ref([])

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
  serviceType: '后端服务',
  repoType: 'git',
  repoUrl: '',
  repoCredentialId: undefined,
  branch: 'master',
  workspace: '',
  env: 'test',
  status: 1,
  description: '',
  bindings: []
})

function flattenGroups(nodes = [], prefix = '') {
  return nodes.flatMap((item) => {
    const label = prefix ? `${prefix} / ${item.name}` : item.name
    return [{ id: item.id, name: label }, ...flattenGroups(item.children || [], label)]
  })
}

function emptyBinding(env = '') {
  return { env, hostGroupId: undefined, k8sClusterId: undefined, namespace: '', workloadType: 'deployment', workloadName: '', databaseId: undefined, monitorDatasourceId: undefined, gatewayId: undefined }
}

function addBinding() { form.bindings.push(emptyBinding()) }
function removeBinding(index) { form.bindings.splice(index, 1) }

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    code: '',
    serviceType: '后端服务',
    repoType: 'git',
    repoUrl: '',
    repoCredentialId: undefined,
    branch: 'master',
    workspace: '',
    env: 'test',
    status: 1,
    description: '',
    bindings: [emptyBinding('test')]
  })
}

async function loadResourceOptions() {
  const [credentials, groups, databases, gateways, clusters, datasources] = await Promise.all([
    queryAssetCredentialOptions(), queryAssetHostGroupList(), queryAssetDatabaseList({ pageNum: 1, pageSize: 1000 }),
    queryAssetGatewayOptions(), queryK8sClusterList(), queryMonitorDatasourceOptions()
  ])
  credentialOptions.value = credentials || []
  hostGroupOptions.value = flattenGroups(groups.tree || groups || [])
  databaseOptions.value = databases.list || []
  gatewayOptions.value = gateways || []
  clusterOptions.value = clusters || []
  datasourceOptions.value = datasources || []
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
  const bindings = await queryOpsApplicationBindings(row.id)
  Object.assign(form, {
    id: row.id,
    name: row.name || '',
    code: row.code || '',
    serviceType: row.serviceType || row.env || '后端服务',
    repoType: row.repoType || 'git',
    repoUrl: row.repoUrl || '',
    repoCredentialId: row.repoCredentialId || undefined,
    branch: row.branch || 'master',
    workspace: row.workspace || '',
    env: row.env || 'test',
    status: row.status || 1,
    description: row.description || '',
    bindings: (bindings || []).map((item) => ({ ...emptyBinding(), ...item }))
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name || !form.code || !form.repoUrl) {
    ElMessage.warning('请填写项目名称、项目编码和仓库地址')
    return
  }
  saving.value = true
  try {
    await saveOpsApplication({ ...form })
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除项目「${row.name}」？`, '删除项目', { type: 'warning' })
  await deleteOpsApplication(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function statusType(status) {
  return Number(status) === 1 ? 'success' : 'info'
}

onMounted(async () => { await Promise.all([loadResourceOptions(), loadData()]) })
</script>

<template>
  <div class="app-page">
    <div class="app-header">
      <div>
        <h1>应用管理</h1>
        <p>统一维护应用代码仓库，并按环境绑定主机、K8s、数据库、网关和监控资源。</p>
      </div>
      <el-button type="primary" @click="openCreate">+ 新建应用</el-button>
    </div>

    <div class="filter-panel">
      <el-form inline>
        <el-form-item label="应用名称">
          <el-input v-model="query.keyword" clearable placeholder="请输入应用名称 / 仓库地址" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="服务类别">
          <el-select v-model="query.serviceType" clearable placeholder="请选择服务类别">
            <el-option label="前端服务" value="前端服务" />
            <el-option label="后端服务" value="后端服务" />
            <el-option label="中间件" value="中间件" />
          </el-select>
        </el-form-item>
        <el-form-item label="环境">
          <el-select v-model="query.env" clearable placeholder="全部环境" @change="loadData">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">搜索</el-button>
          <el-button @click="Object.assign(query, { keyword: '', serviceType: '', env: '', pageNum: 1 }); loadData()">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column prop="name" label="应用名称" min-width="170">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.code }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="业务功能" min-width="170" show-overflow-tooltip />
        <el-table-column prop="serviceType" label="服务类别" width="120">
          <template #default="{ row }">{{ row.serviceType || row.env || '-' }}</template>
        </el-table-column>
        <el-table-column label="默认环境" width="120">
          <template #default="{ row }"><el-tag effect="plain">{{ environmentName(row.env) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="repoUrl" label="仓库地址" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.repoType || 'git' }}</el-tag>
            <span class="repo-url">{{ row.repoUrl }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ Number(row.status) === 1 ? '正常' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建者" width="100">管理员</el-table-column>
        <el-table-column prop="createTime" label="创建时间" min-width="170" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">查看</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑应用' : '新建应用'" width="min(1280px, 94vw)" top="4vh" class="app-project-dialog">
      <el-form :model="form" label-width="96px">
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="应用名称" required><el-input v-model="form.name" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="应用编码" required><el-input v-model="form.code" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="服务类别"><el-input v-model="form.serviceType" placeholder="例如：前端服务 / 后端服务" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="仓库类型">
              <el-radio-group v-model="form.repoType">
                <el-radio-button label="git">Git</el-radio-button>
                <el-radio-button label="svn">SVN</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="仓库地址" required><el-input v-model="form.repoUrl" placeholder="https://git.example.com/team/app.git" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="仓库凭据">
              <el-select v-model="form.repoCredentialId" clearable filterable style="width: 100%" placeholder="公开仓库可不选择">
                <el-option v-for="item in credentialOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="默认分支"><el-input v-model="form.branch" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="默认环境" required>
              <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" placeholder="请选择环境">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="工作目录"><el-input v-model="form.workspace" placeholder="可选，默认 uploads/apps/项目编码" /></el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="业务功能"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="例如：提供用户登录、订单处理或游戏网关等业务能力；将展示在控制台业务拓扑图中" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <div class="binding-header">
          <div><strong>环境资源绑定</strong><span>同一个应用可以在多个环境中关联不同基础设施。</span></div>
          <el-button @click="addBinding">新增环境</el-button>
        </div>
        <div v-for="(binding, index) in form.bindings" :key="index" class="binding-row">
          <el-select v-model="binding.env" placeholder="环境">
            <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
          <el-select v-model="binding.hostGroupId" clearable filterable placeholder="主机组"><el-option v-for="item in hostGroupOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select>
          <el-select v-model="binding.k8sClusterId" clearable filterable placeholder="K8s 集群"><el-option v-for="item in clusterOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select>
          <el-input v-model="binding.namespace" placeholder="命名空间" />
          <el-input v-model="binding.workloadName" placeholder="工作负载" />
          <el-select v-model="binding.databaseId" clearable filterable placeholder="数据库"><el-option v-for="item in databaseOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select>
          <el-select v-model="binding.monitorDatasourceId" clearable filterable placeholder="监控数据源"><el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select>
          <el-select v-model="binding.gatewayId" clearable filterable placeholder="网关"><el-option v-for="item in gatewayOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select>
          <el-button link type="danger" @click="removeBinding(index)">删除</el-button>
        </div>
      </el-form>
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
.table-card { border-radius: 12px; }
.name-cell { display: flex; flex-direction: column; gap: 4px; }
.name-cell strong { color: #1677ff; }
.name-cell span, .repo-url { color: #697b99; }
.repo-url { margin-left: 8px; }
.pager { display: flex; justify-content: flex-end; padding-top: 16px; }
.binding-header { display: flex; justify-content: space-between; align-items: center; margin: 8px 0 12px; padding-top: 16px; border-top: 1px solid #e5edf8; }
.binding-header div { display: flex; align-items: baseline; gap: 12px; }
.binding-header strong { color: #10213d; }
.binding-header span { color: #7d8ba6; font-size: 13px; }
.binding-row { display: grid; grid-template-columns: 110px repeat(7, minmax(110px, 1fr)) 48px; gap: 8px; margin-bottom: 10px; }
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
.binding-header { margin: 20px 0 0; padding: 16px 18px; border: 1px solid #dbe7f7; border-bottom: 0; border-radius: 12px 12px 0 0; background: linear-gradient(90deg, #f5f9ff, #fbfdff); }
.binding-header strong { font-size: 16px; }
.binding-row { padding: 12px; border: 1px solid #dbe7f7; border-top: 1px dashed #e3ebf7; background: #fff; }
.binding-row:last-of-type { border-radius: 0 0 12px 12px; }
</style>
