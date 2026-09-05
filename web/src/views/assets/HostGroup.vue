<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { at } from '../../utils/asset-i18n'
import {
  addAssetHostGroup,
  assetHostGroupInfo,
  deleteAssetHostGroup,
  queryAssetHostGroupList,
  updateAssetHostGroup
} from '../../api/asset'

const router = useRouter()
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

const parentTreeOptions = computed(() => [
  {
    id: 0,
    name: at('rootGroupLabel'),
    children: form.id ? filterTree(cloneTree(treeData.value), form.id) : cloneTree(treeData.value)
  }
])

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

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetHostGroupList(query)
    treeData.value = data.tree || []
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.keyword = ''
  loadData()
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
    ElMessage.warning(at('enterGroupName'))
    return
  }
  if (isEdit.value) {
    await updateAssetHostGroup(form)
    ElMessage.success(at('groupUpdated'))
  } else {
    await addAssetHostGroup(form)
    ElMessage.success(at('groupCreated'))
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(at('deleteGroupConfirm', { name: row.name }), at('notice'), { type: 'warning' })
  await deleteAssetHostGroup(row.id)
  ElMessage.success(at('rowDeleted'))
  await loadData()
}

function openGroupHosts(row) {
  if (!row?.id) return
  router.push({
    path: '/assets/server/hosts',
    query: {
      groupId: String(row.id),
      groupName: row.name || ''
    }
  })
}

onMounted(loadData)
</script>

<template>
  <div class="page-card host-group-page asset-card-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ at('groupManageTitle') }}</h2>
        <p class="page-desc">{{ at('groupManageDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate()">{{ at('addGroupButton') }}</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="query.keyword"
          clearable
          :placeholder="at('groupSearchPlaceholder')"
          style="width: 280px"
          @keyup.enter="loadData"
        />
        <el-button type="primary" @click="loadData">{{ at('search') }}</el-button>
        <el-button @click="resetQuery">{{ at('reset') }}</el-button>
      </div>
    </div>

    <el-table
      v-loading="loading"
      :data="treeData"
      row-key="id"
      border
      default-expand-all
      :tree-props="{ children: 'children' }"
      class="group-table"
    >
      <el-table-column prop="name" :label="at('groupNameColumn')" min-width="240">
        <template #default="{ row }">
          <div class="group-name-cell">
            <span class="group-dot" />
            <button type="button" class="group-link-button" @click="openGroupHosts(row)">{{ row.name }}</button>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="code" label="Code" min-width="140" />
      <el-table-column :label="at('hostCountColumn')" width="120" align="center">
        <template #default="{ row }">
          <el-tag type="info" >{{ at('hostCountUnit', { count: row.hostCount || 0 }) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="sort" :label="at('sortColumn')" width="90" align="center" />
      <el-table-column :label="at('status')" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="Number(row.status) === 1 ? 'success' : 'danger'" effect="light">
            {{ Number(row.status) === 1 ? at('groupNormal') : at('disabledStatus') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" :label="at('noteLabel')" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updateTime" :label="at('updatedAtColumn')" min-width="170" />
      <el-table-column :label="at('actions')" width="260" fixed="right">
        <template #default="{ row }">
          <div class="action-row">
            <el-button link type="primary" @click="openGroupHosts(row)">{{ at('viewHostsButton') }}</el-button>
            <el-button link type="primary" @click="openCreate(row.id)">{{ at('addChildGroupButton') }}</el-button>
            <el-button link type="primary" @click="openEdit(row)">{{ at('edit') }}</el-button>
            <el-button link type="danger" @click="handleDelete(row)">{{ at('delete') }}</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? at('editGroupTitle') : at('addGroupButton')" width="620px">
      <el-form label-width="108px">
        <el-form-item :label="at('parentGroupLabel')">
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
        <el-form-item v-if="isEdit" :label="at('hostCountColumn')">
          <el-tag type="info">{{ at('hostsCountLabel', { count: form.hostCount || 0 }) }}</el-tag>
        </el-form-item>
        <el-form-item :label="at('groupNameColumn')" required>
          <el-input v-model="form.name" :placeholder="at('enterGroupName')" />
        </el-form-item>
        <el-form-item label="Code">
          <el-input v-model="form.code" :placeholder="at('enterGroupCode')" />
        </el-form-item>
        <el-form-item :label="at('sortColumn')">
          <el-input-number v-model="form.sort" :min="0" style="width: 180px" />
        </el-form-item>
        <el-form-item :label="at('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ at('groupNormal') }}</el-radio>
            <el-radio :value="2">{{ at('disabledStatus') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="at('noteLabel')">
          <el-input v-model="form.description" type="textarea" :rows="4" :placeholder="at('enterNote')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" @click="submit">{{ at('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.host-group-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 650;
  color: #18213a;
}

.page-desc {
  margin: 6px 0 0;
  color: #6a7592;
  font-size: 14px;
}

.toolbar-left {
  display: flex;
  gap: 12px;
  align-items: center;
}

.group-table {
  overflow: hidden;
  border-radius: 10px;
}

.group-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.group-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: linear-gradient(135deg, #6c63ff 0%, #5aa7ff 100%);
  box-shadow: 0 0 0 4px rgba(108, 99, 255, 0.12);
}

.group-link-button {
  padding: 0;
  border: 0;
  background: transparent;
  color: #24314f;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.group-link-button:hover {
  color: #315dce;
}

.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 900px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-left {
    flex-wrap: wrap;
  }
}
</style>
