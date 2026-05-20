<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchDeleteSysLoginInfo, cleanSysLoginInfo, deleteSysLoginInfo, querySysLoginInfoList } from '../../api/system'

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
    const data = await querySysLoginInfoList(query)
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
  await deleteSysLoginInfo(id)
  ElMessage.success('删除成功')
  await loadData()
}

async function handleBatchDelete() {
  if (!selectedIds.value.length) return
  await batchDeleteSysLoginInfo(selectedIds.value)
  ElMessage.success('批量删除成功')
  await loadData()
}

async function handleClean() {
  await ElMessageBox.confirm('确认清空登录日志吗？', '提示', { type: 'warning' })
  await cleanSysLoginInfo()
  ElMessage.success('已清空')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card">
    <h2 class="page-title">登录日志</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.username" placeholder="按账号搜索" clearable style="width:220px" />
        <el-button type="primary" @click="loadData">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-button v-permission="'system:loginlog:delete'" :disabled="!selectedIds.length" @click="handleBatchDelete">批量删除</el-button>
        <el-button v-permission="'system:loginlog:clean'" type="danger" plain @click="handleClean">清空日志</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border @selection-change="onSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="username" label="账号" min-width="120" />
      <el-table-column prop="ipAddress" label="IP" min-width="120" />
      <el-table-column prop="browser" label="浏览器" min-width="140" />
      <el-table-column prop="os" label="系统" min-width="140" />
      <el-table-column prop="message" label="结果" min-width="160" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.loginStatus === 1 ? 'success' : 'danger'">{{ row.loginStatus === 1 ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="loginTime" label="登录时间" min-width="180" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button v-permission="'system:loginlog:delete'" link type="danger" @click="handleDelete(row.id)">删除</el-button>
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
