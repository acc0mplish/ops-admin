<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addAssetHostGroup,
  assetHostGroupInfo,
  deleteAssetHostGroup,
  queryAssetHostGroupList,
  updateAssetHostGroup
} from '../../api/asset'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const query = reactive({ keyword: '' })
const form = reactive({ id: undefined, parentId: 0, name: '', code: '', sort: 0, status: 1, description: '' })

function resetForm() {
  Object.assign(form, { id: undefined, parentId: 0, name: '', code: '', sort: 0, status: 1, description: '' })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetHostGroupList(query)
    tableData.value = data.list || []
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
  const data = await assetHostGroupInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await updateAssetHostGroup(form)
    ElMessage.success('主机组已更新')
  } else {
    await addAssetHostGroup(form)
    ElMessage.success('主机组已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除主机组 ${row.name} 吗？`, '提示', { type: 'warning' })
  await deleteAssetHostGroup(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card">
    <h2 class="page-title">主机组管理</h2>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="搜索名称 / 编码" style="width: 240px" @keyup.enter="loadData" />
        <el-button @click="loadData">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreate">新增主机组</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="主机组名称" min-width="180" />
      <el-table-column prop="code" label="编码" min-width="140" />
      <el-table-column prop="sort" label="排序" width="90" />
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑主机组' : '新增主机组'" width="560px">
      <el-form label-width="96px">
        <el-form-item label="上级分组">
          <el-select v-model="form.parentId" filterable style="width: 100%">
            <el-option :value="0" label="根分组" />
            <el-option v-for="item in tableData" :key="item.id" :value="item.id" :label="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="编码"><el-input v-model="form.code" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
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
