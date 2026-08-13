<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addPost, deletePost, queryPostList, postInfo, updatePost, updatePostStatus } from '../../api/system'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])

const form = reactive({
  id: undefined,
  postCode: '',
  postName: '',
  postStatus: 1,
  remark: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    postCode: '',
    postName: '',
    postStatus: 1,
    remark: ''
  })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryPostList()
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
  const data = await postInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await updatePost(form)
    ElMessage.success('岗位已更新')
  } else {
    await addPost(form)
    ElMessage.success('岗位已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除岗位 ${row.postName} 吗？`, '提示', { type: 'warning' })
  await deletePost(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

async function handleStatus(row) {
  await updatePostStatus(row.id, row.postStatus === 1 ? 2 : 1)
  ElMessage.success('状态已更新')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">岗位管理</h2>
    <div class="toolbar">
      <div class="toolbar-left"></div>
      <div class="toolbar-right">
        <el-button v-permission="'system:post:add'" type="primary" @click="openCreate">新增岗位</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="postCode" label="岗位编码" min-width="160" />
      <el-table-column prop="postName" label="岗位名称" min-width="160" />
      <el-table-column prop="remark" label="备注" min-width="220" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.postStatus === 1 ? 'success' : 'danger'">{{ row.postStatus === 1 ? '正常' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button v-permission="'system:post:edit'" link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button v-permission="'system:post:status'" link type="warning" @click="handleStatus(row)">{{ row.postStatus === 1 ? '停用' : '启用' }}</el-button>
          <el-button v-permission="'system:post:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑岗位' : '新增岗位'" width="560px">
      <el-form label-width="90px">
        <el-form-item label="岗位编码"><el-input v-model="form.postCode" /></el-form-item>
        <el-form-item label="岗位名称"><el-input v-model="form.postName" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.postStatus">
            <el-radio :value="1">正常</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
