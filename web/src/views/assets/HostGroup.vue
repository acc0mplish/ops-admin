<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
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
const treeData = ref([])
const query = reactive({ keyword: '' })
const form = reactive({
  id: undefined,
  parentId: 0,
  name: '',
  code: '',
  sort: 0,
  status: 1,
  description: '',
  hostCount: 0
})

const treeProps = {
  label: 'name',
  value: 'id',
  children: 'children'
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    parentId: 0,
    name: '',
    code: '',
    sort: 0,
    status: 1,
    description: '',
    hostCount: 0
  })
}

function cloneTree(nodes = []) {
  return nodes.map((item) => ({
    ...item,
    children: cloneTree(item.children || [])
  }))
}

function filterTree(nodes = [], excludeId) {
  return nodes
    .filter((item) => item.id !== excludeId)
    .map((item) => ({
      ...item,
      children: filterTree(item.children || [], excludeId)
    }))
}

const parentTreeOptions = computed(() => [
  {
    id: 0,
    name: '根分组',
    children: form.id ? filterTree(cloneTree(treeData.value), form.id) : cloneTree(treeData.value)
  }
])

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetHostGroupList(query)
    treeData.value = data.tree || []
  } finally {
    loading.value = false
  }
}

function openCreate(parentId = 0) {
  isEdit.value = false
  resetForm()
  form.parentId = parentId
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await assetHostGroupInfo(row.id)
  Object.assign(form, {
    id: data.id,
    parentId: data.parentId ?? 0,
    name: data.name || '',
    code: data.code || '',
    sort: data.sort || 0,
    status: data.status || 1,
    description: data.description || '',
    hostCount: data.hostCount || 0
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入主机组名称')
    return
  }
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
  await ElMessageBox.confirm(`确认删除主机组「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteAssetHostGroup(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function resetQuery() {
  query.keyword = ''
  loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card">
    <div class="page-header">
      <div>
        <h2 class="page-title">主机组管理</h2>
        <p class="page-desc">按树形结构维护主机组，查看每个分组下的主机关联数量。</p>
      </div>
      <el-button type="primary" @click="openCreate()">新增主机组</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="搜索主机组名称 / 编码"
          style="width: 260px"
          @keyup.enter="loadData"
        />
        <el-button type="primary" @click="loadData">搜索</el-button>
        <el-button @click="resetQuery">重置</el-button>
      </div>
    </div>

    <el-table
      v-loading="loading"
      :data="treeData"
      row-key="id"
      border
      default-expand-all
      :tree-props="{ children: 'children' }"
    >
      <el-table-column prop="name" label="主机组名称" min-width="220">
        <template #default="{ row }">
          <div class="group-name-cell">
            <span class="group-dot" />
            <span>{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="code" label="编码" min-width="150" />
      <el-table-column prop="hostCount" label="关联主机数" width="120" align="center">
        <template #default="{ row }">
          <el-tag type="info">{{ row.hostCount || 0 }} 台</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="90" align="center" />
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">
            {{ row.status === 1 ? '正常' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="备注" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updateTime" label="更新时间" min-width="170" />
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openCreate(row.id)">新增子组</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑主机组' : '新增主机组'" width="620px">
      <el-form label-width="108px">
        <el-form-item label="上级主机组">
          <el-tree-select
            v-model="form.parentId"
            :data="parentTreeOptions"
            :props="treeProps"
            check-strictly
            default-expand-all
            node-key="id"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item v-if="isEdit" label="关联主机数">
          <el-tag type="info">{{ form.hostCount || 0 }} 台主机</el-tag>
        </el-form-item>
        <el-form-item label="主机组名称" required>
          <el-input v-model="form.name" placeholder="请输入主机组名称" />
        </el-form-item>
        <el-form-item label="编码">
          <el-input v-model="form.code" placeholder="请输入主机组编码" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" style="width: 180px" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">正常</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" type="textarea" :rows="4" placeholder="请输入备注信息" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  color: #18213a;
}

.page-desc {
  margin: 8px 0 0;
  color: #6a7592;
  font-size: 14px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 18px;
}

.toolbar-left {
  display: flex;
  gap: 12px;
  align-items: center;
}

.group-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 600;
  color: #24314f;
}

.group-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: linear-gradient(135deg, #6c63ff 0%, #5aa7ff 100%);
  box-shadow: 0 0 0 4px rgba(108, 99, 255, 0.12);
}
</style>
