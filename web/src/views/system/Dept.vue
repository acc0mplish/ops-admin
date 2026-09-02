<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addDept, deleteDept, deptInfo, deptUpdate, queryDeptList } from '../../api/system'
import { buildTree } from '../../utils/tree'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const deptOptions = ref([])

const form = reactive({ id: undefined, parentId: 0, deptType: 3, deptName: '', deptStatus: 1 })

function resetForm() {
  Object.assign(form, { id: undefined, parentId: 0, deptType: 3, deptName: '', deptStatus: 1 })
}

async function loadData() {
  loading.value = true
  try {
    const list = await queryDeptList()
    tableData.value = buildTree(list || [])
    deptOptions.value = list || []
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
  Object.assign(form, await deptInfo(row.id))
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await deptUpdate(form)
    ElMessage.success('부서 정보를 업데이트했습니다.')
  } else {
    await addDept(form)
    ElMessage.success('부서를 생성했습니다.')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`부서 ${row.deptName}을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' })
  await deleteDept(row.id)
  ElMessage.success('부서를 삭제했습니다.')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">부서 관리</h2>
    <div class="toolbar">
      <div class="toolbar-left"></div>
      <div class="toolbar-right"><el-button v-permission="'system:dept:add'" type="primary" @click="openCreate">부서 추가</el-button></div>
    </div>

    <el-table v-loading="loading" :data="tableData" row-key="id" border default-expand-all>
      <el-table-column prop="deptName" label="부서명" min-width="200" />
      <el-table-column prop="deptType" label="부서 유형" width="120" />
      <el-table-column label="상태" width="100">
        <template #default="{ row }"><el-tag :type="row.deptStatus === 1 ? 'success' : 'danger'">{{ row.deptStatus === 1 ? '활성' : '비활성' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="작업" width="180">
        <template #default="{ row }">
          <el-button v-permission="'system:dept:edit'" link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button v-permission="'system:dept:delete'" link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '부서 수정' : '부서 추가'" width="560px">
      <el-form label-width="90px">
        <el-form-item label="상위 부서">
          <el-select v-model="form.parentId" style="width:100%">
            <el-option :value="0" label="최상위 부서" />
            <el-option v-for="item in deptOptions" :key="item.id" :label="item.deptName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="부서명"><el-input v-model="form.deptName" /></el-form-item>
        <el-form-item label="부서 유형">
          <el-radio-group v-model="form.deptType"><el-radio :value="1">회사</el-radio><el-radio :value="2">센터</el-radio><el-radio :value="3">부서</el-radio></el-radio-group>
        </el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="form.deptStatus"><el-radio :value="1">활성</el-radio><el-radio :value="2">비활성</el-radio></el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">취소</el-button><el-button type="primary" @click="submit">저장</el-button></template>
    </el-dialog>
  </div>
</template>
