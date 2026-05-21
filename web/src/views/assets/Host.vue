<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addAssetHost,
  assetHostInfo,
  deleteAssetHost,
  queryAssetCredentialOptions,
  queryAssetHostGroupList,
  queryAssetHostList,
  syncAssetHost,
  updateAssetHost
} from '../../api/asset'

const router = useRouter()
const loading = ref(false)
const syncingId = ref()
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const groupOptions = ref([])
const credentialOptions = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '' })
const form = reactive({
  id: undefined,
  hostName: '',
  groupId: undefined,
  sshUser: '',
  sshIp: '',
  sshPort: 22,
  credentialId: undefined,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    hostName: '',
    groupId: undefined,
    sshUser: '',
    sshIp: '',
    sshPort: 22,
    credentialId: undefined,
    status: 1,
    description: ''
  })
}

async function loadOptions() {
  const [groups, credentials] = await Promise.all([
    queryAssetHostGroupList(),
    queryAssetCredentialOptions()
  ])
  groupOptions.value = groups.list || []
  credentialOptions.value = credentials || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetHostList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.keyword = ''
  query.status = ''
  query.pageNum = 1
  loadData()
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await assetHostInfo(row.id)
  resetForm()
  Object.assign(form, {
    id: data.id,
    hostName: data.hostName,
    groupId: data.groupId,
    sshUser: data.sshUser,
    sshIp: data.sshIp,
    sshPort: data.sshPort || 22,
    credentialId: data.credentialId,
    status: data.status || 1,
    description: data.description
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.hostName || !form.groupId || !form.sshUser || !form.sshIp || !form.credentialId) {
    ElMessage.warning('请填写主机名称、所属分组、SSH 连接和认证凭据')
    return
  }

  if (isEdit.value) {
    await updateAssetHost(form)
    ElMessage.success('主机已更新')
  } else {
    await addAssetHost(form)
    ElMessage.success('主机已创建，可以点击同步采集公网地址和配置信息')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleSync(row) {
  syncingId.value = row.id
  try {
    await syncAssetHost(row.id)
    ElMessage.success('同步完成')
    await loadData()
  } finally {
    syncingId.value = undefined
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除主机 ${row.hostName} 吗？`, '提示', { type: 'warning' })
  await deleteAssetHost(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function groupName(row) {
  return row.group?.name || '-'
}

function statusText(value, onlineText, offlineText, unknownText = '未检测') {
  if (value === 1) return onlineText
  if (value === 2) return offlineText
  return unknownText
}

function statusType(value) {
  if (value === 1) return 'success'
  if (value === 2) return 'danger'
  return 'info'
}

function configText(row) {
  const parts = [row.cpu, row.memory, row.disk].filter(Boolean)
  return parts.length ? parts.join(' / ') : '待同步'
}

function goCredential() {
  router.push('/assets/server/credentials')
}

function goTerminal() {
  router.push('/assets/terminal')
}

onMounted(async () => {
  await loadOptions()
  await loadData()
})
</script>

<template>
  <div class="asset-host-page">
    <section class="query-panel">
      <el-form inline>
        <el-form-item label="主机名称">
          <el-input v-model="query.keyword" clearable placeholder="请输入主机名称" style="width: 160px" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="IP地址">
          <el-input v-model="query.keyword" clearable placeholder="请输入IP地址" style="width: 160px" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item label="主机状态">
          <el-select v-model="query.status" clearable placeholder="请选择状态" style="width: 140px">
            <el-option label="正常" value="1" />
            <el-option label="停用" value="2" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="query-actions">
        <el-button type="primary" @click="loadData">搜索</el-button>
        <el-button color="#f0a43a" @click="resetQuery">重置</el-button>
        <el-button type="success" @click="openCreate">新增</el-button>
        <el-button color="#6f58c9" @click="goTerminal">终端</el-button>
      </div>
    </section>

    <el-table v-loading="loading" :data="tableData" class="host-table">
      <el-table-column label="主机名称" min-width="180">
        <template #default="{ row }">
          <div class="host-name">
            <span class="linux-icon">🐧</span>
            <span>{{ row.hostName }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="IP地址" min-width="170">
        <template #default="{ row }">
          <div class="ip-list">
            <span v-if="row.publicIp" class="ip public">公 {{ row.publicIp }}</span>
            <span v-if="row.privateIp || row.sshIp" class="ip private">内 {{ row.privateIp || row.sshIp }}</span>
            <span v-if="!row.publicIp && !row.privateIp && !row.sshIp">-</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="CPU使用" width="110">
        <template #default>0%</template>
      </el-table-column>
      <el-table-column label="内存使用" width="120">
        <template #default>0%</template>
      </el-table-column>
      <el-table-column label="磁盘使用" width="120">
        <template #default>0%</template>
      </el-table-column>
      <el-table-column label="配置 信息" min-width="160">
        <template #default="{ row }">
          <div class="config-info">
            <span>{{ configText(row) }}</span>
            <small v-if="row.os">{{ row.os }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="存活状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.aliveStatus)" effect="light">
            {{ statusText(row.aliveStatus, '在线', '离线') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="认证状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.authStatus)" effect="light">
            {{ statusText(row.authStatus, '认证成功', '认证失败') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="主机类型" width="100">
        <template #default="{ row }">{{ row.provider || '自建' }}</template>
      </el-table-column>
      <el-table-column label="所属分组" min-width="120">
        <template #default="{ row }">{{ groupName(row) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="success" :loading="syncingId === row.id" @click="handleSync(row)">同步</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        layout="total, sizes, prev, pager, next, jumper"
        :total="total"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑主机' : '新增主机'" width="640px">
      <el-form label-width="96px">
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item label="主机名称" required>
              <el-input v-model="form.hostName" placeholder="请输入主机名称" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="所属分组" required>
              <el-select v-model="form.groupId" filterable placeholder="请选择分组" style="width: 100%">
                <el-option v-for="item in groupOptions" :key="item.id" :value="item.id" :label="item.name" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="SSH连接" required>
              <div class="ssh-line">
                <el-input v-model="form.sshUser" placeholder="用户名" />
                <span>@</span>
                <el-input v-model="form.sshIp" placeholder="主机地址" />
                <span>-p</span>
                <el-input-number v-model="form.sshPort" :min="1" :max="65535" controls-position="right" />
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="认证凭据" required>
              <div class="credential-line">
                <el-select v-model="form.credentialId" clearable filterable placeholder="请选择认证凭据">
                  <el-option v-for="item in credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
                </el-select>
                <el-button color="#f59e0b" @click="goCredential">+ 创建凭据</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="备注">
              <el-input v-model="form.description" type="textarea" :rows="3" placeholder="请输入备注信息" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.asset-host-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.query-panel {
  padding: 24px 16px;
  border: 1px solid #dfe5ff;
  border-radius: 10px;
  background: #f4f6ff;
}

.query-actions {
  display: flex;
  gap: 10px;
  margin-top: 10px;
}

.host-table {
  overflow: hidden;
  border-radius: 10px;
  box-shadow: 0 10px 28px rgba(31, 45, 87, 0.08);
}

.host-name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #2787ff;
  font-weight: 700;
}

.linux-icon {
  font-size: 18px;
}

.ip-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: #2787ff;
  font-size: 13px;
}

.ip {
  line-height: 1.2;
}

.public::first-letter {
  color: #f59e0b;
}

.private::first-letter {
  color: #54c46a;
}

.config-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.config-info small {
  color: #8190ad;
}

.pager {
  display: flex;
  justify-content: flex-end;
}

.ssh-line,
.credential-line {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.ssh-line .el-input:first-child {
  width: 130px;
}

.ssh-line .el-input:nth-child(3) {
  flex: 1;
}

.ssh-line .el-input-number {
  width: 96px;
}

.credential-line .el-select {
  flex: 1;
}
</style>
