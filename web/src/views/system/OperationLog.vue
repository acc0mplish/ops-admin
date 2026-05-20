<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchDeleteSysOperationLog, cleanSysOperationLog, deleteSysOperationLog, querySysOperationLogList } from '../../api/system'

const loading = ref(false)
const selectedIds = ref([])
const tableData = ref([])
const total = ref(0)
const query = reactive({
  pageNum: 1,
  pageSize: 10,
  username: ''
})

async function loadData() {
  loading.value = true
  try {
    const data = await querySysOperationLogList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function onSelectionChange(rows) {
  selectedIds.value = rows.map((item) => item.id)
}

async function handleDelete(id) {
  await deleteSysOperationLog(id)
  ElMessage.success('删除成功')
  await loadData()
}

async function handleBatchDelete() {
  if (!selectedIds.value.length) return
  await batchDeleteSysOperationLog(selectedIds.value)
  ElMessage.success('批量删除成功')
  await loadData()
}

async function handleClean() {
  await ElMessageBox.confirm('确认清空操作日志吗？', '提示', { type: 'warning' })
  await cleanSysOperationLog()
  ElMessage.success('已清空')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card">
    <h2 class="page-title">操作日志</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.username" placeholder="按账号搜索" clearable style="width:220px" />
        <el-button type="primary" @click="loadData">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-button v-permission="'system:operationlog:delete'" :disabled="!selectedIds.length" @click="handleBatchDelete">批量删除</el-button>
        <el-button v-permission="'system:operationlog:clean'" type="danger" plain @click="handleClean">清空日志</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border @selection-change="onSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="username" label="账号" min-width="120" />
      <el-table-column prop="method" label="方法" width="90" />
      <el-table-column prop="ip" label="IP" min-width="120" />
      <el-table-column prop="description" label="描述" min-width="180" />
      <el-table-column prop="url" label="URL" min-width="220" />
      <el-table-column prop="createTime" label="操作时间" min-width="180" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button v-permission="'system:operationlog:delete'" link type="danger" @click="handleDelete(row.id)">删除</el-button>
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
  </div>
</template>
