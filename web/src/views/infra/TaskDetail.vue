<template>
  <el-drawer v-model="visible" :title="inft('taskDetailTitle')" size="55%" @closed="stopPolling">
    <template v-if="task">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item :label="inft('taskUidCol')" :span="2">{{ task.uid }}</el-descriptions-item>
        <el-descriptions-item :label="inft('operationCol')">{{ task.operation }} @ {{ task.operationVersion }}</el-descriptions-item>
        <el-descriptions-item :label="inft('statusCol')">
          <el-tag :type="statusTagType(task.status)" size="small">{{ task.status }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="inft('approvalCol')">{{ task.approvalStatus }}</el-descriptions-item>
        <el-descriptions-item :label="inft('approverCol')">{{ task.approver || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="inft('resourceCol')" :span="2">{{ task.resourceUid }}</el-descriptions-item>
        <el-descriptions-item :label="inft('attemptsCol')">{{ task.attemptCount }}/{{ task.maxAttempts }}</el-descriptions-item>
        <el-descriptions-item :label="inft('requiresApprovalCol')">{{ task.requiresApproval ? inft('builtinYes') : inft('builtinNo') }}</el-descriptions-item>
        <el-descriptions-item :label="inft('cancelRequestedCol')">{{ task.cancelRequested ? inft('builtinYes') : inft('builtinNo') }}</el-descriptions-item>
        <el-descriptions-item :label="inft('createdAtCol')">{{ formatTime(task.createdAt) }}</el-descriptions-item>
        <el-descriptions-item v-if="task.errorCode" :label="inft('errorCodeCol')">{{ task.errorCode }}</el-descriptions-item>
        <el-descriptions-item v-if="task.errorMessage" :label="inft('errorMessageCol')" :span="2">{{ task.errorMessage }}</el-descriptions-item>
      </el-descriptions>

      <div class="detail-actions">
        <el-button v-permission="'ops:job:approve'" type="primary" :disabled="!canDecide" :loading="acting" @click="decide('approve')">{{ inft('approve') }}</el-button>
        <el-button v-permission="'ops:job:approve'" type="danger" plain :disabled="!canDecide" :loading="acting" @click="decide('reject')">{{ inft('reject') }}</el-button>
        <el-button v-permission="'ops:job:approve'" :disabled="!canCancel" :loading="acting" @click="cancelTask">{{ inft('cancelTask') }}</el-button>
      </div>

      <h4 class="json-title">{{ inft('detailPayload') }}</h4>
      <pre class="json-block">{{ pretty(task.payload) }}</pre>

      <h4 class="json-title">{{ inft('detailEvents') }}</h4>
      <el-timeline class="event-timeline">
        <el-timeline-item v-for="(event, index) in events" :key="index" :timestamp="formatTime(event.at)" placement="top" :type="eventType(event.type)">
          <div class="event-head">
            <el-tag size="small" :type="eventType(event.type)">{{ event.type }}</el-tag>
            <span class="event-meta">{{ inft('eventAttempt') }} {{ event.attemptNo }} · {{ event.actor || '-' }}</span>
          </div>
          <pre v-if="Object.keys(event.data || {}).length" class="json-block event-data">{{ pretty(event.data) }}</pre>
        </el-timeline-item>
      </el-timeline>

      <el-button class="drawer-close" @click="visible = false">{{ inft('close') }}</el-button>
    </template>
  </el-drawer>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  approveInfraTask, cancelInfraTask, getInfraTask, listInfraTaskEvents, rejectInfraTask
} from '../../api/infra'
import { inft } from '../../utils/infra-i18n'

const emit = defineEmits(['updated'])

// §13.5 closed 9-state vocabulary; the terminal set is exactly four (r2 A).
const TERMINAL_STATUSES = ['succeeded', 'failed', 'timed_out', 'cancelled']
const POLL_INTERVAL_MS = 2000

const visible = ref(false)
const task = ref(null)
const events = ref([])
const acting = ref(false)
let pollTimer = null
let currentUid = ''

const canDecide = computed(() => task.value?.status === 'awaiting_approval')
const canCancel = computed(() => Boolean(
  task.value && !TERMINAL_STATUSES.includes(task.value.status)
    && task.value.status !== 'cancelling' && !task.value.cancelRequested
))

function statusTagType(status) {
  if (status === 'succeeded') return 'success'
  if (status === 'failed' || status === 'timed_out') return 'danger'
  if (status === 'cancelled' || status === 'cancelling') return 'info'
  if (status === 'running') return 'warning'
  return 'primary'
}

function eventType(type) {
  if (type === 'succeeded' || type === 'approved') return 'success'
  if (type === 'failed' || type === 'rejected' || type === 'lease_expired') return 'danger'
  return 'info'
}

function pretty(value) {
  return JSON.stringify(value ?? {}, null, 2)
}

function formatTime(value) {
  if (!value) return '-'
  return String(value).replace('T', ' ').slice(0, 19)
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

function schedulePoll() {
  stopPolling()
  if (!task.value || TERMINAL_STATUSES.includes(task.value.status)) return
  pollTimer = window.setInterval(() => { loadTask(currentUid, true) }, POLL_INTERVAL_MS)
}

async function loadTask(uid, silent = false) {
  if (!uid) return
  try {
    const response = await getInfraTask(uid)
    task.value = response?.data?.task || null
    if (silent) return
    const eventsResponse = await listInfraTaskEvents(uid)
    events.value = eventsResponse?.data?.events || []
    schedulePoll()
  } catch (error) {
    if (silent) return
    task.value = null
    events.value = []
    ElMessage.error(inft('loadFailed'))
  }
}

async function act(action, successKey) {
  if (!task.value) return
  acting.value = true
  try {
    await action(task.value.uid)
    ElMessage.success(inft(successKey))
    await loadTask(currentUid, true)
    emit('updated')
  } catch (error) {
    ElMessage.error(inft('actionFailed'))
  } finally {
    acting.value = false
  }
}

function decide(verb) {
  return verb === 'approve' ? act(approveInfraTask, 'approveSuccess') : act(rejectInfraTask, 'rejectSuccess')
}

function cancelTask() {
  return act(cancelInfraTask, 'cancelSuccess')
}

function open(uid) {
  currentUid = uid
  task.value = null
  events.value = []
  visible.value = true
  loadTask(uid)
}

defineExpose({ open })

onBeforeUnmount(stopPolling)
</script>

<style scoped>
.detail-actions { display: flex; gap: 8px; margin: 16px 0 4px; }
.json-title { margin: 14px 0 6px; font-size: 14px; }
.json-block { margin: 0; padding: 10px; background: var(--el-fill-color-light); border-radius: 6px; font-size: 12px; overflow: auto; max-height: 260px; }
.event-timeline { margin-top: 10px; padding-left: 4px; }
.event-head { display: flex; align-items: center; gap: 8px; }
.event-meta { color: var(--el-text-color-secondary); font-size: 12px; }
.event-data { margin-top: 6px; max-height: 140px; }
.drawer-close { margin-top: 16px; }
</style>
