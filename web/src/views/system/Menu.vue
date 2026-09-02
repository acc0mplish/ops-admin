<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addMenu, deleteMenu, menuInfo, menuUpdate, queryMenuList, querySysMenuVoList } from '../../api/system'
import { translateRoute } from '../../utils/i18n-runtime'
import { st } from '../../utils/system-i18n'
import { buildTree } from '../../utils/tree'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const menuOptions = ref([])
const form = reactive({ id: undefined, parentId: 0, menuName: '', icon: '', value: '', menuType: 2, url: '', menuStatus: 1, sort: 0 })
function resetForm() { Object.assign(form, { id: undefined, parentId: 0, menuName: '', icon: '', value: '', menuType: 2, url: '', menuStatus: 1, sort: 0 }) }
async function loadData() { loading.value = true; try { const [list, options] = await Promise.all([queryMenuList(), querySysMenuVoList()]); tableData.value = buildTree(list || []); menuOptions.value = options || [] } finally { loading.value = false } }
function openCreate() { isEdit.value = false; resetForm(); dialogVisible.value = true }
async function openEdit(row) { isEdit.value = true; Object.assign(form, await menuInfo(row.id)); dialogVisible.value = true }
async function submit() { if (isEdit.value) { await menuUpdate(form); ElMessage.success(st('updatedSuccess')) } else { await addMenu(form); ElMessage.success(st('createdSuccess')) }; dialogVisible.value = false; await loadData() }
async function handleDelete(row) { await ElMessageBox.confirm(st('confirmDelete', { name: row.menuName }), st('deleteConfirm'), { type: 'warning' }); await deleteMenu(row.id); ElMessage.success(st('deletedSuccess')); await loadData() }
function menuTypeLabel(type) { if (type === 1) return st('directory'); if (type === 3) return st('permission'); return st('page') }
onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('menuManagement') }}</h2>
    <div class="toolbar"><div class="toolbar-left"></div><div class="toolbar-right"><el-button v-permission="'system:menu:add'" type="primary" @click="openCreate">{{ st('addMenu') }}</el-button></div></div>
    <el-table v-loading="loading" :data="tableData" row-key="id" border default-expand-all>
      <el-table-column prop="menuName" :label="st('menuName')" min-width="180"><template #default="{ row }">{{ translateRoute(row.url || '', row.menuName) }}</template></el-table-column>
      <el-table-column prop="url" :label="st('route')" min-width="180" />
      <el-table-column prop="value" :label="st('permissionKey')" min-width="180" />
      <el-table-column prop="menuType" :label="st('menuType')" width="100"><template #default="{ row }">{{ menuTypeLabel(row.menuType) }}</template></el-table-column>
      <el-table-column prop="sort" :label="st('sort')" width="90" />
      <el-table-column :label="st('status')" width="100"><template #default="{ row }"><el-tag :type="row.menuStatus === 1 ? 'success' : 'danger'">{{ row.menuStatus === 1 ? st('active') : st('inactive') }}</el-tag></template></el-table-column>
      <el-table-column :label="st('actions')" width="180" fixed="right"><template #default="{ row }"><el-button v-permission="'system:menu:edit'" link type="primary" @click="openEdit(row)">{{ st('edit') }}</el-button><el-button v-permission="'system:menu:delete'" link type="danger" @click="handleDelete(row)">{{ st('delete') }}</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialogVisible" :title="isEdit ? st('editMenu') : st('addMenu')" width="620px">
      <el-form label-width="110px">
        <el-form-item :label="st('parentMenu')"><el-select v-model="form.parentId" style="width:100%"><el-option :value="0" :label="st('rootMenu')" /><el-option v-for="item in menuOptions" :key="item.id" :label="translateRoute(item.url || '', item.label)" :value="item.id" /></el-select></el-form-item>
        <el-form-item :label="st('menuName')"><el-input v-model="form.menuName" /></el-form-item>
        <el-form-item :label="st('route')"><el-input v-model="form.url" /></el-form-item>
        <el-form-item :label="st('permissionKey')"><el-input v-model="form.value" /></el-form-item>
        <el-form-item :label="st('icon')"><el-input v-model="form.icon" /></el-form-item>
        <el-form-item :label="st('menuType')"><el-radio-group v-model="form.menuType"><el-radio :value="1">{{ st('directory') }}</el-radio><el-radio :value="2">{{ st('page') }}</el-radio><el-radio :value="3">{{ st('permission') }}</el-radio></el-radio-group></el-form-item>
        <el-form-item :label="st('status')"><el-radio-group v-model="form.menuStatus"><el-radio :value="1">{{ st('active') }}</el-radio><el-radio :value="2">{{ st('inactive') }}</el-radio></el-radio-group></el-form-item>
        <el-form-item :label="st('sort')"><el-input-number v-model="form.sort" :min="0" style="width:100%" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ st('cancel') }}</el-button><el-button type="primary" @click="submit">{{ st('save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
