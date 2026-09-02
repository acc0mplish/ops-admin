<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addDept, deleteDept, deptInfo, deptUpdate, queryDeptList } from '../../api/system'
import { translateEntity } from '../../utils/i18n-runtime'
import { st } from '../../utils/system-i18n'
import { buildTree } from '../../utils/tree'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const deptOptions = ref([])
const form = reactive({ id: undefined, parentId: 0, deptType: 3, deptName: '', deptStatus: 1 })

function resetForm() { Object.assign(form, { id: undefined, parentId: 0, deptType: 3, deptName: '', deptStatus: 1 }) }
async function loadData() { loading.value = true; try { const list = await queryDeptList(); tableData.value = buildTree(list || []); deptOptions.value = list || [] } finally { loading.value = false } }
function openCreate() { isEdit.value = false; resetForm(); dialogVisible.value = true }
async function openEdit(row) { isEdit.value = true; Object.assign(form, await deptInfo(row.id)); dialogVisible.value = true }

async function submit() {
  if (isEdit.value) { await deptUpdate(form); ElMessage.success(st('updatedSuccess')) }
  else { await addDept(form); ElMessage.success(st('createdSuccess')) }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(st('confirmDelete', { name: row.deptName }), st('deleteConfirm'), { type: 'warning' })
  await deleteDept(row.id)
  ElMessage.success(st('deletedSuccess'))
  await loadData()
}

function deptTypeLabel(type) {
  if (type === 1) return st('company')
  if (type === 2) return st('center')
  return st('department')
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('departmentManagement') }}</h2>
    <div class="toolbar"><div class="toolbar-left"></div><div class="toolbar-right"><el-button v-permission="'system:dept:add'" type="primary" @click="openCreate">{{ st('addDepartment') }}</el-button></div></div>
    <el-table v-loading="loading" :data="tableData" row-key="id" border default-expand-all>
      <el-table-column prop="deptName" :label="st('departmentName')" min-width="200"><template #default="{ row }">{{ translateEntity(row.deptName, row.deptName || '-') }}</template></el-table-column>
      <el-table-column prop="deptType" :label="st('departmentType')" width="120"><template #default="{ row }">{{ deptTypeLabel(row.deptType) }}</template></el-table-column>
      <el-table-column :label="st('status')" width="100"><template #default="{ row }"><el-tag :type="row.deptStatus === 1 ? 'success' : 'danger'">{{ row.deptStatus === 1 ? st('active') : st('inactive') }}</el-tag></template></el-table-column>
      <el-table-column :label="st('actions')" width="180"><template #default="{ row }"><el-button v-permission="'system:dept:edit'" link type="primary" @click="openEdit(row)">{{ st('edit') }}</el-button><el-button v-permission="'system:dept:delete'" link type="danger" @click="handleDelete(row)">{{ st('delete') }}</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialogVisible" :title="isEdit ? st('editDepartment') : st('addDepartment')" width="560px">
      <el-form label-width="90px">
        <el-form-item :label="st('parentDepartment')"><el-select v-model="form.parentId" style="width:100%"><el-option :value="0" :label="st('rootDepartment')" /><el-option v-for="item in deptOptions" :key="item.id" :label="translateEntity(item.deptName, item.deptName)" :value="item.id" /></el-select></el-form-item>
        <el-form-item :label="st('departmentName')"><el-input v-model="form.deptName" /></el-form-item>
        <el-form-item :label="st('departmentType')"><el-radio-group v-model="form.deptType"><el-radio :value="1">{{ st('company') }}</el-radio><el-radio :value="2">{{ st('center') }}</el-radio><el-radio :value="3">{{ st('department') }}</el-radio></el-radio-group></el-form-item>
        <el-form-item :label="st('status')"><el-radio-group v-model="form.deptStatus"><el-radio :value="1">{{ st('active') }}</el-radio><el-radio :value="2">{{ st('inactive') }}</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" @click="submit">{{ st('save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
