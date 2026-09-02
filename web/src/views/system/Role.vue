<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addRole, assignPermissions, deleteRole, queryMenuList, queryRoleList, queryRoleMenuIdList, roleInfo, roleUpdate, updateRoleStatus } from '../../api/system'
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

function resetForm() { Object.assign(form, { id: undefined, roleName: '', roleKey: '', status: 1, description: '' }) }
async function loadData() { loading.value = true; try { const data = await queryRoleList(); tableData.value = data.list || [] } finally { loading.value = false } }
async function loadMenus() { const data = await queryMenuList(); menuTree.value = buildTree(data || []) }
function openCreate() { isEdit.value = false; resetForm(); dialogVisible.value = true }
async function openEdit(row) { isEdit.value = true; Object.assign(form, await roleInfo(row.id)); dialogVisible.value = true }

async function submit() {
  if (isEdit.value) { await roleUpdate(form); ElMessage.success('Role 정보를 업데이트했습니다.') }
  else { await addRole(form); ElMessage.success('Role을 생성했습니다.') }
  dialogVisible.value = false
  await loadData()
}
async function handleDelete(row) { await ElMessageBox.confirm(`Role ${row.roleName}을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' }); await deleteRole(row.id); ElMessage.success('Role을 삭제했습니다.'); await loadData() }
async function handleStatus(row) { await updateRoleStatus(row.id, row.status === 1 ? 2 : 1); ElMessage.success('Role 상태를 변경했습니다.'); await loadData() }
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
  ElMessage.success('권한을 할당했습니다.')
  permissionVisible.value = false
}
onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">Role 관리</h2>
    <div class="toolbar"><div class="toolbar-left"></div><div class="toolbar-right"><el-button v-permission="'system:role:add'" type="primary" @click="openCreate">Role 추가</el-button></div></div>
    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="roleName" label="Role 이름" min-width="160" />
      <el-table-column prop="roleKey" label="Role Key" min-width="160" />
      <el-table-column prop="description" label="설명" min-width="220" />
      <el-table-column label="상태" width="100"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '활성' : '비활성' }}</el-tag></template></el-table-column>
      <el-table-column label="작업" width="280" fixed="right"><template #default="{ row }">
        <el-button v-permission="'system:role:edit'" link type="primary" @click="openEdit(row)">수정</el-button>
        <el-button v-permission="'system:role:assign'" link type="success" @click="openPermission(row)">권한 할당</el-button>
        <el-button v-permission="'system:role:status'" link type="warning" @click="handleStatus(row)">{{ row.status === 1 ? '비활성화' : '활성화' }}</el-button>
        <el-button v-permission="'system:role:delete'" link type="danger" @click="handleDelete(row)">삭제</el-button>
      </template></el-table-column>
    </el-table>
    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Role 수정' : 'Role 추가'" width="600px">
      <el-form label-width="90px">
        <el-form-item label="Role 이름"><el-input v-model="form.roleName" /></el-form-item>
        <el-form-item label="Role Key"><el-input v-model="form.roleKey" /></el-form-item>
        <el-form-item label="상태"><el-radio-group v-model="form.status"><el-radio :value="1">활성</el-radio><el-radio :value="2">비활성</el-radio></el-radio-group></el-form-item>
        <el-form-item label="설명"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">취소</el-button><el-button type="primary" @click="submit">저장</el-button></template>
    </el-dialog>
    <el-dialog v-model="permissionVisible" title="메뉴 권한 할당" width="520px">
      <el-tree ref="treeRef" :data="menuTree" show-checkbox node-key="id" default-expand-all :props="{ label: 'menuName', children: 'children' }" />
      <template #footer><el-button @click="permissionVisible = false">취소</el-button><el-button type="primary" @click="savePermission">저장</el-button></template>
    </el-dialog>
  </div>
</template>
