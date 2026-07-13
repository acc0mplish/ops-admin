<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import { executeOpsScript, queryOpsExecHistoryDetail, queryOpsScriptOptions } from '../../api/ops'
import OpsTargetSelector from './components/OpsTargetSelector.vue'
import OpsExecutionResultDialog from './components/OpsExecutionResultDialog.vue'

const submitting = ref(false)
const hostOptions = ref([])
const groupOptions = ref([])
const scriptOptions = ref([])
const resultVisible = ref(false)
const resultLoading = ref(false)
const resultTask = ref(null)
const resultRows = ref([])
let pollTimer = null

const form = reactive({
  title: '',
  scriptId: undefined,
  parameters: '',
  hostIds: [],
  groupId: undefined,
  concurrency: 5
})

const selectedScript = computed(() => scriptOptions.value.find((item) => Number(item.id) === Number(form.scriptId || 0)) || null)
const selectedScriptTimeout = computed(() => {
  const value = Number(selectedScript.value?.timeoutSeconds || 0)
  return value > 0 ? value : 300
})

watch(
  () => form.scriptId,
  () => {
    if (!selectedScript.value) return
    if (!form.title.trim()) {
      form.title = selectedScript.value.name || ''
    }
  }
)

async function loadOptions() {
  const [hosts, groups, scripts] = await Promise.all([
    queryAssetHostList({ pageNum: 1, pageSize: 1000 }),
    queryAssetHostGroupList(),
    queryOpsScriptOptions()
  ])
  hostOptions.value = hosts.list || []
  groupOptions.value = groups.tree || []
  scriptOptions.value = scripts || []
}

async function submit() {
  if (!form.scriptId) {
    ElMessage.warning('请选择脚本')
    return
  }
  if (!form.hostIds.length && !form.groupId) {
    ElMessage.warning('请选择目标主机或主机组')
    return
  }
  const highRisk = /rm\s+-rf|mkfs|shutdown|reboot|init\s+[06]|dd\s+if=|iptables\s+-f|systemctl\s+stop|userdel|drop\s+database|truncate\s+table/i.test(selectedScript.value?.content || '')
  const selectedHosts = hostOptions.value.filter((host) => {
    if (form.groupId) {
      return Number(host.groupId) === Number(form.groupId) || (host.hostGroups || []).some((group) => Number(group.id) === Number(form.groupId))
    }
    return form.hostIds.map(Number).includes(Number(host.id))
  })
  const includesProduction = selectedHosts.some((host) => ['prod', 'production'].includes(String(host.environment || '').toLowerCase()) || String(host.environment || '').includes('生产'))
  const confirmationRequired = highRisk || includesProduction
  if (confirmationRequired) {
    await ElMessageBox.confirm(
      includesProduction ? '目标包含生产环境主机，请确认脚本和目标范围后继续。' : '所选脚本包含高风险操作，请确认目标范围后继续。',
      includesProduction ? '生产环境操作确认' : '高风险脚本确认',
      { type: 'warning', confirmButtonText: '确认执行' }
    )
  }

  submitting.value = true
  resultVisible.value = true
  resultLoading.value = true
  resultTask.value = {
    title: form.title || '脚本执行任务',
    taskType: 'script',
    status: 'running',
    summary: '正在执行中，请稍候...'
  }
  resultRows.value = []

  try {
    const data = await executeOpsScript({
      title: form.title,
      scriptId: form.scriptId,
      parameters: form.parameters,
      hostIds: form.hostIds,
      groupIds: form.groupId ? [form.groupId] : [],
      concurrency: form.concurrency,
      riskConfirmed: confirmationRequired
    })
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
  if (resultTask.value?.status && resultTask.value.status !== 'running') {
    stopPolling()
  }
}

function startPolling() {
  stopPolling()
  refreshTaskDetail().catch(() => {})
  pollTimer = setInterval(() => {
    refreshTaskDetail().catch(() => {})
  }, 1200)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function handleResultClosed() {
  stopPolling()
  if (resultTask.value?.id) {
    ElMessage.info('执行结果已保存在快速执行历史中，可随时前往查看。')
  }
}

onMounted(loadOptions)
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">脚本执行</h2>
        <p class="page-desc">从脚本库中选择脚本，再批量下发到目标主机执行。</p>
      </div>
    </div>

    <el-form label-width="110px">
      <el-form-item label="任务名称">
        <el-input v-model="form.title" placeholder="可选，默认自动生成" />
      </el-form-item>

      <el-form-item label="脚本选择" required>
        <el-select v-model="form.scriptId" filterable placeholder="选择脚本库中的脚本" style="width: 100%">
          <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </el-form-item>

      <el-form-item label="执行参数">
        <el-input v-model="form.parameters" placeholder="为空时使用脚本默认参数" />
      </el-form-item>

      <OpsTargetSelector
        :host-options="hostOptions"
        :group-options="groupOptions"
        :host-ids="form.hostIds"
        :group-id="form.groupId"
        @update:host-ids="form.hostIds = $event"
        @update:group-id="form.groupId = $event"
      />

      <el-form-item label="并发数">
        <el-input-number v-model="form.concurrency" :min="1" :max="10" />
      </el-form-item>

      <el-form-item label="超时时间">
        <el-input :model-value="`${selectedScriptTimeout} 秒`" readonly />
        <span class="inline-hint">使用脚本库中的超时秒数，这里不可修改</span>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" :loading="submitting" @click="submit">立即执行</el-button>
      </el-form-item>
    </el-form>

    <OpsExecutionResultDialog
      v-model="resultVisible"
      :loading="resultLoading"
      :task="resultTask"
      :results="resultRows"
      @closed="handleResultClosed"
    />
  </div>
</template>

<style scoped>
.ops-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 22px;
  font-weight: 700;
  color: #14213d;
}

.page-desc {
  margin: 0;
  color: #7282a0;
}

.inline-hint {
  margin-left: 12px;
  color: #7282a0;
  font-size: 13px;
}
</style>
