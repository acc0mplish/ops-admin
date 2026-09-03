<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import { executeOpsScript, queryOpsExecHistoryDetail, queryOpsScriptOptions } from '../../api/ops'
import OpsTargetSelector from './components/OpsTargetSelector.vue'
import OpsExecutionResultDialog from './components/OpsExecutionResultDialog.vue'
import { confirmRiskOperation } from '../../composables/useRiskConfirm'
import { ot } from '../../utils/ops-i18n'

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
  variables: {},
  hostIds: [],
  groupId: undefined,
  concurrency: 5,
  timeoutSeconds: 300
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
    form.timeoutSeconds = selectedScriptTimeout.value
    form.variables = Object.fromEntries((selectedScript.value.variables || []).filter((item) => !item.secret).map((item) => [item.name, item.defaultValue || '']))
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
    ElMessage.warning(ot('selectScript'))
    return
  }
  if (!form.hostIds.length && !form.groupId) {
    ElMessage.warning(ot('targetRequired'))
    return
  }
  const requiredVariable = (selectedScript.value?.variables || []).find((item) => item.required && !String(form.variables[item.name] || '').trim())
  if (requiredVariable) {
    ElMessage.warning(ot('variableRequired', { name: requiredVariable.name }))
    return
  }
  const highRisk = /rm\s+-rf|mkfs|shutdown|reboot|init\s+[06]|dd\s+if=|iptables\s+-f|systemctl\s+stop|userdel|drop\s+database|truncate\s+table/i.test(selectedScript.value?.content || '')
  const selectedHosts = hostOptions.value.filter((host) => {
    if (form.groupId) {
      return Number(host.groupId) === Number(form.groupId) || (host.hostGroups || []).some((group) => Number(group.id) === Number(form.groupId))
    }
    return form.hostIds.map(Number).includes(Number(host.id))
  })
  const includesProduction = selectedHosts.some((host) => ['prod', 'production'].includes(String(host.environment || '').toLowerCase()) || String(host.environment || '').includes('\u751f\u4ea7'))
  const confirmationRequired = highRisk || includesProduction
  if (confirmationRequired) {
    await confirmRiskOperation({
      operation: ot('scriptExecutionRisk', { name: selectedScript.value?.name || '-' }),
      targetSummary: selectedHosts.length ? selectedHosts.map((host) => host.hostName || host.sshIp || host.id).slice(0, 4).join(', ') : ot('selectedHostGroup'),
      targetCount: selectedHosts.length,
      production: includesProduction,
      destructive: highRisk
    })
  }

  submitting.value = true
  resultVisible.value = true
  resultLoading.value = true
  resultTask.value = {
    title: form.title || ot('scriptTask'),
    taskType: 'script',
    status: 'running',
    summary: ot('executingPleaseWait')
  }
  resultRows.value = []

  try {
    const data = await executeOpsScript({
      title: form.title,
      scriptId: form.scriptId,
      parameters: form.parameters,
      variables: form.variables,
      hostIds: form.hostIds,
      groupIds: form.groupId ? [form.groupId] : [],
      concurrency: form.concurrency,
      timeoutSeconds: form.timeoutSeconds,
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
    ElMessage.info(ot('resultSavedToHistory'))
  }
}

onMounted(loadOptions)
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ ot('scriptExecution') }}</h2>
        <p class="page-desc">{{ ot('scriptExecutionDesc') }}</p>
      </div>
    </div>

    <el-form label-width="110px">
      <el-form-item :label="ot('taskName')">
        <el-input v-model="form.title" :placeholder="ot('optionalAutoGenerated')" />
      </el-form-item>

      <el-form-item :label="ot('scriptSelection')" required>
        <el-select v-model="form.scriptId" filterable :placeholder="ot('selectLibraryScript')" style="width: 100%">
          <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </el-form-item>

      <el-form-item v-if="selectedScript" :label="ot('executionVariables')">
        <div class="execution-variables">
          <div class="execution-variable-head"><span>{{ ot('variableInjectionHint') }}</span></div>
          <el-empty v-if="!(selectedScript.variables || []).length" :image-size="40" :description="ot('noVariables')" />
          <div v-else class="execution-variable-grid">
            <label v-for="variable in selectedScript.variables" :key="variable.name" class="execution-variable-item"><span><b>VARIABLE_{{ variable.name }}</b><em v-if="variable.required">{{ ot('required') }}</em><small>{{ variable.description || ot('injectedVariable') }}</small></span><el-input v-model="form.variables[variable.name]" :type="variable.secret ? 'password' : 'text'" :show-password="variable.secret" :placeholder="variable.secret ? ot('secretValue') : (variable.defaultValue || ot('enterValue'))" /></label>
          </div>
        </div>
      </el-form-item>

      <OpsTargetSelector
        :host-options="hostOptions"
        :group-options="groupOptions"
        :host-ids="form.hostIds"
        :group-id="form.groupId"
        @update:host-ids="form.hostIds = $event"
        @update:group-id="form.groupId = $event"
      />

      <el-form-item :label="ot('concurrency')">
        <el-input-number v-model="form.concurrency" :min="1" :max="10" />
      </el-form-item>

      <el-form-item :label="ot('timeout')">
        <div class="timeout-field">
          <el-input-number v-model="form.timeoutSeconds" :min="10" :max="3600" :step="10" controls-position="right" />
          <span class="timeout-unit">{{ ot('timeoutAutoTerminate') }}</span>
        </div>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" :loading="submitting" @click="submit">{{ ot('executeNow') }}</el-button>
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

.timeout-field {
  display: inline-flex;
  align-items: center;
  gap: 12px;
}

.timeout-unit {
  color: #7282a0;
  font-size: 13px;
}

.execution-variables { width: 100%; padding: 14px 16px; border: 1px solid #d7e4fb; border-radius: 9px; background: #fafcff; }.execution-variable-head { margin-bottom: 12px; color: #7184a2; font-size: 13px; }.execution-variable-head code { color: #456ee8; font-family: 'JetBrains Mono', Consolas, monospace; }.execution-variable-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }.execution-variable-item { display: grid; grid-template-columns: minmax(140px, .9fr) minmax(0, 1.1fr); align-items: center; gap: 10px; padding: 9px 10px; border: 1px solid #e0e8f5; border-radius: 7px; background: #fff; }.execution-variable-item b { display: block; color: #25446e; font: 12px 'JetBrains Mono', Consolas, monospace; }.execution-variable-item small { display: block; margin-top: 3px; color: #8492a8; font-size: 12px; }.execution-variable-item em { margin-left: 6px; color: #e55858; font-size: 12px; font-style: normal; } @media (max-width: 900px) { .execution-variable-grid { grid-template-columns: 1fr; }.execution-variable-item { grid-template-columns: 1fr; } }
</style>
