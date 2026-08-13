<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addAssetCredential,
  assetCredentialInfo,
  deleteAssetCredential,
  queryAssetCredentialList,
  updateAssetCredential
} from '../../api/asset'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', authType: '' })
const form = reactive({
  id: undefined,
  name: '',
  authType: 'password',
  username: '',
  password: '',
  privateKey: '',
  passphrase: '',
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    authType: 'password',
    username: '',
    password: '',
    privateKey: '',
    passphrase: '',
    status: 1,
    description: ''
  })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetCredentialList(query)
    tableData.value = data.list || []
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
  const data = await assetCredentialInfo(row.id)
  Object.assign(form, data, { password: '', privateKey: '', passphrase: '' })
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await updateAssetCredential(form)
    ElMessage.success('凭据已更新')
  } else {
    await addAssetCredential(form)
    ElMessage.success('凭据已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除凭据 ${row.name} 吗？`, '提示', { type: 'warning' })
  await deleteAssetCredential(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card asset-card-page">
    <h2 class="page-title">凭据管理</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="搜索名称 / 用户名" style="width: 220px" @keyup.enter="loadData" />
        <el-select v-model="query.authType" clearable placeholder="认证类型" style="width: 140px">
          <el-option label="密码认证" value="password" />
          <el-option label="密钥认证" value="key" />
        </el-select>
        <el-button @click="loadData">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreate">新增凭据</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="凭据名称" min-width="180" />
      <el-table-column label="认证类型" width="120">
        <template #default="{ row }">
          <el-tag>{{ row.authType === 'key' ? '密钥认证' : '密码认证' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '正常' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="备注" min-width="220" />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        layout="total, sizes, prev, pager, next"
        :total="total"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑凭据' : '新增凭据'" width="640px">
      <el-form label-width="96px">
        <el-form-item label="凭据名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="认证类型">
          <el-radio-group v-model="form.authType">
            <el-radio value="password">密码认证</el-radio>
            <el-radio value="key">密钥认证</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item>
        <el-form-item v-if="form.authType === 'password'" label="密码">
          <el-input v-model="form.password" show-password :placeholder="isEdit ? '不填写则保持不变' : ''" />
        </el-form-item>
        <template v-else>
          <el-form-item label="私钥">
            <el-input v-model="form.privateKey" type="textarea" :rows="5" :placeholder="isEdit ? '不填写则保持不变' : ''" />
          </el-form-item>
          <el-form-item label="密钥口令">
            <el-input v-model="form.passphrase" show-password :placeholder="isEdit ? '不填写则保持不变' : ''" />
          </el-form-item>
        </template>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">正常</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-card { min-height: 300px; }
.page-title { margin-bottom: 14px; }
.toolbar { padding: 12px; border: 1px solid #e8edf3; border-radius: 9px; background: #f9fafc; }
.pager { margin-top: 16px; padding-top: 14px; border-top: 1px solid #edf0f5; display: flex; justify-content: flex-end; }
</style>
