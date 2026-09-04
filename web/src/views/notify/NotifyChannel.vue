<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteNotifyChannel, notifyChannelInfo, queryNotifyChannelList, saveNotifyChannel } from '../../api/ops'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', channelType: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  channelType: 'dingtalk',
  webhookUrl: '',
  secret: '',
  headersJson: '{}',
  status: 1,
  description: ''
})

const channelTypes = [
  { label: 'DingTalk Bot', value: 'dingtalk' },
  { label: 'WeCom Bot', value: 'wecom' },
  { label: 'Feishu Bot', value: 'feishu' },
  { label: '사용자 정의 HTTP Webhook', value: 'webhook' }
]

function channelTypeLabel(value) {
  return channelTypes.find((item) => item.value === value)?.label || value
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    channelType: 'dingtalk',
    webhookUrl: '',
    secret: '',
    headersJson: '{}',
    status: 1,
    description: ''
  })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryNotifyChannelList(query)
    rows.value = data.list || []
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
  const data = await notifyChannelInfo(row.id)
  Object.assign(form, data)
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.webhookUrl.trim()) {
    ElMessage.warning('Channel 이름과 Webhook 주소를 입력하십시오')
    return
  }
  saving.value = true
  try {
    await saveNotifyChannel(form)
    ElMessage.success('저장했습니다.')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Notification Channel "${row.name}"을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteNotifyChannel(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">메시지 알림 Channel</h2>
        <p class="page-desc">DingTalk, WeCom, Feishu Bot과 사용자 정의 HTTP Webhook의 전송 경로를 관리합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">Channel 추가</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="Channel 이름 / 설명 검색" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.channelType" clearable placeholder="Channel 유형" style="width: 180px">
        <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="상태" style="width: 120px">
        <el-option label="활성화" value="1" />
        <el-option label="비활성화" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="Channel 이름" min-width="180" />
      <el-table-column label="Channel 유형" width="180">
        <template #default="{ row }">{{ channelTypeLabel(row.channelType) }}</template>
      </el-table-column>
      <el-table-column prop="webhookUrl" label="Webhook 주소" min-width="320" show-overflow-tooltip />
      <el-table-column label="상태" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updateTime" label="수정 시각" width="180" />
      <el-table-column label="작업" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Notification Channel 수정' : 'Notification Channel 추가'" width="760px">
      <el-form label-width="120px">
        <el-form-item label="Channel 이름" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="Channel 유형">
          <el-select v-model="form.channelType" style="width: 100%">
            <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="Webhook 주소" required><el-input v-model="form.webhookUrl" /></el-form-item>
        <el-form-item label="서명 Secret">
          <el-input v-model="form.secret" show-password placeholder="DingTalk 서명 Secret. 선택 사항" />
        </el-form-item>
        <el-form-item label="Request Header JSON">
          <el-input v-model="form.headersJson" type="textarea" :rows="5" placeholder='{"Authorization":"Bearer xxx"}' />
        </el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">활성화</el-radio>
            <el-radio :value="2">비활성화</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="설명"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="saving" @click="submit">저장</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
</style>
