<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addAdmin, adminInfo, adminUpdate, deleteAdmin, previewLDAPUsers, queryAdminList, queryDeptList, querySysPostVoList, querySysRoleVoList, resetPassword, syncLDAPUsers, updateAdminStatus } from '../../api/system'

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

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  username: '',
  status: ''
})

const form = reactive({
  id: undefined,
  username: '',
  password: '',
  nickname: '',
  roleId: undefined,
  deptId: undefined,
  postId: undefined,
  email: '',
  phone: '',
  note: '',
  status: 1
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    username: '',
    password: '',
    nickname: '',
    roleId: undefined,
    deptId: undefined,
    postId: undefined,
    email: '',
    phone: '',
    note: '',
    status: 1
  })
}

async function loadOptions() {
  const [roleData, deptData, postData] = await Promise.all([
    querySysRoleVoList(),
    queryDeptList(),
    querySysPostVoList()
  ])
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

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await adminInfo(row.id)
  Object.assign(form, data, { password: '' })
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await adminUpdate(form)
    ElMessage.success('사용자 정보를 업데이트했습니다.')
  } else {
    await addAdmin(form)
    ElMessage.success('사용자를 생성했습니다.')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`사용자 ${row.username}을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' })
  await deleteAdmin(row.id)
  ElMessage.success('사용자를 삭제했습니다.')
  await loadData()
}

async function toggleStatus(row) {
  const next = row.status === 1 ? 2 : 1
  await updateAdminStatus(row.id, next)
  ElMessage.success('사용자 상태를 변경했습니다.')
  await loadData()
}

async function handleResetPassword(row) {
  await resetPassword(row.id, '123456')
  ElMessage.success('비밀번호를 123456으로 초기화했습니다.')
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
    ElMessage.warning('동기화할 LDAP 사용자를 하나 이상 선택하십시오.')
    return
  }
  ldapSyncing.value = true
  try {
    const result = await syncLDAPUsers(selectedLDAPUsers.value)
    ElMessage.success(`LDAP 동기화 완료: 신규 ${result.created || 0}명, 업데이트 ${result.updated || 0}명`)
    ldapDialogVisible.value = false
    await loadData()
  } finally {
    ldapSyncing.value = false
  }
}

onMounted(async () => {
  await loadOptions()
  await loadData()
})
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">사용자 관리</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.username" placeholder="사용자명으로 검색" clearable style="width: 220px" />
        <el-select v-model="query.status" placeholder="상태" clearable style="width: 140px">
          <el-option label="활성" :value="1" />
          <el-option label="비활성" :value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">조회</el-button>
      </div>
      <div class="toolbar-right">
        <el-button v-permission="'system:admin:ldapSync'" @click="openLDAPSync">LDAP에서 동기화</el-button>
        <el-button v-permission="'system:admin:add'" type="primary" @click="openCreate">사용자 추가</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="계정" min-width="120" />
      <el-table-column prop="nickname" label="표시 이름" min-width="120" />
      <el-table-column prop="roleName" label="Role" min-width="140" />
      <el-table-column prop="deptName" label="부서" min-width="140" />
      <el-table-column prop="postName" label="직무" min-width="140" />
      <el-table-column prop="email" label="이메일" min-width="180" />
      <el-table-column prop="phone" label="휴대전화" min-width="140" />
      <el-table-column label="상태" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '활성' : '비활성' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="작업" width="320" fixed="right">
        <template #default="{ row }">
          <el-button v-permission="'system:admin:edit'" link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button v-permission="'system:admin:status'" link type="warning" @click="toggleStatus(row)">{{ row.status === 1 ? '비활성화' : '활성화' }}</el-button>
          <el-button v-permission="'system:admin:resetpwd'" link type="success" @click="handleResetPassword(row)">비밀번호 초기화</el-button>
          <el-button v-permission="'system:admin:delete'" link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top:16px;display:flex;justify-content:flex-end;">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        :total="total"
        layout="total, prev, pager, next, sizes"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '사용자 수정' : '사용자 추가'" width="720px">
      <el-form label-width="90px">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="계정"><el-input v-model="form.username" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="표시 이름"><el-input v-model="form.nickname" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="비밀번호"><el-input v-model="form.password" type="password" show-password :placeholder="isEdit ? '변경하지 않으려면 비워 두십시오.' : '비밀번호를 입력하십시오.'" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Role"><el-select v-model="form.roleId" style="width:100%"><el-option v-for="item in roles" :key="item.id" :label="item.roleName" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="부서"><el-select v-model="form.deptId" style="width:100%"><el-option v-for="item in depts" :key="item.id" :label="item.deptName" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="직무"><el-select v-model="form.postId" style="width:100%"><el-option v-for="item in posts" :key="item.id" :label="item.postName" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="이메일"><el-input v-model="form.email" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="휴대전화"><el-input v-model="form.phone" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="상태"><el-radio-group v-model="form.status"><el-radio :value="1">활성</el-radio><el-radio :value="2">비활성</el-radio></el-radio-group></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="비고"><el-input v-model="form.note" type="textarea" :rows="3" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submit">{{ isEdit ? '저장' : '생성' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ldapDialogVisible" title="LDAP 사용자 동기화" width="900px" destroy-on-close>
      <div class="ldap-dialog-tip">Directory Service에서 사용자를 먼저 조회한 뒤 동기화할 계정을 선택하십시오. 기존 로컬 사용자는 표시 이름, 이메일, 휴대전화만 갱신하며 비밀번호, Role, 상태는 덮어쓰지 않습니다.</div>
      <div class="ldap-toolbar">
        <el-input v-model="ldapKeyword" placeholder="사용자명으로 LDAP 사용자 필터링" clearable @keyup.enter="loadLDAPUsers" />
        <el-button :loading="ldapLoading" @click="loadLDAPUsers">LDAP 조회</el-button>
      </div>
      <el-table v-loading="ldapLoading" :data="ldapUsers" row-key="username" @selection-change="(rows) => { selectedLDAPUsers = rows.map((item) => item.username) }">
        <el-table-column type="selection" width="52" :reserve-selection="true" />
        <el-table-column prop="username" label="사용자명" min-width="160" />
        <el-table-column prop="nickname" label="표시 이름" min-width="160" />
        <el-table-column prop="email" label="이메일" min-width="210" />
        <el-table-column prop="phone" label="휴대전화" min-width="150" />
        <el-table-column prop="dn" label="DN" min-width="260" show-overflow-tooltip />
      </el-table>
      <template #footer>
        <el-button @click="ldapDialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="ldapSyncing" @click="submitLDAPSync">선택 사용자 동기화 ({{ selectedLDAPUsers.length }})</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ldap-dialog-tip { margin-bottom: 14px; padding: 11px 13px; color: #486283; background: #f2f7ff; border: 1px solid #dce9ff; border-radius: 6px; }
.ldap-toolbar { display: flex; gap: 12px; margin-bottom: 14px; }
.ldap-toolbar .el-input { width: 320px; }
</style>
