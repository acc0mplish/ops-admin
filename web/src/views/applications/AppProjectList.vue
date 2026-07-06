<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsApplication, queryOpsApplicationList, saveOpsApplication } from '../../api/ops'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const rows = ref([])
const total = ref(0)

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  serviceType: ''
})

const form = reactive({
  id: undefined,
  name: '',
  code: '',
  serviceType: '后端服务',
  repoType: 'git',
  repoUrl: '',
  branch: 'master',
  workspace: '',
  env: 'test',
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    code: '',
    serviceType: '后端服务',
    repoType: 'git',
    repoUrl: '',
    branch: 'master',
    workspace: '',
    env: 'test',
    status: 1,
    description: ''
  })
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

function openEdit(row) {
  Object.assign(form, {
    id: row.id,
    name: row.name || '',
    code: row.code || '',
    serviceType: row.serviceType || row.env || '后端服务',
    repoType: row.repoType || 'git',
    repoUrl: row.repoUrl || '',
    branch: row.branch || 'master',
    workspace: row.workspace || '',
    env: row.env || 'test',
    status: row.status || 1,
    description: row.description || ''
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

onMounted(loadData)
</script>

<template>
  <div class="app-page">
    <div class="app-header">
      <div>
        <h1>项目列表</h1>
        <p>维护应用项目、服务类别和 Git/SVN 仓库地址，构建任务会基于这里的仓库拉取代码。</p>
      </div>
      <el-button type="primary" @click="openCreate">+ 新建项目</el-button>
    </div>

    <div class="filter-panel">
      <el-form inline>
        <el-form-item label="项目名称">
          <el-input v-model="query.keyword" clearable placeholder="请输入项目名称 / 仓库地址" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="服务类别">
          <el-select v-model="query.serviceType" clearable placeholder="请选择服务类别">
            <el-option label="前端服务" value="前端服务" />
            <el-option label="后端服务" value="后端服务" />
            <el-option label="中间件" value="中间件" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">搜索</el-button>
          <el-button @click="Object.assign(query, { keyword: '', serviceType: '', pageNum: 1 }); loadData()">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column prop="name" label="项目名称" min-width="170">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.code }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="项目描述" min-width="170" show-overflow-tooltip />
        <el-table-column prop="serviceType" label="服务类别" width="120">
          <template #default="{ row }">{{ row.serviceType || row.env || '-' }}</template>
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

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑项目' : '新建项目'" width="720px">
      <el-form :model="form" label-width="96px">
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="项目名称" required><el-input v-model="form.name" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="项目编码" required><el-input v-model="form.code" /></el-form-item>
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
            <el-form-item label="默认分支"><el-input v-model="form.branch" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="默认环境"><el-input v-model="form.env" /></el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="工作目录"><el-input v-model="form.workspace" placeholder="可选，默认 uploads/apps/项目编码" /></el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="项目描述"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :label="1">启用</el-radio>
                <el-radio :label="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
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
.table-card { border-radius: 12px; }
.name-cell { display: flex; flex-direction: column; gap: 4px; }
.name-cell strong { color: #1677ff; }
.name-cell span, .repo-url { color: #697b99; }
.repo-url { margin-left: 8px; }
.pager { display: flex; justify-content: flex-end; padding-top: 16px; }
</style>
