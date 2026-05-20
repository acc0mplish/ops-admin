<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addDept, deleteDept, deptInfo, deptUpdate, queryDeptList } from '../../api/system'
import { buildTree } from '../../utils/tree'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const deptOptions = ref([])

const form = reactive({
  id: undefined,
  parentId: 0,
  deptType: 3,
  deptName: '',
  deptStatus: 1
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    parentId: 0,
    deptType: 3,
    deptName: '',
    deptStatus: 1
  })
}

async function loadData() {
  loading.value = true
  try {
    const list = await queryDeptList()
    tableData.value = buildTree(list || [])
    deptOptions.value = list || []
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
  const data = await deptInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await deptUpdate(form)
    ElMessage.success('部门已更新')
  } else {
    await addDept(form)
    ElMessage.success('部门已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除部门 ${row.deptName} 吗？`, '提示', { type: 'warning' })
  await deleteDept(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card">
    <h2 class="page-title">部门管理</h2>
    <div class="toolbar">
      <div class="toolbar-left"></div>
      <div class="toolbar-right">
        <el-button v-permission="'system:dept:add'" type="primary" @click="openCreate">新增部门</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" row-key="id" border default-expand-all>
      <el-table-column prop="deptName" label="部门名称" min-width="200" />
      <el-table-column prop="deptType" label="部门类型" width="120" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.deptStatus === 1 ? 'success' : 'danger'">{{ row.deptStatus === 1 ? '正常' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button v-permission="'system:dept:edit'" link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button v-permission="'system:dept:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑部门' : '新增部门'" width="560px">
      <el-form label-width="90px">
        <el-form-item label="上级部门">
          <el-select v-model="form.parentId" style="width:100%">
            <el-option :value="0" label="顶级部门" />
            <el-option v-for="item in deptOptions" :key="item.id" :label="item.deptName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="部门名称"><el-input v-model="form.deptName" /></el-form-item>
        <el-form-item label="部门类型">
          <el-radio-group v-model="form.deptType">
            <el-radio :value="1">公司</el-radio>
            <el-radio :value="2">中心</el-radio>
            <el-radio :value="3">部门</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.deptStatus">
            <el-radio :value="1">正常</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
