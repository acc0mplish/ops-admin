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
  PENDING: ['待申请', 'info'], APPLYING: ['申请中', 'warning'], DNS_PENDING: ['待 DNS 验证', 'warning'], VALIDATING: ['验证中', 'warning'], ISSUING: ['签发中', 'warning'], ISSUED: ['已签发', 'success'], NORMAL: ['正常', 'success'], EXPIRING: ['即将过期', 'warning'], EXPIRED: ['已过期', 'danger'], RENEWING: ['续签中', 'warning'], RENEW_FAILED: ['续签失败', 'danger'], APPLY_FAILED: ['申请失败', 'danger'], SYNC_FAILED: ['同步失败', 'danger'], REVOKED: ['已吊销', 'info']
}
const sourceLabels = { ALIYUN: '阿里云同步', TENCENT: '腾讯云同步', MANUAL: '手动上传', ACME: '平台 ACME' }
const typeLabels = { SINGLE: '单域名', WILDCARD: '泛域名', SAN: 'SAN 多域名' }
const taskLabels = { APPLY: '申请证书', RENEW: '续签证书', SYNC: '云端同步', DELETE: '删除证书' }
const stageLabels = { QUEUED: '等待执行', STARTING: '正在启动', PREPARING: '准备申请', DNS_CHALLENGE: '配置 DNS 验证', VALIDATING: '等待 CA 验证', ISSUING: '签发证书', SAVING: '保存证书', CLOUD_SYNC: '同步云端', COMPLETED: '已完成', FAILED: '执行失败' }
const hasActiveTask = computed(() => tasks.value.some(item => ['PENDING', 'RUNNING'].includes(item.status)))

function formatDate(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' }
function providerLabel(value) { return value === 'aliyun' ? '阿里云' : value === 'tencent' ? '腾讯云' : '仅本地' }
function statusLabel(value) { return statusMeta[value]?.[0] || value || '—' }
function statusType(value) { return statusMeta[value]?.[1] || 'info' }
function syncLabel(value) { return ({ SYNCED: '已同步', FAILED: '同步失败', PENDING: '同步中', LOCAL: '仅本地' })[value] || value || '—' }

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
    ElMessage.success('续签策略已保存'); await load()
  } finally { saving.value = false }
}
async function renewNow() {
  await ElMessageBox.confirm('将重新执行 DNS-01 验证并签发新版本，旧证书在成功前仍会保留。', '立即续签', { type: 'warning' })
  await renewSSLCertificate(certificate.value.id); ElMessage.success('续签任务已创建'); await loadTasks(); syncPolling()
}
async function syncCloud() { await resyncSSLCertificate(certificate.value.id); ElMessage.success('云端同步任务已创建'); await loadTasks(); syncPolling() }
async function remove() {
  const deleteCloud = Boolean(certificate.value.cloudCertificateId)
  await ElMessageBox.confirm(`删除前会检查公网解析与证书有效期。${deleteCloud ? '同时尝试删除云端证书；云端删除失败时本地数据会保留。' : ''}`, '删除证书', { type: 'error', confirmButtonText: '确认删除' })
  const result = await deleteSSLCertificate({ id: certificate.value.id, deleteCloud })
  if (result.taskId) { ElMessage.success('删除任务已创建'); await loadTasks(); syncPolling() } else { ElMessage.success('证书已删除'); router.push('/domains/public/certificates') }
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
          <el-button link class="cert-back" @click="router.push('/domains/public/certificates')">← 返回 SSL 证书</el-button>
          <div class="domain-eyebrow">{{ uiT('tlsCertificateDetail') }}</div>
          <div class="cert-title"><h2>{{ certificate.name }}</h2><el-tag :type="statusType(certificate.status)" effect="light">{{ statusLabel(certificate.status) }}</el-tag></div>
          <p class="domain-mono">{{ certificate.domains.join(' · ') }}</p>
        </div>
        <div class="cert-actions">
          <el-dropdown trigger="click"><el-button>下载证书⌄</el-button><template #dropdown><el-dropdown-menu>
            <el-dropdown-item v-permission="'domains:ssl:download'" @click="download('certificate')">{{ uiT('certificatePem') }}</el-dropdown-item>
            <el-dropdown-item v-permission="'domains:ssl:download'" @click="download('chain')">{{ uiT('certificateChain') }}</el-dropdown-item>
            <el-dropdown-item v-if="certificate.hasPrivateKey" v-permission="'domains:ssl:download-key'" divided @click="download('private-key', true)">Private Key（敏感）</el-dropdown-item>
            <el-dropdown-item v-if="certificate.hasPrivateKey" v-permission="'domains:ssl:download-key'" @click="download('zip', true)">完整 ZIP（敏感）</el-dropdown-item>
          </el-dropdown-menu></template></el-dropdown>
          <el-button v-permission="'domains:ssl:sync'" @click="syncCloud">同步到云端</el-button>
          <el-button v-if="certificate.source==='ACME'" v-permission="'domains:ssl:renew'" type="primary" @click="renewNow">立即续签</el-button>
          <el-button v-permission="'domains:ssl:delete'" type="danger" plain @click="remove">删除</el-button>
        </div>
      </div>

      <div v-if="hasActiveTask" class="cert-running"><span class="cert-pulse"></span><strong>{{ taskLabels[tasks[0].taskType] || tasks[0].taskType }}</strong><span>{{ stageLabels[tasks[0].stage] || tasks[0].stage }}</span><el-progress :percentage="tasks[0].progress" :stroke-width="6" /></div>

      <div class="cert-grid">
        <article class="cert-card cert-overview">
          <div class="cert-card-head"><div><span>证书信息</span><h3>身份与有效期</h3></div><strong :class="{'is-warning':certificate.remainingDays<=30}">{{ certificate.remainingDays }} 天</strong></div>
          <dl class="cert-meta">
            <div><dt>主域名</dt><dd class="domain-mono">{{ certificate.mainDomain }}</dd></div><div><dt>证书类型</dt><dd>{{ typeLabels[certificate.type] || certificate.type }}</dd></div>
            <div><dt>来源</dt><dd>{{ sourceLabels[certificate.source] || certificate.source }}</dd></div><div><dt>签发机构</dt><dd>{{ certificate.issuer || '—' }}</dd></div>
            <div><dt>生效时间</dt><dd>{{ formatDate(certificate.notBefore) }}</dd></div><div><dt>到期时间</dt><dd>{{ formatDate(certificate.notAfter) }}</dd></div>
            <div><dt>密钥算法</dt><dd class="domain-mono">{{ certificate.keyAlgorithm || '—' }}</dd></div><div><dt>Private Key</dt><dd><el-tag :type="certificate.hasPrivateKey?'success':'info'" effect="plain">{{ certificate.hasPrivateKey?'已加密保存':'未保存' }}</el-tag></dd></div>
            <div class="cert-wide"><dt>序列号</dt><dd class="domain-mono cert-break">{{ certificate.serialNumber || '—' }}</dd></div><div class="cert-wide"><dt>SHA-256 指纹</dt><dd class="domain-mono cert-break">{{ certificate.fingerprintSha256 || '—' }}</dd></div>
          </dl>
        </article>

        <article class="cert-card">
          <div class="cert-card-head"><div><span>云端副本</span><h3>同步状态</h3></div><el-tag :type="certificate.cloudSyncStatus==='SYNCED'?'success':certificate.cloudSyncStatus==='FAILED'?'danger':'info'" effect="plain">{{ syncLabel(certificate.cloudSyncStatus) }}</el-tag></div>
          <dl class="cert-stack"><div><dt>云服务商</dt><dd>{{ providerLabel(certificate.provider) }}</dd></div><div><dt>DNS / 云账号</dt><dd>{{ certificate.dnsAccountName || '—' }}</dd></div><div><dt>云端证书 ID</dt><dd class="domain-mono cert-break">{{ certificate.cloudCertificateId || '—' }}</dd></div><div><dt>最后同步</dt><dd>{{ formatDate(certificate.lastSyncAt) }}</dd></div></dl>
          <el-alert v-if="certificate.lastSyncError" :title="certificate.lastSyncError" type="error" :closable="false" show-icon />
        </article>

        <article class="cert-card">
          <div class="cert-card-head"><div><span>续签策略</span><h3>自动续签</h3></div><el-switch v-model="renewForm.autoRenew" :disabled="certificate.source!=='ACME'" /></div>
          <el-form label-position="top"><el-form-item label="到期前多少天开始续签"><el-input-number v-model="renewForm.renewBeforeDays" :min="1" :max="90" :disabled="certificate.source!=='ACME'" /><span class="cert-unit">天</span></el-form-item><el-form-item label="ACME CA"><span class="domain-mono">{{ certificate.acmeCa || '—' }}</span></el-form-item><el-button v-if="certificate.source==='ACME'" v-permission="'domains:ssl:settings'" type="primary" :loading="saving" @click="saveRenewSettings">保存续签策略</el-button><div v-else class="domain-form-tip">云同步和手工上传证书不由平台自动续签。</div></el-form>
          <el-alert v-if="certificate.lastRenewError" :title="certificate.lastRenewError" type="error" :closable="false" show-icon />
        </article>
      </div>

      <article class="cert-card cert-tasks">
        <div class="cert-card-head"><div><span>执行记录</span><h3>证书任务</h3></div><el-button link @click="loadTasks">刷新</el-button></div>
        <el-table :data="tasks" border><el-table-column label="任务" width="130"><template #default="{row}">{{ taskLabels[row.taskType] || row.taskType }}</template></el-table-column><el-table-column label="进度" min-width="210"><template #default="{row}"><div class="task-progress"><el-progress :percentage="row.progress" :status="row.status==='FAILED'?'exception':row.status==='SUCCESS'?'success':''" /><small>{{ stageLabels[row.stage] || row.stage }}</small></div></template></el-table-column><el-table-column prop="username" label="操作人" width="120"/><el-table-column label="开始时间" width="180"><template #default="{row}">{{ formatDate(row.startedAt || row.createdAt) }}</template></el-table-column><el-table-column label="完成时间" width="180"><template #default="{row}">{{ formatDate(row.finishedAt) }}</template></el-table-column><el-table-column prop="errorMessage" label="失败原因" min-width="220" show-overflow-tooltip /></el-table>
      </article>

      <div class="cert-footnote">创建于 {{ formatDate(certificate.createdAt) }} · 最后更新 {{ formatDate(certificate.updatedAt) }} · 创建人 {{ certificate.createdBy || '—' }}</div>
    </template>
  </section>
</template>

<style scoped>
.cert-detail{display:flex;flex-direction:column;gap:16px}.cert-hero{display:flex;justify-content:space-between;gap:24px;padding:4px 2px 16px;border-bottom:1px solid #e5eaf3}.cert-back{margin-bottom:16px;color:#64748b}.cert-title{display:flex;align-items:center;gap:12px}.cert-title h2{margin:3px 0 4px;font-size:28px}.cert-hero p{margin:4px 0 0;color:#526581}.cert-actions{display:flex;align-items:flex-start;justify-content:flex-end;gap:8px;flex-wrap:wrap}.cert-running{display:grid;grid-template-columns:auto auto minmax(120px,1fr) minmax(240px,2fr);align-items:center;gap:10px;padding:12px 14px;border:1px solid #bfdbfe;border-radius:12px;background:#eff6ff;color:#475569}.cert-pulse{width:9px;height:9px;border-radius:50%;background:#10b981;box-shadow:0 0 0 5px #d1fae5}.cert-grid{display:grid;grid-template-columns:minmax(0,1.5fr) minmax(280px,.75fr);gap:14px}.cert-card{padding:18px;border:1px solid #dce5f2;border-radius:14px;background:#fff}.cert-overview{grid-row:span 2}.cert-card-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:18px}.cert-card-head span{color:#64748b;font-size:12px;letter-spacing:.06em}.cert-card-head h3{margin:4px 0 0;font-size:18px;color:#13233d}.cert-card-head>strong{font-size:26px;color:#047857}.cert-card-head>strong.is-warning{color:#b45309}.cert-meta{display:grid;grid-template-columns:1fr 1fr;gap:18px 24px;margin:0}.cert-meta div,.cert-stack div{min-width:0}.cert-meta dt,.cert-stack dt{margin-bottom:6px;color:#71819b;font-size:12px}.cert-meta dd,.cert-stack dd{margin:0;color:#18243a}.cert-wide{grid-column:1/-1}.cert-stack{display:grid;gap:16px;margin:0 0 16px}.cert-break{overflow-wrap:anywhere}.cert-unit{margin-left:8px;color:#64748b}.cert-tasks{overflow:hidden}.task-progress{display:grid;grid-template-columns:minmax(120px,1fr) 90px;align-items:center;gap:10px}.task-progress small{color:#64748b}.cert-footnote{text-align:right;color:#8492a8;font-size:12px}
@media(max-width:1100px){.cert-hero{flex-direction:column}.cert-actions{justify-content:flex-start}.cert-grid{grid-template-columns:1fr}.cert-overview{grid-row:auto}}
@media(max-width:720px){.cert-actions,.cert-actions .el-button,.cert-actions .el-dropdown{width:100%}.cert-running{grid-template-columns:auto 1fr}.cert-running .el-progress{grid-column:1/-1}.cert-meta{grid-template-columns:1fr}.cert-wide{grid-column:auto}.task-progress{grid-template-columns:1fr}}
</style>
