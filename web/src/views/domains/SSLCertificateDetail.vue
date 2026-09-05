<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteSSLCertificate,
  downloadSSLCertificate,
  getSSLCertificate,
  querySSLCertificateTasks,
  renewSSLCertificate,
  resyncSSLCertificate,
  updateSSLCertificateRenewSettings
} from '../../api/domain'
import { dt } from '../../utils/domain-i18n'

const route = useRoute(), router = useRouter()
const loading = ref(false), saving = ref(false), certificate = ref(null), tasks = ref([])
const renewForm = reactive({ autoRenew: false, renewBeforeDays: 30 })
let pollTimer

const statusMeta = {
  PENDING: [dt('sslStatusPending'), 'info'], APPLYING: [dt('sslStatusApplying'), 'warning'], DNS_PENDING: [dt('sslStatusDnsPending'), 'warning'], VALIDATING: [dt('sslStatusValidating'), 'warning'], ISSUING: [dt('sslStatusIssuing'), 'warning'], ISSUED: [dt('sslStatusIssued'), 'success'], NORMAL: [dt('sslStatusNormal'), 'success'], EXPIRING: [dt('sslStatusExpiring'), 'warning'], EXPIRED: [dt('sslStatusExpired'), 'danger'], RENEWING: [dt('sslStatusRenewing'), 'warning'], RENEW_FAILED: [dt('sslStatusRenewFailed'), 'danger'], APPLY_FAILED: [dt('sslStatusApplyFailed'), 'danger'], SYNC_FAILED: [dt('sslStatusSyncFailed'), 'danger'], REVOKED: [dt('sslStatusRevoked'), 'info']
}
const sourceLabels = { ALIYUN: dt('sourceAliyunSync'), TENCENT: dt('sourceTencentSync'), MANUAL: dt('sourceManualUpload'), ACME: dt('sourcePlatformAcme') }
const typeLabels = { SINGLE: dt('certTypeSingle'), WILDCARD: dt('certTypeWildcard'), SAN: dt('certTypeSan') }
const taskLabels = { APPLY: dt('taskApply'), RENEW: dt('taskRenew'), SYNC: dt('cloudSync'), DELETE: dt('deleteCert') }
const stageLabels = { QUEUED: dt('stageQueued'), STARTING: dt('stageStarting'), PREPARING: dt('stagePreparing'), DNS_CHALLENGE: dt('stageDnsChallenge'), VALIDATING: dt('stageValidating'), ISSUING: dt('stageIssuing'), SAVING: dt('stageSaving'), CLOUD_SYNC: dt('cloudSync'), COMPLETED: dt('stageCompleted'), FAILED: dt('stageFailed') }
const hasActiveTask = computed(() => tasks.value.some(item => ['PENDING', 'RUNNING'].includes(item.status)))

function formatDate(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' }
function providerLabel(value) { return value === 'aliyun' ? 'Alibaba Cloud' : value === 'tencent' ? 'Tencent Cloud' : dt('syncStateLocal') }
function statusLabel(value) { return statusMeta[value]?.[0] || value || '—' }
function statusType(value) { return statusMeta[value]?.[1] || 'info' }
function syncLabel(value) { return ({ SYNCED: dt('syncStateSynced'), FAILED: dt('sslStatusSyncFailed'), PENDING: dt('syncStatePending'), LOCAL: dt('syncStateLocal') })[value] || value || '—' }

async function load() {
  loading.value = true
  try {
    certificate.value = await getSSLCertificate(Number(route.params.id))
    Object.assign(renewForm, { autoRenew: certificate.value.autoRenew, renewBeforeDays: certificate.value.renewBeforeDays || 30 })
    tasks.value = certificate.value.tasks || []
    syncPolling()
  } finally { loading.value = false }
}

async function loadTasks() {
  tasks.value = await querySSLCertificateTasks({ certificateId: Number(route.params.id), limit: 30 }) || []
  if (!hasActiveTask.value) { stopPolling(); await load() }
}
function syncPolling() { if (hasActiveTask.value && !pollTimer) pollTimer = setInterval(loadTasks, 3000); if (!hasActiveTask.value) stopPolling() }
function stopPolling() { if (pollTimer) clearInterval(pollTimer); pollTimer = null }

async function saveRenewSettings() {
  saving.value = true
  try {
    await updateSSLCertificateRenewSettings({ id: certificate.value.id, ...renewForm })
    ElMessage.success(dt('renewPolicySaved')); await load()
  } finally { saving.value = false }
}
async function renewNow() {
  await ElMessageBox.confirm(dt('renewConfirmDetail'), dt('renewNow'), { type: 'warning' })
  await renewSSLCertificate(certificate.value.id); ElMessage.success(dt('renewTaskCreated')); await loadTasks(); syncPolling()
}
async function syncCloud() { await resyncSSLCertificate(certificate.value.id); ElMessage.success(dt('resyncTaskCreated')); await loadTasks(); syncPolling() }
async function remove() {
  const deleteCloud = Boolean(certificate.value.cloudCertificateId)
  await ElMessageBox.confirm(dt('deleteConfirmDetailPrefix') + (deleteCloud ? ` ${dt('deleteCloudAttempt')}` : ''), dt('deleteCert'), { type: 'error', confirmButtonText: dt('confirmDelete') })
  const result = await deleteSSLCertificate({ id: certificate.value.id, deleteCloud })
  if (result.taskId) { ElMessage.success(dt('deleteTaskCreatedDetail')); await loadTasks(); syncPolling() } else { ElMessage.success(dt('certDeleted')); router.push('/domains/public/certificates') }
}
function filenameFrom(response, fallback) {
  const disposition = response.headers?.['content-disposition'] || ''
  const matched = disposition.match(/filename="?([^";]+)"?/i)
  return matched?.[1] || fallback
}
async function download(type, sensitive = false) {
  const response = await downloadSSLCertificate(certificate.value.id, type, sensitive)
  const url = URL.createObjectURL(response.data)
  const link = document.createElement('a'); link.href = url; link.download = filenameFrom(response, `${certificate.value.name}-${type}.pem`); link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
onBeforeUnmount(stopPolling)
</script>

<template>
  <section v-loading="loading" class="domain-page domain-panel page-card cert-detail">
    <template v-if="certificate">
      <div class="cert-hero">
        <div>
          <el-button link class="cert-back" @click="router.push('/domains/public/certificates')">{{ dt('backToSslList') }}</el-button>
          <div class="domain-eyebrow">{{ uiT('tlsCertificateDetail') }}</div>
          <div class="cert-title"><h2>{{ certificate.name }}</h2><el-tag :type="statusType(certificate.status)" effect="light">{{ statusLabel(certificate.status) }}</el-tag></div>
          <p class="domain-mono">{{ certificate.domains.join(' · ') }}</p>
        </div>
        <div class="cert-actions">
          <el-dropdown trigger="click"><el-button>{{ dt('downloadCertMenu') }}</el-button><template #dropdown><el-dropdown-menu>
            <el-dropdown-item v-permission="'domains:ssl:download'" @click="download('certificate')">{{ uiT('certificatePem') }}</el-dropdown-item>
            <el-dropdown-item v-permission="'domains:ssl:download'" @click="download('chain')">{{ uiT('certificateChain') }}</el-dropdown-item>
            <el-dropdown-item v-if="certificate.hasPrivateKey" v-permission="'domains:ssl:download-key'" divided @click="download('private-key', true)">{{ dt('privateKeySensitive') }}</el-dropdown-item>
            <el-dropdown-item v-if="certificate.hasPrivateKey" v-permission="'domains:ssl:download-key'" @click="download('zip', true)">{{ dt('fullZipSensitive') }}</el-dropdown-item>
          </el-dropdown-menu></template></el-dropdown>
          <el-button v-permission="'domains:ssl:sync'" @click="syncCloud">{{ dt('syncToCloud') }}</el-button>
          <el-button v-if="certificate.source==='ACME'" v-permission="'domains:ssl:renew'" type="primary" @click="renewNow">{{ dt('renewNow') }}</el-button>
          <el-button v-permission="'domains:ssl:delete'" type="danger" plain @click="remove">{{ dt('delete') }}</el-button>
        </div>
      </div>

      <div v-if="hasActiveTask" class="cert-running"><span class="cert-pulse"></span><strong>{{ taskLabels[tasks[0].taskType] || tasks[0].taskType }}</strong><span>{{ stageLabels[tasks[0].stage] || tasks[0].stage }}</span><el-progress :percentage="tasks[0].progress" :stroke-width="6" /></div>

      <div class="cert-grid">
        <article class="cert-card cert-overview">
          <div class="cert-card-head"><div><span>{{ dt('certInfoTitle') }}</span><h3>{{ dt('certInfoSub') }}</h3></div><strong :class="{'is-warning':certificate.remainingDays<=30}">{{ certificate.remainingDays }} {{ dt('dayUnit') }}</strong></div>
          <dl class="cert-meta">
            <div><dt>Main Domain</dt><dd class="domain-mono">{{ certificate.mainDomain }}</dd></div><div><dt>{{ dt('certTypeLabel') }}</dt><dd>{{ typeLabels[certificate.type] || certificate.type }}</dd></div>
            <div><dt>Source</dt><dd>{{ sourceLabels[certificate.source] || certificate.source }}</dd></div><div><dt>{{ dt('issuer') }}</dt><dd>{{ certificate.issuer || '—' }}</dd></div>
            <div><dt>{{ dt('notBeforeLabel') }}</dt><dd>{{ formatDate(certificate.notBefore) }}</dd></div><div><dt>{{ dt('notAfterLabel') }}</dt><dd>{{ formatDate(certificate.notAfter) }}</dd></div>
            <div><dt>{{ dt('keyAlgorithm') }}</dt><dd class="domain-mono">{{ certificate.keyAlgorithm || '—' }}</dd></div><div><dt>Private Key</dt><dd><el-tag :type="certificate.hasPrivateKey?'success':'info'" effect="plain">{{ certificate.hasPrivateKey?dt('keyStoredEncrypted'):dt('keyNotStored') }}</el-tag></dd></div>
            <div class="cert-wide"><dt>{{ dt('serialNumber') }}</dt><dd class="domain-mono cert-break">{{ certificate.serialNumber || '—' }}</dd></div><div class="cert-wide"><dt>SHA-256 Fingerprint</dt><dd class="domain-mono cert-break">{{ certificate.fingerprintSha256 || '—' }}</dd></div>
          </dl>
        </article>

        <article class="cert-card">
          <div class="cert-card-head"><div><span>{{ dt('cloudCopyTitle') }}</span><h3>{{ dt('syncStatusTitle') }}</h3></div><el-tag :type="certificate.cloudSyncStatus==='SYNCED'?'success':certificate.cloudSyncStatus==='FAILED'?'danger':'info'" effect="plain">{{ syncLabel(certificate.cloudSyncStatus) }}</el-tag></div>
          <dl class="cert-stack"><div><dt>Cloud Provider</dt><dd>{{ providerLabel(certificate.provider) }}</dd></div><div><dt>DNS / Cloud Account</dt><dd>{{ certificate.dnsAccountName || '—' }}</dd></div><div><dt>{{ dt('cloudCertId') }}</dt><dd class="domain-mono cert-break">{{ certificate.cloudCertificateId || '—' }}</dd></div><div><dt>{{ dt('lastSync') }}</dt><dd>{{ formatDate(certificate.lastSyncAt) }}</dd></div></dl>
          <el-alert v-if="certificate.lastSyncError" :title="certificate.lastSyncError" type="error" :closable="false" show-icon />
        </article>

        <article class="cert-card">
          <div class="cert-card-head"><div><span>{{ dt('renewPolicyTitle') }}</span><h3>{{ dt('autoRenewLabel') }}</h3></div><el-switch v-model="renewForm.autoRenew" :disabled="certificate.source!=='ACME'" /></div>
          <el-form label-position="top"><el-form-item :label="dt('renewStartDaysBefore')"><el-input-number v-model="renewForm.renewBeforeDays" :min="1" :max="90" :disabled="certificate.source!=='ACME'" /><span class="cert-unit">{{ dt('dayUnit') }}</span></el-form-item><el-form-item label="ACME CA"><span class="domain-mono">{{ certificate.acmeCa || '—' }}</span></el-form-item><el-button v-if="certificate.source==='ACME'" v-permission="'domains:ssl:settings'" type="primary" :loading="saving" @click="saveRenewSettings">{{ dt('saveRenewPolicy') }}</el-button><div v-else class="domain-form-tip">{{ dt('noAutoRenewTip') }}</div></el-form>
          <el-alert v-if="certificate.lastRenewError" :title="certificate.lastRenewError" type="error" :closable="false" show-icon />
        </article>
      </div>

      <article class="cert-card cert-tasks">
        <div class="cert-card-head"><div><span>{{ dt('executionHistory') }}</span><h3>{{ dt('certTaskTitle') }}</h3></div><el-button link @click="loadTasks">{{ dt('refresh') }}</el-button></div>
        <el-table :data="tasks" border><el-table-column label="Task" width="130"><template #default="{row}">{{ taskLabels[row.taskType] || row.taskType }}</template></el-table-column><el-table-column :label="dt('progressCol')" min-width="210"><template #default="{row}"><div class="task-progress"><el-progress :percentage="row.progress" :status="row.status==='FAILED'?'exception':row.status==='SUCCESS'?'success':''" /><small>{{ stageLabels[row.stage] || row.stage }}</small></div></template></el-table-column><el-table-column prop="username" :label="dt('operatorShort')" width="120"/><el-table-column :label="dt('startedAtLabel')" width="180"><template #default="{row}">{{ formatDate(row.startedAt || row.createdAt) }}</template></el-table-column><el-table-column :label="dt('finishedAtLabel')" width="180"><template #default="{row}">{{ formatDate(row.finishedAt) }}</template></el-table-column><el-table-column prop="errorMessage" :label="dt('failReason')" min-width="220" show-overflow-tooltip /></el-table>
      </article>

      <div class="cert-footnote">{{ dt('certFootnote', { created: formatDate(certificate.createdAt), updated: formatDate(certificate.updatedAt), by: certificate.createdBy || '—' }) }}</div>
    </template>
  </section>
</template>

<style scoped>
.cert-detail{display:flex;flex-direction:column;gap:16px}.cert-hero{display:flex;justify-content:space-between;gap:24px;padding:4px 2px 16px;border-bottom:1px solid #e5eaf3}.cert-back{margin-bottom:16px;color:#64748b}.cert-title{display:flex;align-items:center;gap:12px}.cert-title h2{margin:3px 0 4px;font-size:28px}.cert-hero p{margin:4px 0 0;color:#526581}.cert-actions{display:flex;align-items:flex-start;justify-content:flex-end;gap:8px;flex-wrap:wrap}.cert-running{display:grid;grid-template-columns:auto auto minmax(120px,1fr) minmax(240px,2fr);align-items:center;gap:10px;padding:12px 14px;border:1px solid #bfdbfe;border-radius:12px;background:#eff6ff;color:#475569}.cert-pulse{width:9px;height:9px;border-radius:50%;background:#10b981;box-shadow:0 0 0 5px #d1fae5}.cert-grid{display:grid;grid-template-columns:minmax(0,1.5fr) minmax(280px,.75fr);gap:14px}.cert-card{padding:18px;border:1px solid #dce5f2;border-radius:14px;background:#fff}.cert-overview{grid-row:span 2}.cert-card-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:18px}.cert-card-head span{color:#64748b;font-size:12px;letter-spacing:.06em}.cert-card-head h3{margin:4px 0 0;font-size:18px;color:#13233d}.cert-card-head>strong{font-size:26px;color:#047857}.cert-card-head>strong.is-warning{color:#b45309}.cert-meta{display:grid;grid-template-columns:1fr 1fr;gap:18px 24px;margin:0}.cert-meta div,.cert-stack div{min-width:0}.cert-meta dt,.cert-stack dt{margin-bottom:6px;color:#71819b;font-size:12px}.cert-meta dd,.cert-stack dd{margin:0;color:#18243a}.cert-wide{grid-column:1/-1}.cert-stack{display:grid;gap:16px;margin:0 0 16px}.cert-break{overflow-wrap:anywhere}.cert-unit{margin-left:8px;color:#64748b}.cert-tasks{overflow:hidden}.task-progress{display:grid;grid-template-columns:minmax(120px,1fr) 90px;align-items:center;gap:10px}.task-progress small{color:#64748b}.cert-footnote{text-align:right;color:#8492a8;font-size:12px}
@media(max-width:1100px){.cert-hero{flex-direction:column}.cert-actions{justify-content:flex-start}.cert-grid{grid-template-columns:1fr}.cert-overview{grid-row:auto}}
@media(max-width:720px){.cert-actions,.cert-actions .el-button,.cert-actions .el-dropdown{width:100%}.cert-running{grid-template-columns:auto 1fr}.cert-running .el-progress{grid-column:1/-1}.cert-meta{grid-template-columns:1fr}.cert-wide{grid-column:auto}.task-progress{grid-template-columns:1fr}}
</style>
