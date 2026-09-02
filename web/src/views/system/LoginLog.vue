<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchDeleteSysLoginInfo, cleanSysLoginInfo, deleteSysLoginInfo, querySysLoginInfoList } from '../../api/system'
import { st } from '../../utils/system-i18n'

const loading = ref(false)
const selectedIds = ref([])
const tableData = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, username: '' })
async function loadData() { loading.value = true; try { const data = await querySysLoginInfoList(query); tableData.value = data.list || []; total.value = data.total || 0 } finally { loading.value = false } }
function onSelectionChange(rows) { selectedIds.value = rows.map((item) => item.id) }
async function handleDelete(id) { await deleteSysLoginInfo(id); ElMessage.success(st('deletedSuccess')); await loadData() }
async function handleBatchDelete() { if (!selectedIds.value.length) return; await batchDeleteSysLoginInfo(selectedIds.value); ElMessage.success(st('deletedSuccess')); await loadData() }
async function handleClean() { await ElMessageBox.confirm(st('clearLogsConfirm'), st('deleteConfirm'), { type: 'warning' }); await cleanSysLoginInfo(); ElMessage.success(st('deletedSuccess')); await loadData() }
onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('loginLog') }}</h2>
    <div class="toolbar">
      <div class="toolbar-left"><el-input v-model="query.username" :placeholder="st('searchUsername')" clearable style="width:220px" /><el-button type="primary" @click="loadData">{{ st('query') }}</el-button></div>
      <div class="toolbar-right"><el-button v-permission="'system:loginlog:delete'" :disabled="!selectedIds.length" @click="handleBatchDelete">{{ st('delete') }}</el-button><el-button v-permission="'system:loginlog:clean'" type="danger" plain @click="handleClean">{{ st('clear') }}</el-button></div>
    </div>
    <el-table v-loading="loading" :data="tableData" border @selection-change="onSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="username" :label="st('account')" min-width="120" />
      <el-table-column prop="ipAddress" :label="st('ipAddress')" min-width="120" />
      <el-table-column prop="browser" label="Browser" min-width="140" />
      <el-table-column prop="os" label="OS" min-width="140" />
      <el-table-column prop="message" :label="st('result')" min-width="160" />
      <el-table-column :label="st('status')" width="100"><template #default="{ row }"><el-tag :type="row.loginStatus === 1 ? 'success' : 'danger'">{{ row.loginStatus === 1 ? st('success') : st('failure') }}</el-tag></template></el-table-column>
      <el-table-column prop="loginTime" :label="st('loginTime')" min-width="180" />
      <el-table-column :label="st('actions')" width="100"><template #default="{ row }"><el-button v-permission="'system:loginlog:delete'" link type="danger" @click="handleDelete(row.id)">{{ st('delete') }}</el-button></template></el-table-column>
    </el-table>
    <div style="margin-top:16px;display:flex;justify-content:flex-end;"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, prev, pager, next, sizes" @current-change="loadData" @size-change="loadData" /></div>
  </div>
</template>
