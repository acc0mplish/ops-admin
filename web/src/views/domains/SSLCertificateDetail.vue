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

const route = useRoute(), router = useRouter()
const loading = ref(false), saving = ref(false), certificate = ref(null), tasks = ref([])
const renewForm = reactive({ autoRenew: false, renewBeforeDays: 30 })
let pollTimer

const statusMeta = {
  PENDING: ['발급 대기', 'info'], APPLYING: ['신청 중', 'warning'], DNS_PENDING: ['DNS 검증 대기', 'warning'], VALIDATING: ['검증 중', 'warning'], ISSUING: ['발급 중', 'warning'], ISSUED: ['발급됨', 'success'], NORMAL: ['정상', 'success'], EXPIRING: ['만료 임박', 'warning'], EXPIRED: ['만료', 'danger'], RENEWING: ['갱신 중', 'warning'], RENEW_FAILED: ['갱신 실패', 'danger'], APPLY_FAILED: ['발급 실패', 'danger'], SYNC_FAILED: ['동기화 실패', 'danger'], REVOKED: ['폐기됨', 'info']
}
const sourceLabels = { ALIYUN: 'Alibaba Cloud 동기화', TENCENT: 'Tencent Cloud 동기화', MANUAL: '수동 업로드', ACME: '플랫폼 ACME' }
const typeLabels = { SINGLE: '단일 Domain', WILDCARD: '와일드카드 Domain', SAN: 'SAN 다중 Domain' }
const taskLabels = { APPLY: '인증서 발급', RENEW: '인증서 갱신', SYNC: '클라우드 동기화', DELETE: '인증서 삭제' }
const stageLabels = { QUEUED: '실행 대기', STARTING: '시작 중', PREPARING: '발급 준비', DNS_CHALLENGE: 'DNS 검증 구성', VALIDATING: 'CA 검증 대기', ISSUING: '인증서 발급', SAVING: '인증서 저장', CLOUD_SYNC: '클라우드 동기화', COMPLETED: '완료됨', FAILED: '실행 실패' }
const hasActiveTask = computed(() => tasks.value.some(item => ['PENDING', 'RUNNING'].includes(item.status)))

function formatDate(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' }
function providerLabel(value) { return value === 'aliyun' ? 'Alibaba Cloud' : value === 'tencent' ? 'Tencent Cloud' : '로컬 전용' }
function statusLabel(value) { return statusMeta[value]?.[0] || value || '—' }
function statusType(value) { return statusMeta[value]?.[1] || 'info' }
function syncLabel(value) { return ({ SYNCED: '동기화됨', FAILED: '동기화 실패', PENDING: '동기화 중', LOCAL: '로컬 전용' })[value] || value || '—' }

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
    ElMessage.success('갱신 정책을 저장했습니다.'); await load()
  } finally { saving.value = false }
}
async function renewNow() {
  await ElMessageBox.confirm('DNS-01 검증을 다시 실행하고 새 버전을 발급합니다. 기존 인증서는 성공할 때까지 유지됩니다.', '즉시 갱신', { type: 'warning' })
  await renewSSLCertificate(certificate.value.id); ElMessage.success('갱신 Task를 생성했습니다.'); await loadTasks(); syncPolling()
}
async function syncCloud() { await resyncSSLCertificate(certificate.value.id); ElMessage.success('클라우드 동기화 Task를 생성했습니다.'); await loadTasks(); syncPolling() }
async function remove() {
  const deleteCloud = Boolean(certificate.value.cloudCertificateId)
  await ElMessageBox.confirm(`삭제 전에 퍼블릭 DNS와 인증서 유효 기간을 확인합니다. ${deleteCloud ? '클라우드 인증서도 함께 삭제를 시도하며, 클라우드 삭제가 실패하면 로컬 Data는 유지됩니다.' : ''}`, '인증서 삭제', { type: 'error', confirmButtonText: '삭제 확인' })
  const result = await deleteSSLCertificate({ id: certificate.value.id, deleteCloud })
  if (result.taskId) { ElMessage.success('삭제 Task를 생성했습니다.'); await loadTasks(); syncPolling() } else { ElMessage.success('인증서를 삭제했습니다.'); router.push('/domains/public/certificates') }
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
          <el-button link class="cert-back" @click="router.push('/domains/public/certificates')">← SSL 인증서로 돌아가기</el-button>
          <div class="domain-eyebrow">{{ uiT('tlsCertificateDetail') }}</div>
          <div class="cert-title"><h2>{{ certificate.name }}</h2><el-tag :type="statusType(certificate.status)" effect="light">{{ statusLabel(certificate.status) }}</el-tag></div>
          <p class="domain-mono">{{ certificate.domains.join(' · ') }}</p>
        </div>
        <div class="cert-actions">
          <el-dropdown trigger="click"><el-button>인증서 다운로드⌄</el-button><template #dropdown><el-dropdown-menu>
            <el-dropdown-item v-permission="'domains:ssl:download'" @click="download('certificate')">{{ uiT('certificatePem') }}</el-dropdown-item>
            <el-dropdown-item v-permission="'domains:ssl:download'" @click="download('chain')">{{ uiT('certificateChain') }}</el-dropdown-item>
            <el-dropdown-item v-if="certificate.hasPrivateKey" v-permission="'domains:ssl:download-key'" divided @click="download('private-key', true)">Private Key (민감)</el-dropdown-item>
            <el-dropdown-item v-if="certificate.hasPrivateKey" v-permission="'domains:ssl:download-key'" @click="download('zip', true)">전체 ZIP (민감)</el-dropdown-item>
          </el-dropdown-menu></template></el-dropdown>
          <el-button v-permission="'domains:ssl:sync'" @click="syncCloud">클라우드로 동기화</el-button>
          <el-button v-if="certificate.source==='ACME'" v-permission="'domains:ssl:renew'" type="primary" @click="renewNow">즉시 갱신</el-button>
          <el-button v-permission="'domains:ssl:delete'" type="danger" plain @click="remove">삭제</el-button>
        </div>
      </div>

      <div v-if="hasActiveTask" class="cert-running"><span class="cert-pulse"></span><strong>{{ taskLabels[tasks[0].taskType] || tasks[0].taskType }}</strong><span>{{ stageLabels[tasks[0].stage] || tasks[0].stage }}</span><el-progress :percentage="tasks[0].progress" :stroke-width="6" /></div>

      <div class="cert-grid">
        <article class="cert-card cert-overview">
          <div class="cert-card-head"><div><span>인증서 정보</span><h3>식별 정보와 유효 기간</h3></div><strong :class="{'is-warning':certificate.remainingDays<=30}">{{ certificate.remainingDays }} 일</strong></div>
          <dl class="cert-meta">
            <div><dt>Main Domain</dt><dd class="domain-mono">{{ certificate.mainDomain }}</dd></div><div><dt>인증서 유형</dt><dd>{{ typeLabels[certificate.type] || certificate.type }}</dd></div>
            <div><dt>Source</dt><dd>{{ sourceLabels[certificate.source] || certificate.source }}</dd></div><div><dt>발급 기관</dt><dd>{{ certificate.issuer || '—' }}</dd></div>
            <div><dt>적용 시각</dt><dd>{{ formatDate(certificate.notBefore) }}</dd></div><div><dt>만료 시각</dt><dd>{{ formatDate(certificate.notAfter) }}</dd></div>
            <div><dt>Key 알고리즘</dt><dd class="domain-mono">{{ certificate.keyAlgorithm || '—' }}</dd></div><div><dt>Private Key</dt><dd><el-tag :type="certificate.hasPrivateKey?'success':'info'" effect="plain">{{ certificate.hasPrivateKey?'암호화 저장됨':'저장 안 됨' }}</el-tag></dd></div>
            <div class="cert-wide"><dt>시리얼 번호</dt><dd class="domain-mono cert-break">{{ certificate.serialNumber || '—' }}</dd></div><div class="cert-wide"><dt>SHA-256 Fingerprint</dt><dd class="domain-mono cert-break">{{ certificate.fingerprintSha256 || '—' }}</dd></div>
          </dl>
        </article>

        <article class="cert-card">
          <div class="cert-card-head"><div><span>클라우드 사본</span><h3>동기화 상태</h3></div><el-tag :type="certificate.cloudSyncStatus==='SYNCED'?'success':certificate.cloudSyncStatus==='FAILED'?'danger':'info'" effect="plain">{{ syncLabel(certificate.cloudSyncStatus) }}</el-tag></div>
          <dl class="cert-stack"><div><dt>Cloud Provider</dt><dd>{{ providerLabel(certificate.provider) }}</dd></div><div><dt>DNS / Cloud Account</dt><dd>{{ certificate.dnsAccountName || '—' }}</dd></div><div><dt>클라우드 인증서 ID</dt><dd class="domain-mono cert-break">{{ certificate.cloudCertificateId || '—' }}</dd></div><div><dt>마지막 동기화</dt><dd>{{ formatDate(certificate.lastSyncAt) }}</dd></div></dl>
          <el-alert v-if="certificate.lastSyncError" :title="certificate.lastSyncError" type="error" :closable="false" show-icon />
        </article>

        <article class="cert-card">
          <div class="cert-card-head"><div><span>갱신 정책</span><h3>자동 갱신</h3></div><el-switch v-model="renewForm.autoRenew" :disabled="certificate.source!=='ACME'" /></div>
          <el-form label-position="top"><el-form-item label="만료 며칠 전에 갱신 시작"><el-input-number v-model="renewForm.renewBeforeDays" :min="1" :max="90" :disabled="certificate.source!=='ACME'" /><span class="cert-unit">일</span></el-form-item><el-form-item label="ACME CA"><span class="domain-mono">{{ certificate.acmeCa || '—' }}</span></el-form-item><el-button v-if="certificate.source==='ACME'" v-permission="'domains:ssl:settings'" type="primary" :loading="saving" @click="saveRenewSettings">갱신 정책 저장</el-button><div v-else class="domain-form-tip">클라우드 동기화 및 수동 업로드 인증서는 플랫폼이 자동 갱신하지 않습니다.</div></el-form>
          <el-alert v-if="certificate.lastRenewError" :title="certificate.lastRenewError" type="error" :closable="false" show-icon />
        </article>
      </div>

      <article class="cert-card cert-tasks">
        <div class="cert-card-head"><div><span>실행 기록</span><h3>인증서 Task</h3></div><el-button link @click="loadTasks">새로고침</el-button></div>
        <el-table :data="tasks" border><el-table-column label="Task" width="130"><template #default="{row}">{{ taskLabels[row.taskType] || row.taskType }}</template></el-table-column><el-table-column label="진행률" min-width="210"><template #default="{row}"><div class="task-progress"><el-progress :percentage="row.progress" :status="row.status==='FAILED'?'exception':row.status==='SUCCESS'?'success':''" /><small>{{ stageLabels[row.stage] || row.stage }}</small></div></template></el-table-column><el-table-column prop="username" label="작업자" width="120"/><el-table-column label="시작 시각" width="180"><template #default="{row}">{{ formatDate(row.startedAt || row.createdAt) }}</template></el-table-column><el-table-column label="완료 시각" width="180"><template #default="{row}">{{ formatDate(row.finishedAt) }}</template></el-table-column><el-table-column prop="errorMessage" label="실패 원인" min-width="220" show-overflow-tooltip /></el-table>
      </article>

      <div class="cert-footnote">생성 {{ formatDate(certificate.createdAt) }} · 마지막 업데이트 {{ formatDate(certificate.updatedAt) }} · 생성자 {{ certificate.createdBy || '—' }}</div>
    </template>
  </section>
</template>

<style scoped>
.cert-detail{display:flex;flex-direction:column;gap:16px}.cert-hero{display:flex;justify-content:space-between;gap:24px;padding:4px 2px 16px;border-bottom:1px solid #e5eaf3}.cert-back{margin-bottom:16px;color:#64748b}.cert-title{display:flex;align-items:center;gap:12px}.cert-title h2{margin:3px 0 4px;font-size:28px}.cert-hero p{margin:4px 0 0;color:#526581}.cert-actions{display:flex;align-items:flex-start;justify-content:flex-end;gap:8px;flex-wrap:wrap}.cert-running{display:grid;grid-template-columns:auto auto minmax(120px,1fr) minmax(240px,2fr);align-items:center;gap:10px;padding:12px 14px;border:1px solid #bfdbfe;border-radius:12px;background:#eff6ff;color:#475569}.cert-pulse{width:9px;height:9px;border-radius:50%;background:#10b981;box-shadow:0 0 0 5px #d1fae5}.cert-grid{display:grid;grid-template-columns:minmax(0,1.5fr) minmax(280px,.75fr);gap:14px}.cert-card{padding:18px;border:1px solid #dce5f2;border-radius:14px;background:#fff}.cert-overview{grid-row:span 2}.cert-card-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:18px}.cert-card-head span{color:#64748b;font-size:12px;letter-spacing:.06em}.cert-card-head h3{margin:4px 0 0;font-size:18px;color:#13233d}.cert-card-head>strong{font-size:26px;color:#047857}.cert-card-head>strong.is-warning{color:#b45309}.cert-meta{display:grid;grid-template-columns:1fr 1fr;gap:18px 24px;margin:0}.cert-meta div,.cert-stack div{min-width:0}.cert-meta dt,.cert-stack dt{margin-bottom:6px;color:#71819b;font-size:12px}.cert-meta dd,.cert-stack dd{margin:0;color:#18243a}.cert-wide{grid-column:1/-1}.cert-stack{display:grid;gap:16px;margin:0 0 16px}.cert-break{overflow-wrap:anywhere}.cert-unit{margin-left:8px;color:#64748b}.cert-tasks{overflow:hidden}.task-progress{display:grid;grid-template-columns:minmax(120px,1fr) 90px;align-items:center;gap:10px}.task-progress small{color:#64748b}.cert-footnote{text-align:right;color:#8492a8;font-size:12px}
@media(max-width:1100px){.cert-hero{flex-direction:column}.cert-actions{justify-content:flex-start}.cert-grid{grid-template-columns:1fr}.cert-overview{grid-row:auto}}
@media(max-width:720px){.cert-actions,.cert-actions .el-button,.cert-actions .el-dropdown{width:100%}.cert-running{grid-template-columns:auto 1fr}.cert-running .el-progress{grid-column:1/-1}.cert-meta{grid-template-columns:1fr}.cert-wide{grid-column:auto}.task-progress{grid-template-columns:1fr}}
</style>
