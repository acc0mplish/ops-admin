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

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', type: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  type: 'prometheus',
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
    ElMessage.warning('请填写数据源名称和地址')
    return
  }
  saving.value = true
  try {
    await saveMonitorDatasource(form)
    ElMessage.success('保存成功')
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
    ElMessage.success('数据源连接成功')
  } finally {
    testing.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除数据源「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteMonitorDatasource(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>数据源管理</h2>
        <p>接入 Prometheus 或 VictoriaMetrics，供指标查询和告警规则复用。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增数据源</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索名称 / 地址" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.type" clearable placeholder="类型" style="width: 170px">
        <el-option label="Prometheus" value="prometheus" />
        <el-option label="VictoriaMetrics" value="victoriametrics" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 130px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="名称" min-width="180" />
      <el-table-column prop="type" label="类型" width="150" />
      <el-table-column prop="url" label="地址" min-width="260" show-overflow-tooltip />
      <el-table-column label="认证" width="120">
        <template #default="{ row }">{{ row.authType || 'none' }}</template>
      </el-table-column>
      <el-table-column label="默认" width="90" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.isDefault" type="success">默认</el-tag>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">测试</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑数据源' : '新增数据源'" width="760px">
      <el-form label-width="110px">
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="form.type">
            <el-radio-button label="prometheus">Prometheus</el-radio-button>
            <el-radio-button label="victoriametrics">VictoriaMetrics</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="地址" required><el-input v-model="form.url" placeholder="http://prometheus:9090" /></el-form-item>
        <el-form-item label="认证方式">
          <el-select v-model="form.authType" style="width: 100%">
            <el-option label="无认证" value="none" />
            <el-option label="Basic" value="basic" />
            <el-option label="Bearer Token" value="bearer" />
          </el-select>
        </el-form-item>
        <template v-if="form.authType === 'basic'">
          <el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item>
          <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        </template>
        <el-form-item v-if="form.authType === 'bearer'" label="Token">
          <el-input v-model="form.token" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="默认数据源"><el-switch v-model="form.isDefault" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button :loading="testing" @click="handleTest()">测试连接</el-button>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
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
