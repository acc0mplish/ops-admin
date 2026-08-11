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
    ElMessage.warning('请填写网关名称、地址和凭据')
    return
  }
  if (isEdit.value) {
    await updateAssetGateway(form)
    ElMessage.success('网关已更新')
  } else {
    await addAssetGateway(form)
    ElMessage.success('网关已创建')
  }
  dialogVisible.value = false
  loadData()
}

async function handleTest(row) {
  await testAssetGateway(row.id)
  ElMessage.success('网关连接正常')
  loadData()
}

async function toggleStatus(row) {
  await updateAssetGatewayStatus({ id: row.id, status: row.status === 1 ? 2 : 1 })
  ElMessage.success(row.status === 1 ? '已禁用网关' : '已启用网关')
  loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除网关「${row.name}」？`, '删除确认', { type: 'warning' })
  await deleteAssetGateway(row.id)
  ElMessage.success('已删除网关')
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
        <h1>网关管理</h1>
        <p>维护 SSH 跳板网关，为内网主机、数据库和 K8s 集群提供统一访问入口。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增网关</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" placeholder="搜索名称 / 地址 / 网络区域" clearable style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" placeholder="状态" clearable style="width: 140px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
      <el-button @click="Object.assign(query, { keyword: '', status: '', pageNum: 1 }); loadData()">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="list" class="gateway-table">
      <el-table-column prop="name" label="网关名称" min-width="160" />
      <el-table-column prop="host" label="网关地址" min-width="170">
        <template #default="{ row }">{{ row.host }}:{{ row.port || 22 }}</template>
      </el-table-column>
      <el-table-column label="凭据" min-width="150">
        <template #default="{ row }">{{ row.credential?.name || '-' }}</template>
      </el-table-column>
      <el-table-column prop="networkZone" label="网络区域" min-width="140" />
      <el-table-column label="引用资产" min-width="210">
        <template #default="{ row }">
          <span>主机 {{ row.hostCount || 0 }}</span>
          <el-divider direction="vertical" />
          <span>数据库 {{ row.databaseCount || 0 }}</span>
          <el-divider direction="vertical" />
          <span>K8s {{ row.clusterCount || 0 }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="连通状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.connectStatus === 1 ? 'success' : row.connectStatus === 2 ? 'danger' : 'info'">
            {{ row.connectStatus === 1 ? '正常' : row.connectStatus === 2 ? '失败' : '未检测' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最近检测" min-width="190"><template #default="{ row }">{{ formatDateTime(row.lastCheckTime) }}</template></el-table-column>
      <el-table-column label="操作" fixed="right" width="260">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">测试</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link :type="row.status === 1 ? 'warning' : 'success'" @click="toggleStatus(row)">
            {{ row.status === 1 ? '禁用' : '启用' }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑网关' : '新增网关'" width="720px">
      <el-form label-width="110px">
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item label="网关名称" required>
              <el-input v-model="form.name" placeholder="例如：prod-bastion" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="网关编码">
              <el-input v-model="form.code" placeholder="可选，例如 prod-vpc-a" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="网关地址" required>
              <el-input v-model="form.host" placeholder="IP 或域名" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="SSH端口" required>
              <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="登录凭据" required>
              <el-select v-model="form.credentialId" filterable placeholder="请选择网关凭据" style="width: 100%">
                <el-option v-for="item in credentials" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="网络区域">
              <el-input v-model="form.networkZone" placeholder="例如：prod-vpc / idc-a" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="备注">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveGateway">保存</el-button>
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
