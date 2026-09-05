<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteNotifyChannel, notifyChannelInfo, queryNotifyChannelList, saveNotifyChannel } from '../../api/ops'
import { nt } from '../../utils/notify-i18n'

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
  { label: nt('userDefinedWebhook'), value: 'webhook' }
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
    ElMessage.warning(nt('warnChannelFields'))
    return
  }
  saving.value = true
  try {
    await saveNotifyChannel(form)
    ElMessage.success(nt('saved'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(nt('channelDeleteConfirm', { name: row.name }), nt('notice'), { type: 'warning' })
  await deleteNotifyChannel(row.id)
  ElMessage.success(nt('deleted'))
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ nt('channelPageTitle') }}</h2>
        <p class="page-desc">{{ nt('channelPageDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ nt('addChannel') }}</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="nt('channelSearchPlaceholder')" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.channelType" clearable :placeholder="nt('channelType')" style="width: 180px">
        <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.status" clearable :placeholder="nt('status')" style="width: 120px">
        <el-option :label="nt('actionEnable')" value="1" />
        <el-option :label="nt('actionDisable')" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ nt('search') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" :label="nt('channelName')" min-width="180" />
      <el-table-column :label="nt('channelType')" width="180">
        <template #default="{ row }">{{ channelTypeLabel(row.channelType) }}</template>
      </el-table-column>
      <el-table-column prop="webhookUrl" :label="nt('webhookUrlLabel')" min-width="320" show-overflow-tooltip />
      <el-table-column :label="nt('status')" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? nt('enabled') : nt('disabled') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updateTime" :label="nt('updatedAt')" width="180" />
      <el-table-column :label="nt('actions')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ nt('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ nt('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? nt('editChannelTitle') : nt('addChannelTitle')" width="760px">
      <el-form label-width="120px">
        <el-form-item :label="nt('channelName')" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="nt('channelType')">
          <el-select v-model="form.channelType" style="width: 100%">
            <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="nt('webhookUrlLabel')" required><el-input v-model="form.webhookUrl" /></el-form-item>
        <el-form-item :label="nt('signingSecret')">
          <el-input v-model="form.secret" show-password :placeholder="nt('secretPlaceholder')" />
        </el-form-item>
        <el-form-item label="Request Header JSON">
          <el-input v-model="form.headersJson" type="textarea" :rows="5" placeholder='{"Authorization":"Bearer xxx"}' />
        </el-form-item>
        <el-form-item :label="nt('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ nt('actionEnable') }}</el-radio>
            <el-radio :value="2">{{ nt('actionDisable') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="nt('description')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ nt('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ nt('save') }}</el-button>
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
