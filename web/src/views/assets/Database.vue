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
  env: '',
  tag: ''
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
  tags: [],
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
    tags: [],
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
  if (row.connectStatus === 1) return '已连接'
  if (row.connectStatus === 2) return '连接异常'
  return '未检测'
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
  Object.assign(form, data, { password: '', tags: data.tags || [] })
  dialogVisible.value = true
}

async function handleTest() {
  testing.value = true
  try {
    const data = await testAssetDatabase(form)
    ElMessage.success(`连接成功，${databaseTypeLabel(data.dbType || form.dbType)} 版本 ${data.version || '-'}`)
  } finally {
    testing.value = false
  }
}

async function submit() {
  if (form.connectionMode === 'gateway' && !form.gatewayId) {
    ElMessage.warning('请选择访问网关')
    return
  }
  if (isEdit.value) {
    await updateAssetDatabase(form)
    ElMessage.success('数据库资产已更新')
  } else {
    await addAssetDatabase(form)
    ElMessage.success('数据库资产已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除数据库资产 ${row.name} 吗？`, '提示', { type: 'warning' })
  await deleteAssetDatabase(row.id)
  ElMessage.success('删除成功')
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
  <div class="database-page page-card">
    <div class="page-header">
      <div>
        <h2 class="page-title">数据库管理</h2>
        <p class="page-desc">统一维护 MySQL、PostgreSQL、MongoDB 与 Redis 资产，并按数据库类型提供连接、结构与工作台能力。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增数据库</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="搜索数据库名称 / 主机 / 库名"
          style="width: 260px"
          @keyup.enter="loadData"
        />
        <el-select v-model="query.dbType" clearable style="width: 140px" placeholder="数据库类型">
          <el-option label="全部类型" value="" />
          <el-option label="MySQL" value="mysql" />
          <el-option label="PostgreSQL" value="postgresql" />
          <el-option label="MongoDB" value="mongodb" />
          <el-option label="Redis" value="redis" />
        </el-select>
        <el-select v-model="query.status" clearable style="width: 140px" placeholder="状态">
          <el-option label="启用" value="1" />
          <el-option label="停用" value="2" />
        </el-select>
        <el-select v-model="query.env" clearable style="width: 160px" placeholder="全部环境" @change="loadData">
          <el-option v-for="item in environmentOptions" :key="item.code" :label="item.name" :value="item.code" />
        </el-select>
        <el-input v-model="query.tag" clearable placeholder="标签" style="width: 150px" @keyup.enter="loadData" />
        <el-button type="primary" @click="loadData">搜索</el-button>
        <el-button @click="Object.assign(query, { pageNum: 1, pageSize: 10, keyword: '', dbType: '', status: '', env: '', tag: '' }); loadData()">重置</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border class="data-table">
      <el-table-column label="数据库名称" min-width="180">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ row.name }}</el-button>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="120">
        <template #default="{ row }"><el-tag effect="plain">{{ databaseTypeLabel(row.dbType) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="访问模式" width="110">
        <template #default="{ row }">
          <el-tag :type="row.accessMode === 'readonly' ? 'warning' : 'success'" effect="plain">
            {{ row.accessMode === 'readonly' ? '只读' : '读写' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="环境" width="120">
        <template #default="{ row }">
          <el-tag effect="plain">{{ environmentName(row.env) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="标签" min-width="160">
        <template #default="{ row }">
          <el-tag v-for="tag in row.tags || []" :key="tag" class="asset-tag" effect="plain">{{ tag }}</el-tag>
          <span v-if="!row.tags?.length">-</span>
        </template>
      </el-table-column>
      <el-table-column label="连接地址" min-width="220">
        <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
      </el-table-column>
      <el-table-column label="访问方式" min-width="150">
        <template #default="{ row }">
          <span v-if="row.connectionMode === 'gateway'">网关：{{ row.gateway?.name || '-' }}</span>
          <span v-else>直连</span>
        </template>
      </el-table-column>
      <el-table-column prop="dbName" label="默认库" min-width="140" />
      <el-table-column prop="username" label="账号" min-width="120" />
      <el-table-column prop="version" label="版本" min-width="140" />
      <el-table-column label="连接状态" width="110">
        <template #default="{ row }">
          <el-tag :type="connectionStatusType(row)" effect="light">{{ connectionStatusText(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最近检测" min-width="170"><template #default="{ row }">{{ formatDateTime(row.lastCheckTime) }}</template></el-table-column>
      <el-table-column prop="description" label="备注" min-width="180" />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openWorkbench(row)">工作台</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑数据库资产' : '新增数据库资产'" width="720px">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="数据库名称"><el-input v-model="form.name" placeholder="例如：order-prod" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="数据库类型">
              <el-select v-model="form.dbType" style="width: 100%" @change="onDatabaseTypeChange">
                <el-option label="MySQL" value="mysql" />
                <el-option label="PostgreSQL" value="postgresql" />
                <el-option label="MongoDB" value="mongodb" />
                <el-option label="Redis" value="redis" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="所属环境" required>
              <el-select v-model="form.env" :loading="environmentLoading" style="width: 100%" placeholder="请选择环境">
                <el-option v-for="item in environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="访问模式" required>
              <el-radio-group v-model="form.accessMode">
                <el-radio-button value="readonly">只读</el-radio-button>
                <el-radio-button value="readwrite">读写</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="资产标签">
              <el-select v-model="form.tags" multiple filterable allow-create default-first-option style="width: 100%" placeholder="输入标签后回车，例如：核心、支付">
                <el-option v-for="tag in form.tags" :key="tag" :label="tag" :value="tag" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="主机地址"><el-input v-model="form.host" placeholder="例如：10.0.0.12" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="端口"><el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="form.dbType === 'redis' || form.dbType === 'mongodb' ? '用户名（可选）' : '用户名'">
              <el-input v-model="form.username" :placeholder="form.dbType === 'redis' || form.dbType === 'mongodb' ? '未启用认证时可留空' : ''" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="密码"><el-input v-model="form.password" show-password :placeholder="isEdit ? '留空则保持不变' : ''" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="form.dbType === 'redis' ? '逻辑库编号' : '默认库'">
              <el-input v-model="form.dbName" :placeholder="form.dbType === 'redis' ? '例如：0' : form.dbType === 'mongodb' ? '例如：admin' : '例如：app_db'" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="字符集">
              <el-input v-model="form.charset" :disabled="form.dbType === 'mongodb' || form.dbType === 'redis'" placeholder="默认 utf8mb4" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="连接方式">
              <el-radio-group v-model="form.connectionMode">
                <el-radio-button label="direct">直连</el-radio-button>
                <el-radio-button label="gateway">通过网关</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col v-if="form.connectionMode === 'gateway'" :span="12">
            <el-form-item label="访问网关" required>
              <el-select v-model="form.gatewayId" filterable placeholder="请选择网关" style="width: 100%">
                <el-option v-for="item in gatewayOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="备注"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">停用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button :loading="testing" @click="handleTest">测试连接</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.database-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.asset-tag { margin-right: 6px; }

.page-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.page-desc {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
}

.toolbar,
.toolbar-left {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.pager {
  display: flex;
  justify-content: flex-end;
}
</style>
