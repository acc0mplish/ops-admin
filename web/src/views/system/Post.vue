<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addPost, deletePost, queryPostList, postInfo, updatePost, updatePostStatus } from '../../api/system'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const form = reactive({ id: undefined, postCode: '', postName: '', postStatus: 1, remark: '' })

function resetForm() {
  Object.assign(form, { id: undefined, postCode: '', postName: '', postStatus: 1, remark: '' })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryPostList()
    tableData.value = data.list || []
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
  Object.assign(form, await postInfo(row.id))
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await updatePost(form)
    ElMessage.success('직무 정보를 업데이트했습니다.')
  } else {
    await addPost(form)
    ElMessage.success('직무를 생성했습니다.')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`직무 ${row.postName}을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' })
  await deletePost(row.id)
  ElMessage.success('직무를 삭제했습니다.')
  await loadData()
}

async function handleStatus(row) {
  await updatePostStatus(row.id, row.postStatus === 1 ? 2 : 1)
  ElMessage.success('직무 상태를 변경했습니다.')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">직무 관리</h2>
    <div class="toolbar">
      <div class="toolbar-left"></div>
      <div class="toolbar-right"><el-button v-permission="'system:post:add'" type="primary" @click="openCreate">직무 추가</el-button></div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="postCode" label="직무 코드" min-width="160" />
      <el-table-column prop="postName" label="직무명" min-width="160" />
      <el-table-column prop="remark" label="비고" min-width="220" />
      <el-table-column label="상태" width="100">
        <template #default="{ row }"><el-tag :type="row.postStatus === 1 ? 'success' : 'danger'">{{ row.postStatus === 1 ? '활성' : '비활성' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="작업" width="200">
        <template #default="{ row }">
          <el-button v-permission="'system:post:edit'" link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button v-permission="'system:post:status'" link type="warning" @click="handleStatus(row)">{{ row.postStatus === 1 ? '비활성화' : '활성화' }}</el-button>
          <el-button v-permission="'system:post:delete'" link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '직무 수정' : '직무 추가'" width="560px">
      <el-form label-width="90px">
        <el-form-item label="직무 코드"><el-input v-model="form.postCode" /></el-form-item>
        <el-form-item label="직무명"><el-input v-model="form.postName" /></el-form-item>
        <el-form-item label="상태"><el-radio-group v-model="form.postStatus"><el-radio :value="1">활성</el-radio><el-radio :value="2">비활성</el-radio></el-radio-group></el-form-item>
        <el-form-item label="비고"><el-input v-model="form.remark" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">취소</el-button><el-button type="primary" @click="submit">저장</el-button></template>
    </el-dialog>
  </div>
</template>
