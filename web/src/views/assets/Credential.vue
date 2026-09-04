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
    ElMessage.success('Credential을 업데이트했습니다.')
  } else {
    await addAssetCredential(form)
    ElMessage.success('Credential을 생성했습니다.')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Credential “${row.name}”을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteAssetCredential(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card asset-card-page">
    <h2 class="page-title">Credential 관리</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="이름 / 사용자 이름 검색" style="width: 220px" @keyup.enter="loadData" />
        <el-select v-model="query.authType" clearable placeholder="인증 방식" style="width: 140px">
          <el-option label="비밀번호 인증" value="password" />
          <el-option label="키 인증" value="key" />
        </el-select>
        <el-button @click="loadData">검색</el-button>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreate">Credential 추가</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Credential 이름" min-width="180" />
      <el-table-column label="인증 방식" width="120">
        <template #default="{ row }">
          <el-tag>{{ row.authType === 'key' ? '키 인증' : '비밀번호 인증' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="username" label="사용자 이름" min-width="140" />
      <el-table-column label="상태" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '정상' : '사용 중지' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="비고" min-width="220" />
      <el-table-column label="작업" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Credential 수정' : 'Credential 추가'" width="640px">
      <el-form label-width="96px">
        <el-form-item label="Credential 이름"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="인증 방식">
          <el-radio-group v-model="form.authType">
            <el-radio value="password">비밀번호 인증</el-radio>
            <el-radio value="key">키 인증</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="사용자 이름"><el-input v-model="form.username" /></el-form-item>
        <el-form-item v-if="form.authType === 'password'" label="비밀번호">
          <el-input v-model="form.password" show-password :placeholder="isEdit ? '입력하지 않으면 기존 값을 유지합니다' : ''" />
        </el-form-item>
        <template v-else>
          <el-form-item label="Private Key">
            <el-input v-model="form.privateKey" type="textarea" :rows="5" :placeholder="isEdit ? '입력하지 않으면 기존 값을 유지합니다' : ''" />
          </el-form-item>
          <el-form-item label="Key Passphrase">
            <el-input v-model="form.passphrase" show-password :placeholder="isEdit ? '입력하지 않으면 기존 값을 유지합니다' : ''" />
          </el-form-item>
        </template>
        <el-form-item label="상태">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">정상</el-radio>
            <el-radio :value="2">사용 중지</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="비고"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submit">저장</el-button>
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
