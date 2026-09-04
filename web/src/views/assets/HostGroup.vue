<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
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
    name: '루트 Group',
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
    ElMessage.warning('Host Group 이름을 입력하십시오.')
    return
  }
  if (isEdit.value) {
    await updateAssetHostGroup(form)
    ElMessage.success('Host Group을 수정했습니다.')
  } else {
    await addAssetHostGroup(form)
    ElMessage.success('Host Group을 생성했습니다.')
  }
  dialogVisible.value = false
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Host Group “${row.name}”을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteAssetHostGroup(row.id)
  ElMessage.success('삭제했습니다.')
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
        <h2 class="page-title">Host Group 관리</h2>
        <p class="page-desc">트리 구조로 Host Group을 관리합니다. Group 이름을 클릭하면 해당 Group의 서버 목록으로 바로 이동합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate()">Host Group 추가</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="Host Group 이름 / Code 검색"
          style="width: 280px"
          @keyup.enter="loadData"
        />
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button @click="resetQuery">초기화</el-button>
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
      <el-table-column prop="name" label="Host Group 이름" min-width="240">
        <template #default="{ row }">
          <div class="group-name-cell">
            <span class="group-dot" />
            <button type="button" class="group-link-button" @click="openGroupHosts(row)">{{ row.name }}</button>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="code" label="Code" min-width="140" />
      <el-table-column label="연관 Host 수" width="120" align="center">
        <template #default="{ row }">
          <el-tag type="info" >{{ row.hostCount || 0 }}대</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="정렬" width="90" align="center" />
      <el-table-column label="상태" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="Number(row.status) === 1 ? 'success' : 'danger'" effect="light">
            {{ Number(row.status) === 1 ? '정상' : '사용 중지' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="비고" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updateTime" label="수정 시간" min-width="170" />
      <el-table-column label="작업" width="260" fixed="right">
        <template #default="{ row }">
          <div class="action-row">
            <el-button link type="primary" @click="openGroupHosts(row)">Host 조회</el-button>
            <el-button link type="primary" @click="openCreate(row.id)">하위 Group 추가</el-button>
            <el-button link type="primary" @click="openEdit(row)">수정</el-button>
            <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Host Group 수정' : 'Host Group 추가'" width="620px">
      <el-form label-width="108px">
        <el-form-item label="상위 Host Group">
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
        <el-form-item v-if="isEdit" label="연관 Host 수">
          <el-tag type="info">Host {{ form.hostCount || 0 }}대</el-tag>
        </el-form-item>
        <el-form-item label="Host Group 이름" required>
          <el-input v-model="form.name" placeholder="Host Group 이름을 입력하십시오." />
        </el-form-item>
        <el-form-item label="Code">
          <el-input v-model="form.code" placeholder="Host Group Code를 입력하십시오." />
        </el-form-item>
        <el-form-item label="정렬">
          <el-input-number v-model="form.sort" :min="0" style="width: 180px" />
        </el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">정상</el-radio>
            <el-radio :value="2">사용 중지</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="비고">
          <el-input v-model="form.description" type="textarea" :rows="4" placeholder="비고를 입력하십시오." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submit">저장</el-button>
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
