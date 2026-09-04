<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import { executeOpsFileDispatch, queryOpsExecHistoryDetail } from '../../api/ops'
import OpsTargetSelector from './components/OpsTargetSelector.vue'
import OpsExecutionResultDialog from './components/OpsExecutionResultDialog.vue'
import { confirmRiskOperation } from '../../composables/useRiskConfirm'
import { ot } from '../../utils/ops-i18n'

const submitting = ref(false)
const hostOptions = ref([])
const groupOptions = ref([])
const resultVisible = ref(false)
const resultLoading = ref(false)
const resultTask = ref(null)
const resultRows = ref([])
let pollTimer = null

const form = reactive({
  title: '',
  sourceType: 'upload',
  sourceHostId: undefined,
  sourcePath: '',
  targetPath: '',
  hostIds: [],
  groupId: undefined,
  concurrency: 5,
  timeoutSeconds: 10,
  overwrite: false,
  file: null
})

async function loadOptions() {
  const [hosts, groups] = await Promise.all([
    queryAssetHostList({ pageNum: 1, pageSize: 1000 }),
    queryAssetHostGroupList()
  ])
  hostOptions.value = hosts.list || []
  groupOptions.value = groups.tree || []
}

function handleFileChange(file) {
  form.file = file?.raw || null
}

async function submit() {
  if (!form.targetPath.trim()) {
    ElMessage.warning(ot('targetPathRequired'))
    return
  }
  if (!form.hostIds.length && !form.groupId) {
    ElMessage.warning(ot('targetRequired'))
    return
  }
  if (form.sourceType === 'upload' && !form.file) {
    ElMessage.warning(ot('uploadFileRequired'))
    return
  }
  if (form.sourceType === 'server' && (!form.sourceHostId || !form.sourcePath.trim())) {
    ElMessage.warning(ot('sourceServerRequired'))
    return
  }
  const highRisk = /^\/(etc|boot|usr)\//.test(form.targetPath.trim())
  const selectedHosts = hostOptions.value.filter((host) => {
    if (form.groupId) {
      return Number(host.groupId) === Number(form.groupId) || (host.hostGroups || []).some((group) => Number(group.id) === Number(form.groupId))
    }
    return form.hostIds.map(Number).includes(Number(host.id))
  })
  const includesProduction = selectedHosts.some((host) => ['prod', 'production'].includes(String(host.environment || '').toLowerCase()) || String(host.environment || '').includes('운영'))
  const confirmationRequired = highRisk || includesProduction || form.overwrite
  if (confirmationRequired) {
    await confirmRiskOperation({
      operation: ot('fileDispatchRisk', { path: form.targetPath }),
      targetSummary: selectedHosts.length ? selectedHosts.map((host) => host.hostName || host.sshIp || host.id).slice(0, 4).join(', ') : ot('selectedHostGroup'),
      targetCount: selectedHosts.length,
      production: includesProduction,
      destructive: highRisk || form.overwrite
    })
  }

  const payload = new FormData()
  payload.append('title', form.title)
  payload.append('sourceType', form.sourceType)
  payload.append('sourceHostId', String(form.sourceHostId || 0))
  payload.append('sourcePath', form.sourcePath)
  payload.append('targetPath', form.targetPath)
  payload.append('hostIds', JSON.stringify(form.hostIds))
  payload.append('groupIds', JSON.stringify(form.groupId ? [form.groupId] : []))
  payload.append('concurrency', String(form.concurrency))
  payload.append('timeoutSeconds', String(form.timeoutSeconds))
  payload.append('overwrite', String(form.overwrite))
  payload.append('riskConfirmed', String(confirmationRequired))
  if (form.file) payload.append('file', form.file)

  submitting.value = true
  resultVisible.value = true
  resultLoading.value = true
  resultTask.value = {
    title: form.title || ot('fileDispatchTask'),
    taskType: 'file',
    status: 'running',
    summary: ot('executingPleaseWait')
  }
  resultRows.value = []
  try {
    const data = await executeOpsFileDispatch(payload)
    resultTask.value = data.task || null
    resultRows.value = data.results || []
    startPolling()
  } finally {
    submitting.value = false
  }
}

async function refreshTaskDetail() {
  if (!resultTask.value?.id) return
  const data = await queryOpsExecHistoryDetail(resultTask.value.id)
  resultTask.value = data.task || null
  resultRows.value = data.results || []
  resultLoading.value = false
  if (resultTask.value?.status && resultTask.value.status !== 'running') stopPolling()
}

function startPolling() {
  stopPolling()
  refreshTaskDetail().catch(() => {})
  pollTimer = setInterval(() => refreshTaskDetail().catch(() => {}), 1200)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function handleResultClosed() {
  stopPolling()
  if (resultTask.value?.id) ElMessage.info(ot('resultSavedToHistory'))
}

onMounted(loadOptions)
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header"><div><h2 class="page-title">{{ ot('fileDispatch') }}</h2><p class="page-desc">{{ ot('fileDispatchDesc') }}</p></div></div>
    <el-form label-width="110px">
      <el-form-item :label="ot('taskName')"><el-input v-model="form.title" :placeholder="ot('optionalAutoGenerated')" /></el-form-item>
      <el-form-item :label="ot('sourceMode')"><el-radio-group v-model="form.sourceType"><el-radio value="upload">{{ ot('localUpload') }}</el-radio><el-radio value="server">{{ ot('serverSourceFile') }}</el-radio></el-radio-group></el-form-item>
      <el-form-item v-if="form.sourceType === 'upload'" :label="ot('uploadFile')"><el-upload :auto-upload="false" :limit="1" :on-change="handleFileChange"><el-button>{{ ot('chooseFile') }}</el-button></el-upload></el-form-item>
      <template v-else>
        <el-form-item :label="ot('sourceServer')"><el-select v-model="form.sourceHostId" filterable :placeholder="ot('selectSourceServer')" style="width: 100%"><el-option v-for="item in hostOptions" :key="item.id" :label="`${item.hostName} (${item.sshIp || item.privateIp || '-'})`" :value="item.id" /></el-select></el-form-item>
        <el-form-item :label="ot('sourceFilePath')"><el-input v-model="form.sourcePath" :placeholder="ot('sourceFileExample')" /></el-form-item>
      </template>
      <el-form-item :label="ot('targetPath')" required><el-input v-model="form.targetPath" :placeholder="ot('targetPathExample')" /></el-form-item>
      <OpsTargetSelector :host-options="hostOptions" :group-options="groupOptions" :host-ids="form.hostIds" :group-id="form.groupId" @update:host-ids="form.hostIds = $event" @update:group-id="form.groupId = $event" />
      <el-form-item :label="ot('concurrency')"><el-input-number v-model="form.concurrency" :min="1" :max="10" /></el-form-item>
      <el-form-item :label="ot('timeout')"><el-input-number v-model="form.timeoutSeconds" :min="10" :max="3600" /><span class="inline-hint">{{ ot('timeoutHint') }}</span></el-form-item>
      <el-form-item :label="ot('overwriteExisting')"><el-switch v-model="form.overwrite" /></el-form-item>
      <el-form-item><el-button type="primary" :loading="submitting" @click="submit">{{ ot('dispatchNow') }}</el-button></el-form-item>
    </el-form>
    <OpsExecutionResultDialog v-model="resultVisible" :loading="resultLoading" :task="resultTask" :results="resultRows" @closed="handleResultClosed" />
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-title { margin: 0 0 8px; font-size: 22px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.inline-hint { margin-left: 12px; color: #7282a0; font-size: 13px; }
</style>
