<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { at } from '../../utils/asset-i18n'
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
    ElMessage.success(at('credentialUpdated'))
  } else {
    await addAssetCredential(form)
    ElMessage.success(at('credentialCreated'))
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(at('deleteCredentialConfirm', { name: row.name }), at('notice'), { type: 'warning' })
  await deleteAssetCredential(row.id)
  ElMessage.success(at('rowDeleted'))
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card asset-card-page">
    <h2 class="page-title">{{ at('credentialManageTitle') }}</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="at('credentialSearchPlaceholder')" style="width: 220px" @keyup.enter="loadData" />
        <el-select v-model="query.authType" clearable :placeholder="at('authTypeLabel')" style="width: 140px">
          <el-option :label="at('passwordAuthOption')" value="password" />
          <el-option :label="at('keyAuthOption')" value="key" />
        </el-select>
        <el-button @click="loadData">{{ at('search') }}</el-button>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreate">{{ at('addCredentialButton') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" :label="at('credentialNameColumn')" min-width="180" />
      <el-table-column :label="at('authTypeLabel')" width="120">
        <template #default="{ row }">
          <el-tag>{{ row.authType === 'key' ? at('keyAuthOption') : at('passwordAuthOption') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="username" :label="at('usernameLabel')" min-width="140" />
      <el-table-column :label="at('status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? at('groupNormal') : at('disabledStatus') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" :label="at('noteLabel')" min-width="220" />
      <el-table-column :label="at('actions')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ at('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ at('delete') }}</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? at('editCredentialTitle') : at('addCredentialButton')" width="640px">
      <el-form label-width="96px">
        <el-form-item :label="at('credentialNameColumn')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="at('authTypeLabel')">
          <el-radio-group v-model="form.authType">
            <el-radio value="password">{{ at('passwordAuthOption') }}</el-radio>
            <el-radio value="key">{{ at('keyAuthOption') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="at('usernameLabel')"><el-input v-model="form.username" /></el-form-item>
        <el-form-item v-if="form.authType === 'password'" :label="at('passwordLabel')">
          <el-input v-model="form.password" show-password :placeholder="isEdit ? at('keepExistingPlaceholder') : ''" />
        </el-form-item>
        <template v-else>
          <el-form-item label="Private Key">
            <el-input v-model="form.privateKey" type="textarea" :rows="5" :placeholder="isEdit ? at('keepExistingPlaceholder') : ''" />
          </el-form-item>
          <el-form-item label="Key Passphrase">
            <el-input v-model="form.passphrase" show-password :placeholder="isEdit ? at('keepExistingPlaceholder') : ''" />
          </el-form-item>
        </template>
        <el-form-item :label="at('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ at('groupNormal') }}</el-radio>
            <el-radio :value="2">{{ at('disabledStatus') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="at('noteLabel')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" @click="submit">{{ at('save') }}</el-button>
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
