<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { addMenu, deleteMenu, menuInfo, menuUpdate, queryMenuList, querySysMenuVoList } from '../../api/system'
import { buildTree } from '../../utils/tree'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const menuOptions = ref([])

const form = reactive({
  id: undefined,
  parentId: 0,
  menuName: '',
  icon: '',
  value: '',
  menuType: 2,
  url: '',
  menuStatus: 1,
  sort: 0
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    parentId: 0,
    menuName: '',
    icon: '',
    value: '',
    menuType: 2,
    url: '',
    menuStatus: 1,
    sort: 0
  })
}

async function loadData() {
  loading.value = true
  try {
    const [list, options] = await Promise.all([queryMenuList(), querySysMenuVoList()])
    tableData.value = buildTree(list || [])
    menuOptions.value = options || []
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
  const data = await menuInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function submit() {
  if (isEdit.value) {
    await menuUpdate(form)
    ElMessage.success('菜单已更新')
  } else {
    await addMenu(form)
    ElMessage.success('菜单已创建')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除菜单 ${row.menuName} 吗？`, '提示', { type: 'warning' })
  await deleteMenu(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">菜单管理</h2>
    <div class="toolbar">
      <div class="toolbar-left"></div>
      <div class="toolbar-right">
        <el-button v-permission="'system:menu:add'" type="primary" @click="openCreate">新增菜单</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" row-key="id" border default-expand-all>
      <el-table-column prop="menuName" label="菜单名称" min-width="180" />
      <el-table-column prop="url" label="路由" min-width="180" />
      <el-table-column prop="value" label="权限值" min-width="180" />
      <el-table-column prop="menuType" label="类型" width="100" />
      <el-table-column prop="sort" label="排序" width="90" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.menuStatus === 1 ? 'success' : 'danger'">{{ row.menuStatus === 1 ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button v-permission="'system:menu:edit'" link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button v-permission="'system:menu:delete'" link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑菜单' : '新增菜单'" width="620px">
      <el-form label-width="90px">
        <el-form-item label="上级菜单">
          <el-select v-model="form.parentId" style="width:100%">
            <el-option :value="0" label="顶级菜单" />
            <el-option v-for="item in menuOptions" :key="item.id" :label="item.label" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="菜单名称"><el-input v-model="form.menuName" /></el-form-item>
        <el-form-item label="路由"><el-input v-model="form.url" /></el-form-item>
        <el-form-item label="权限值"><el-input v-model="form.value" /></el-form-item>
        <el-form-item label="图标"><el-input v-model="form.icon" /></el-form-item>
        <el-form-item label="菜单类型">
          <el-radio-group v-model="form.menuType">
            <el-radio :value="1">目录</el-radio>
            <el-radio :value="2">菜单</el-radio>
            <el-radio :value="3">按钮</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.menuStatus">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" style="width:100%" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
