<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addAdmin, adminInfo, adminUpdate, deleteAdmin, previewLDAPUsers, queryAdminList, queryDeptList, querySysPostVoList, querySysRoleVoList, resetPassword, syncLDAPUsers, updateAdminStatus } from '../../api/system'
import { translateEntity } from '../../utils/i18n-runtime'
import { st } from '../../utils/system-i18n'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const total = ref(0)
const roles = ref([])
const depts = ref([])
const posts = ref([])
const ldapDialogVisible = ref(false)
const ldapLoading = ref(false)
const ldapSyncing = ref(false)
const ldapKeyword = ref('')
const ldapUsers = ref([])
const selectedLDAPUsers = ref([])

const query = reactive({ pageNum: 1, pageSize: 10, username: '', status: '' })
const form = reactive({ id: undefined, username: '', password: '', nickname: '', roleId: undefined, deptId: undefined, postId: undefined, email: '', phone: '', note: '', status: 1 })

function resetForm() {
  Object.assign(form, { id: undefined, username: '', password: '', nickname: '', roleId: undefined, deptId: undefined, postId: undefined, email: '', phone: '', note: '', status: 1 })
}

async function loadOptions() {
  const [roleData, deptData, postData] = await Promise.all([querySysRoleVoList(), queryDeptList(), querySysPostVoList()])
  roles.value = roleData || []
  depts.value = deptData || []
  posts.value = postData || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAdminList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() { isEdit.value = false; resetForm(); dialogVisible.value = true }

async function openEdit(row) {
  isEdit.value = true
  const data = await adminInfo(row.id)
  Object.assign(form, data, { password: '' })
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await adminUpdate(form)
    ElMessage.success(st('userUpdated'))
  } else {
    await addAdmin(form)
    ElMessage.success(st('userCreated'))
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(st('deleteUserConfirm', { name: row.username }), st('deleteConfirm'), { type: 'warning' })
  await deleteAdmin(row.id)
  ElMessage.success(st('userDeleted'))
  await loadData()
}

async function toggleStatus(row) {
  const next = row.status === 1 ? 2 : 1
  await updateAdminStatus(row.id, next)
  ElMessage.success(st('userStatusUpdated'))
  await loadData()
}

async function handleResetPassword(row) {
  await resetPassword(row.id, '123456')
  ElMessage.success(st('passwordResetDefault'))
}

async function openLDAPSync() {
  ldapDialogVisible.value = true
  ldapKeyword.value = ''
  selectedLDAPUsers.value = []
  await loadLDAPUsers()
}

async function loadLDAPUsers() {
  ldapLoading.value = true
  try {
    ldapUsers.value = await previewLDAPUsers(ldapKeyword.value) || []
    selectedLDAPUsers.value = []
  } finally {
    ldapLoading.value = false
  }
}

async function submitLDAPSync() {
  if (!selectedLDAPUsers.value.length) {
    ElMessage.warning(st('ldapSelectRequired'))
    return
  }
  ldapSyncing.value = true
  try {
    const result = await syncLDAPUsers(selectedLDAPUsers.value)
    ElMessage.success(st('ldapSyncResult', { created: result.created || 0, updated: result.updated || 0 }))
    ldapDialogVisible.value = false
    await loadData()
  } finally {
    ldapSyncing.value = false
  }
}

onMounted(async () => { await loadOptions(); await loadData() })
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('userManagement') }}</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.username" :placeholder="st('searchUsername')" clearable style="width: 220px" />
        <el-select v-model="query.status" :placeholder="st('status')" clearable style="width: 140px">
          <el-option :label="st('active')" :value="1" />
          <el-option :label="st('inactive')" :value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ st('query') }}</el-button>
      </div>
      <div class="toolbar-right">
        <el-button v-permission="'system:admin:ldapSync'" @click="openLDAPSync">{{ st('syncFromLdap') }}</el-button>
        <el-button v-permission="'system:admin:add'" type="primary" @click="openCreate">{{ st('addUser') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" :label="st('account')" min-width="120" />
      <el-table-column prop="nickname" :label="st('displayName')" min-width="120" />
      <el-table-column prop="roleName" :label="st('role')" min-width="140">
        <template #default="{ row }">{{ translateEntity(row.roleName, row.roleName || '-') }}</template>
      </el-table-column>
      <el-table-column prop="deptName" :label="st('department')" min-width="140">
        <template #default="{ row }">{{ translateEntity(row.deptName, row.deptName || '-') }}</template>
      </el-table-column>
      <el-table-column prop="postName" :label="st('position')" min-width="140">
        <template #default="{ row }">{{ translateEntity(row.postName, row.postName || '-') }}</template>
      </el-table-column>
      <el-table-column prop="email" :label="st('email')" min-width="180" />
      <el-table-column prop="phone" :label="st('phone')" min-width="140" />
      <el-table-column :label="st('status')" width="100">
        <template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? st('active') : st('inactive') }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="st('actions')" width="320" fixed="right">
        <template #default="{ row }">
          <el-button v-permission="'system:admin:edit'" link type="primary" @click="openEdit(row)">{{ st('edit') }}</el-button>
          <el-button v-permission="'system:admin:status'" link type="warning" @click="toggleStatus(row)">{{ row.status === 1 ? st('deactivate') : st('activate') }}</el-button>
          <el-button v-permission="'system:admin:resetpwd'" link type="success" @click="handleResetPassword(row)">{{ st('resetPassword') }}</el-button>
          <el-button v-permission="'system:admin:delete'" link type="danger" @click="handleDelete(row)">{{ st('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top:16px;display:flex;justify-content:flex-end;">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, prev, pager, next, sizes" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? st('editUser') : st('addUser')" width="720px">
      <el-form label-width="90px">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item :label="st('account')"><el-input v-model="form.username" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('displayName')"><el-input v-model="form.nickname" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('password')"><el-input v-model="form.password" type="password" show-password :placeholder="isEdit ? st('passwordKeep') : st('passwordInput')" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('role')"><el-select v-model="form.roleId" style="width:100%"><el-option v-for="item in roles" :key="item.id" :label="translateEntity(item.roleName, item.roleName)" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('department')"><el-select v-model="form.deptId" style="width:100%"><el-option v-for="item in depts" :key="item.id" :label="translateEntity(item.deptName, item.deptName)" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('position')"><el-select v-model="form.postId" style="width:100%"><el-option v-for="item in posts" :key="item.id" :label="translateEntity(item.postName, item.postName)" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('email')"><el-input v-model="form.email" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('phone')"><el-input v-model="form.phone" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="st('status')"><el-radio-group v-model="form.status"><el-radio :value="1">{{ st('active') }}</el-radio><el-radio :value="2">{{ st('inactive') }}</el-radio></el-radio-group></el-form-item></el-col>
          <el-col :span="24"><el-form-item :label="st('note')"><el-input v-model="form.note" type="textarea" :rows="3" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" @click="submit">{{ isEdit ? st('save') : st('create') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="ldapDialogVisible" :title="st('ldapSyncTitle')" width="900px" destroy-on-close>
      <div class="ldap-dialog-tip">{{ st('ldapSyncTip') }}</div>
      <div class="ldap-toolbar"><el-input v-model="ldapKeyword" :placeholder="st('ldapFilter')" clearable @keyup.enter="loadLDAPUsers" /><el-button :loading="ldapLoading" @click="loadLDAPUsers">{{ st('ldapQuery') }}</el-button></div>
      <el-table v-loading="ldapLoading" :data="ldapUsers" row-key="username" @selection-change="(rows) => { selectedLDAPUsers = rows.map((item) => item.username) }">
        <el-table-column type="selection" width="52" :reserve-selection="true" />
        <el-table-column prop="username" :label="st('username')" min-width="160" />
        <el-table-column prop="nickname" :label="st('displayName')" min-width="160" />
        <el-table-column prop="email" :label="st('email')" min-width="210" />
        <el-table-column prop="phone" :label="st('phone')" min-width="150" />
        <el-table-column prop="dn" label="DN" min-width="260" show-overflow-tooltip />
      </el-table>
      <template #footer><el-button @click="ldapDialogVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" :loading="ldapSyncing" @click="submitLDAPSync">{{ st('syncSelected', { count: selectedLDAPUsers.length }) }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ldap-dialog-tip { margin-bottom: 14px; padding: 11px 13px; color: #486283; background: #f2f7ff; border: 1px solid #dce9ff; border-radius: 6px; }
.ldap-toolbar { display: flex; gap: 12px; margin-bottom: 14px; }
.ldap-toolbar .el-input { width: 320px; }
</style>
