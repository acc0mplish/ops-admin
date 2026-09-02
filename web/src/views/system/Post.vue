<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addPost, deletePost, queryPostList, postInfo, updatePost, updatePostStatus } from '../../api/system'
import { translateEntity } from '../../utils/i18n-runtime'
import { st } from '../../utils/system-i18n'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const form = reactive({ id: undefined, postCode: '', postName: '', postStatus: 1, remark: '' })

function resetForm() { Object.assign(form, { id: undefined, postCode: '', postName: '', postStatus: 1, remark: '' }) }

async function loadData() {
  loading.value = true
  try { const data = await queryPostList(); tableData.value = data.list || [] } finally { loading.value = false }
}

function openCreate() { isEdit.value = false; resetForm(); dialogVisible.value = true }
async function openEdit(row) { isEdit.value = true; Object.assign(form, await postInfo(row.id)); dialogVisible.value = true }

async function submit() {
  if (isEdit.value) { await updatePost(form); ElMessage.success(st('updatedSuccess')) }
  else { await addPost(form); ElMessage.success(st('createdSuccess')) }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(st('confirmDelete', { name: row.postName }), st('deleteConfirm'), { type: 'warning' })
  await deletePost(row.id)
  ElMessage.success(st('deletedSuccess'))
  await loadData()
}

async function handleStatus(row) {
  await updatePostStatus(row.id, row.postStatus === 1 ? 2 : 1)
  ElMessage.success(st('statusUpdated'))
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('positionManagement') }}</h2>
    <div class="toolbar"><div class="toolbar-left"></div><div class="toolbar-right"><el-button v-permission="'system:post:add'" type="primary" @click="openCreate">{{ st('addPosition') }}</el-button></div></div>
    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="postCode" :label="st('positionCode')" min-width="160" />
      <el-table-column prop="postName" :label="st('positionName')" min-width="160"><template #default="{ row }">{{ translateEntity(row.postName, row.postName || '-') }}</template></el-table-column>
      <el-table-column prop="remark" :label="st('remark')" min-width="220" />
      <el-table-column :label="st('status')" width="100"><template #default="{ row }"><el-tag :type="row.postStatus === 1 ? 'success' : 'danger'">{{ row.postStatus === 1 ? st('active') : st('inactive') }}</el-tag></template></el-table-column>
      <el-table-column :label="st('actions')" width="200"><template #default="{ row }"><el-button v-permission="'system:post:edit'" link type="primary" @click="openEdit(row)">{{ st('edit') }}</el-button><el-button v-permission="'system:post:status'" link type="warning" @click="handleStatus(row)">{{ row.postStatus === 1 ? st('deactivate') : st('activate') }}</el-button><el-button v-permission="'system:post:delete'" link type="danger" @click="handleDelete(row)">{{ st('delete') }}</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialogVisible" :title="isEdit ? st('editPosition') : st('addPosition')" width="560px">
      <el-form label-width="90px">
        <el-form-item :label="st('positionCode')"><el-input v-model="form.postCode" /></el-form-item>
        <el-form-item :label="st('positionName')"><el-input v-model="form.postName" /></el-form-item>
        <el-form-item :label="st('status')"><el-radio-group v-model="form.postStatus"><el-radio :value="1">{{ st('active') }}</el-radio><el-radio :value="2">{{ st('inactive') }}</el-radio></el-radio-group></el-form-item>
        <el-form-item :label="st('remark')"><el-input v-model="form.remark" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" @click="submit">{{ st('save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
