<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addRole, assignPermissions, deleteRole, queryMenuList, queryRoleList, queryRoleMenuIdList, roleInfo, roleUpdate, updateRoleStatus } from '../../api/system'
import { translateEntity, translateRoute } from '../../utils/i18n-runtime'
import { st } from '../../utils/system-i18n'
import { buildTree } from '../../utils/tree'

const loading = ref(false)
const dialogVisible = ref(false)
const permissionVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const menuTree = ref([])
const roleIdForPermission = ref()
const treeRef = ref()
const form = reactive({ id: undefined, roleName: '', roleKey: '', status: 1, description: '' })
const treeProps = { children: 'children', label: (data) => translateRoute(data.url || '', data.menuName || '') }

function resetForm() { Object.assign(form, { id: undefined, roleName: '', roleKey: '', status: 1, description: '' }) }
async function loadData() { loading.value = true; try { const data = await queryRoleList(); tableData.value = data.list || [] } finally { loading.value = false } }
async function loadMenus() { const data = await queryMenuList(); menuTree.value = buildTree(data || []) }
function openCreate() { isEdit.value = false; resetForm(); dialogVisible.value = true }
async function openEdit(row) { isEdit.value = true; Object.assign(form, await roleInfo(row.id)); dialogVisible.value = true }

async function submit() {
  if (isEdit.value) { await roleUpdate(form); ElMessage.success(st('updatedSuccess')) }
  else { await addRole(form); ElMessage.success(st('createdSuccess')) }
  dialogVisible.value = false
  await loadData()
}
async function handleDelete(row) { await ElMessageBox.confirm(st('confirmDelete', { name: row.roleName }), st('deleteConfirm'), { type: 'warning' }); await deleteRole(row.id); ElMessage.success(st('deletedSuccess')); await loadData() }
async function handleStatus(row) { await updateRoleStatus(row.id, row.status === 1 ? 2 : 1); ElMessage.success(st('statusUpdated')); await loadData() }
async function openPermission(row) {
  roleIdForPermission.value = row.id
  if (!menuTree.value.length) await loadMenus()
  permissionVisible.value = true
  const data = await queryRoleMenuIdList(row.id)
  treeRef.value?.setCheckedKeys((data || []).map((item) => item.id))
}
async function savePermission() {
  const checkedKeys = treeRef.value?.getCheckedKeys() || []
  const halfKeys = treeRef.value?.getHalfCheckedKeys() || []
  await assignPermissions(roleIdForPermission.value, [...new Set([...checkedKeys, ...halfKeys])])
  ElMessage.success(st('updatedSuccess'))
  permissionVisible.value = false
}
onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('roleManagement') }}</h2>
    <div class="toolbar"><div class="toolbar-left"></div><div class="toolbar-right"><el-button v-permission="'system:role:add'" type="primary" @click="openCreate">{{ st('addRole') }}</el-button></div></div>
    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="roleName" :label="st('roleName')" min-width="160"><template #default="{ row }">{{ translateEntity(row.roleName, row.roleName || '-') }}</template></el-table-column>
      <el-table-column prop="roleKey" :label="st('roleKey')" min-width="160" />
      <el-table-column prop="description" :label="st('description')" min-width="220" />
      <el-table-column :label="st('status')" width="100"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? st('active') : st('inactive') }}</el-tag></template></el-table-column>
      <el-table-column :label="st('actions')" width="280" fixed="right"><template #default="{ row }">
        <el-button v-permission="'system:role:edit'" link type="primary" @click="openEdit(row)">{{ st('edit') }}</el-button>
        <el-button v-permission="'system:role:assign'" link type="success" @click="openPermission(row)">{{ st('assignPermissions') }}</el-button>
        <el-button v-permission="'system:role:status'" link type="warning" @click="handleStatus(row)">{{ row.status === 1 ? st('deactivate') : st('activate') }}</el-button>
        <el-button v-permission="'system:role:delete'" link type="danger" @click="handleDelete(row)">{{ st('delete') }}</el-button>
      </template></el-table-column>
    </el-table>
    <el-dialog v-model="dialogVisible" :title="isEdit ? st('editRole') : st('addRole')" width="600px">
      <el-form label-width="90px">
        <el-form-item :label="st('roleName')"><el-input v-model="form.roleName" /></el-form-item>
        <el-form-item :label="st('roleKey')"><el-input v-model="form.roleKey" /></el-form-item>
        <el-form-item :label="st('status')"><el-radio-group v-model="form.status"><el-radio :value="1">{{ st('active') }}</el-radio><el-radio :value="2">{{ st('inactive') }}</el-radio></el-radio-group></el-form-item>
        <el-form-item :label="st('description')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" @click="submit">{{ st('save') }}</el-button></template>
    </el-dialog>
    <el-dialog v-model="permissionVisible" :title="st('assignPermissions')" width="520px">
      <el-tree ref="treeRef" :data="menuTree" show-checkbox node-key="id" default-expand-all :props="treeProps" />
      <template #footer><el-button @click="permissionVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" @click="savePermission">{{ st('save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
