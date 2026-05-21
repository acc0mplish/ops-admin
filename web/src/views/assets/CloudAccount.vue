<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addAssetCloudAccount,
  assetCloudAccountInfo,
  deleteAssetCloudAccount,
  queryAssetCloudAccountList,
  updateAssetCloudAccount
} from '../../api/asset'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', provider: '' })
const form = reactive({ id: undefined, name: '', provider: 'aliyun', accessKey: '', secretKey: '', region: '', status: 1, description: '' })

function resetForm() {
  Object.assign(form, { id: undefined, name: '', provider: 'aliyun', accessKey: '', secretKey: '', region: '', status: 1, description: '' })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetCloudAccountList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
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
  const data = await assetCloudAccountInfo(row.id)
  Object.assign(form, data, { secretKey: '' })
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await updateAssetCloudAccount(form)
    ElMessage.success('云账号已更新')
  } else {
    await addAssetCloudAccount(form)
    ElMessage.success('云账号已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除云账号 ${row.name} 吗？`, '提示', { type: 'warning' })
  await deleteAssetCloudAccount(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card">
    <h2 class="page-title">云账号管理</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="搜索名称 / AccessKey" style="width: 240px" @keyup.enter="loadData" />
        <el-select v-model="query.provider" clearable placeholder="云厂商" style="width: 140px">
          <el-option label="阿里云" value="aliyun" />
          <el-option label="腾讯云" value="tencent" />
          <el-option label="华为云" value="huawei" />
          <el-option label="百度云" value="baidu" />
          <el-option label="AWS" value="aws" />
        </el-select>
        <el-button @click="loadData">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreate">新增云账号</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="账号名称" min-width="180" />
      <el-table-column prop="provider" label="云厂商" width="120" />
      <el-table-column prop="accessKey" label="AccessKey" min-width="220" />
      <el-table-column prop="region" label="默认区域" min-width="140" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '正常' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="备注" min-width="220" />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        layout="total, sizes, prev, pager, next"
        :total="total"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑云账号' : '新增云账号'" width="620px">
      <el-form label-width="96px">
        <el-form-item label="账号名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="云厂商">
          <el-select v-model="form.provider" style="width: 100%">
            <el-option label="阿里云" value="aliyun" />
            <el-option label="腾讯云" value="tencent" />
            <el-option label="华为云" value="huawei" />
            <el-option label="百度云" value="baidu" />
            <el-option label="AWS" value="aws" />
          </el-select>
        </el-form-item>
        <el-form-item label="AccessKey"><el-input v-model="form.accessKey" /></el-form-item>
        <el-form-item label="SecretKey"><el-input v-model="form.secretKey" show-password :placeholder="isEdit ? '不填写则保持不变' : ''" /></el-form-item>
        <el-form-item label="默认区域"><el-input v-model="form.region" placeholder="例如 cn-hangzhou" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">正常</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
