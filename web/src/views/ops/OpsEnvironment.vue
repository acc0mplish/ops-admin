<template>
  <div class="env-page">
    <section class="page-head">
      <div>
        <h1>Environment 모델</h1>
        <p>dev / test / prod 등 Environment를 통합 관리하고 Application, K8s, Database, Monitoring이 동일한 Environment를 중심으로 구성되도록 합니다.</p>
      </div>
      <el-button type="primary" @click="openForm()">새 Environment</el-button>
    </section>

    <section class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="Environment 이름 / Code 검색" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="상태" @change="loadData">
        <el-option label="활성화" value="1" />
        <el-option label="비활성화" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
      <el-button @click="resetQuery">초기화</el-button>
    </section>

    <el-table :data="list" class="data-table">
      <el-table-column prop="name" label="Environment 이름" min-width="160" />
      <el-table-column prop="code" label="Environment Code" width="140" />
      <el-table-column prop="sort" label="정렬" width="100" />
      <el-table-column label="상태" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="설명" min-width="240" show-overflow-tooltip />
      <el-table-column label="작업" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openForm(row)">수정</el-button>
          <el-button link type="danger" @click="remove(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? 'Environment 수정' : '새 Environment'" width="560px">
      <el-form :model="form" label-width="92px">
        <el-form-item label="Environment 이름" required>
          <el-input v-model="form.name" placeholder="예: 테스트 Environment" />
        </el-form-item>
        <el-form-item label="Environment Code" required>
          <el-input v-model="form.code" :disabled="Boolean(form.id)" placeholder="예: test" />
        </el-form-item>
        <el-form-item label="정렬">
          <el-input-number v-model="form.sort" :min="0" />
        </el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">활성화</el-radio>
            <el-radio :value="2">비활성화</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="설명">
          <el-input v-model="form.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submit">저장</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsEnvironment, queryOpsEnvironmentList, saveOpsEnvironment } from '../../api/ops'

const query = reactive({ keyword: '', status: '' })
const list = ref([])
const dialogVisible = ref(false)
const form = reactive({ id: 0, name: '', code: '', sort: 0, status: 1, description: '' })

async function loadData() {
  list.value = await queryOpsEnvironmentList(query)
}

function resetQuery() {
  query.keyword = ''
  query.status = ''
  loadData()
}

function openForm(row) {
  Object.assign(form, row || { id: 0, name: '', code: '', sort: 0, status: 1, description: '' })
  dialogVisible.value = true
}

async function submit() {
  await saveOpsEnvironment(form)
  ElMessage.success('저장했습니다.')
  dialogVisible.value = false
  loadData()
}

async function remove(row) {
  await ElMessageBox.confirm(`Environment ${row.name}을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' })
  await deleteOpsEnvironment(row.id)
  ElMessage.success('삭제했습니다.')
  loadData()
}

onMounted(loadData)
</script>

<style scoped>
.env-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.page-head,
.toolbar,
.data-table {
  background: #fff;
  border: 1px solid #dfe8f6;
  border-radius: 8px;
}
.page-head {
  display: flex;
  justify-content: space-between;
  padding: 24px;
}
.page-head h1 {
  margin: 0;
  color: #071a3d;
}
.page-head p {
  margin: 8px 0 0;
  color: #6d7f9f;
}
.toolbar {
  display: flex;
  gap: 12px;
  padding: 16px;
}
.toolbar .el-input {
  width: 280px;
}
.toolbar .el-select {
  width: 150px;
}
</style>
