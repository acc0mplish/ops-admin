<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  applySSLCertificate,
  deleteSSLCertificate,
  queryDNSAccountOptions,
  querySSLCertificateDomainOptions,
  querySSLCertificates,
  querySSLCertificateTasks,
  renewSSLCertificate,
  resyncSSLCertificate,
  syncSSLCertificates,
  uploadSSLCertificate
} from '../../api/domain'

const router = useRouter()
const loading = ref(false), submitting = ref(false), syncing = ref(false)
const rows = ref([]), total = ref(0), accounts = ref([]), domainOptions = ref([]), tasks = ref([])
const applyVisible = ref(false), uploadVisible = ref(false), syncVisible = ref(false), deleteVisible = ref(false)
const deleteTarget = ref(null)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '', source: '', accountId: '' })
const stats = reactive({ total: 0, valid: 0, expiring: 0, expired: 0, autoRenew: 0 })
const applyForm = reactive({ name: '', mainDomain: '', type: 'SINGLE', certificateDomain: '', includeRootDomain: false, autoRenew: true, renewBeforeDays: 30, acmeEnvironment: 'staging', acmeEmail: '' })
const uploadForm = reactive({ name: '', mainDomain: '', certificatePem: '', privateKeyPem: '', certificateChain: '', syncToCloud: false, renewBeforeDays: 30 })
const syncAccountId = ref(''), deleteCloud = ref(true)
let pollTimer

const statusMeta = {
  PENDING: ['待申请', 'info'], APPLYING: ['申请中', 'warning'], DNS_PENDING: ['待 DNS 验证', 'warning'], VALIDATING: ['验证中', 'warning'], ISSUING: ['签发中', 'warning'], ISSUED: ['已签发', 'success'], NORMAL: ['正常', 'success'], EXPIRING: ['即将过期', 'warning'], EXPIRED: ['已过期', 'danger'], RENEWING: ['续签中', 'warning'], RENEW_FAILED: ['续签失败', 'danger'], APPLY_FAILED: ['申请失败', 'danger'], SYNC_FAILED: ['同步失败', 'danger'], REVOKED: ['已吊销', 'info']
}
const sourceLabels = { ALIYUN: '阿里云同步', TENCENT: '腾讯云同步', MANUAL: '手动上传', ACME: '平台申请' }
const typeLabels = { SINGLE: '单域名', WILDCARD: '泛域名', SAN: 'SAN 多域名' }
const activeTasks = computed(() => tasks.value.filter((item) => ['PENDING', 'RUNNING'].includes(item.status)))

function statusLabel(value) { return statusMeta[value]?.[0] || value || '-' }
function statusType(value) { return statusMeta[value]?.[1] || 'info' }
function providerLabel(value) { return value === 'aliyun' ? '阿里云' : value === 'tencent' ? '腾讯云' : '-' }
function formatDate(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-' }

async function load() {
  loading.value = true
  try {
    const data = await querySSLCertificates(query)
    rows.value = data.list || []; total.value = data.total || 0; Object.assign(stats, data.stats || {})
  } finally { loading.value = false }
}

async function loadTasks() { tasks.value = await querySSLCertificateTasks({ limit: 20 }) || []; if (!activeTasks.value.length && pollTimer) { clearInterval(pollTimer); pollTimer = null; await load() } }
function startPolling() { loadTasks(); if (!pollTimer) pollTimer = setInterval(loadTasks, 3000) }

function resetApply() { Object.assign(applyForm, { name: '', mainDomain: '', type: 'SINGLE', certificateDomain: '', includeRootDomain: false, autoRenew: true, renewBeforeDays: 30, acmeEnvironment: 'staging', acmeEmail: '' }); applyVisible.value = true }
function resetUpload() { Object.assign(uploadForm, { name: '', mainDomain: '', certificatePem: '', privateKeyPem: '', certificateChain: '', syncToCloud: false, renewBeforeDays: 30 }); uploadVisible.value = true }
function selectedDomainInfo(domain) { return domainOptions.value.find((item) => item.domain === domain) || {} }

watch(() => applyForm.mainDomain, (value) => { if (!value) return; applyForm.certificateDomain = applyForm.type === 'WILDCARD' ? `*.${value}` : value; if (!applyForm.name) applyForm.name = value })
watch(() => applyForm.type, (value) => { if (applyForm.mainDomain) applyForm.certificateDomain = value === 'WILDCARD' ? `*.${applyForm.mainDomain}` : applyForm.mainDomain; if (value !== 'WILDCARD') applyForm.includeRootDomain = false })

async function submitApply() {
  if (!applyForm.mainDomain || !applyForm.certificateDomain || !applyForm.name.trim()) { ElMessage.warning('请填写完整的申请信息'); return }
  submitting.value = true
  try { await applySSLCertificate(applyForm); ElMessage.success('证书申请任务已创建'); applyVisible.value = false; await load(); startPolling() } finally { submitting.value = false }
}

async function submitUpload() {
  if (!uploadForm.name.trim() || !uploadForm.mainDomain || !uploadForm.certificatePem.trim() || !uploadForm.privateKeyPem.trim()) { ElMessage.warning('请填写证书名称、主域名、Certificate 和 Private Key'); return }
  submitting.value = true
  try { await uploadSSLCertificate(uploadForm); ElMessage.success('证书已校验并安全保存'); uploadVisible.value = false; await load(); if (uploadForm.syncToCloud) startPolling() } finally { submitting.value = false }
}

async function submitSync() { syncing.value = true; try { await syncSSLCertificates(Number(syncAccountId.value) || 0); ElMessage.success('云证书同步任务已创建'); syncVisible.value = false; startPolling() } finally { syncing.value = false } }
async function renew(row) { await ElMessageBox.confirm(`将为 ${row.name} 重新执行 DNS-01 验证并签发新证书，确认继续？`, '立即续签', { type: 'warning' }); await renewSSLCertificate(row.id); ElMessage.success('续签任务已创建'); startPolling() }
async function resync(row) { await resyncSSLCertificate(row.id); ElMessage.success('云端同步任务已创建'); startPolling() }
function openDelete(row) { deleteTarget.value = row; deleteCloud.value = Boolean(row.cloudCertificateId); deleteVisible.value = true }
async function submitDelete() { submitting.value = true; try { const data = await deleteSSLCertificate({ id: deleteTarget.value.id, deleteCloud: deleteCloud.value }); deleteVisible.value = false; if (data.taskId) { ElMessage.success('证书删除任务已创建'); startPolling() } else { ElMessage.success('证书已删除'); await load() } } finally { submitting.value = false } }
async function command({ command, row }) { if (command === 'renew') await renew(row); if (command === 'sync') await resync(row); if (command === 'delete') openDelete(row) }

onMounted(async () => { const [accountList, domains] = await Promise.all([queryDNSAccountOptions(), querySSLCertificateDomainOptions()]); accounts.value = accountList || []; domainOptions.value = domains || []; await Promise.all([load(), loadTasks()]); if (activeTasks.value.length) startPolling() })
onBeforeUnmount(() => { if (pollTimer) clearInterval(pollTimer) })
</script>

<template>
  <section class="domain-page domain-panel page-card ssl-page">
    <div class="domain-page-head">
      <div><div class="domain-eyebrow">TLS CERTIFICATE INVENTORY</div><h2>SSL 证书</h2><p>统一管理云端证书与平台 ACME 证书。Private Key 加密保存，普通查询接口永不返回密钥内容。</p></div>
      <div class="ssl-head-actions">
        <el-button v-permission="'domains:ssl:sync'" @click="syncVisible=true">从云端同步</el-button>
        <el-button v-permission="'domains:ssl:apply'" @click="resetApply">申请证书</el-button>
        <el-button v-permission="'domains:ssl:upload'" type="primary" @click="resetUpload">上传证书</el-button>
      </div>
    </div>

    <div v-if="activeTasks.length" class="ssl-task-strip">
      <div><strong>{{ activeTasks.length }} 个证书任务正在执行</strong><span>{{ activeTasks[0].stage }} · {{ activeTasks[0].progress }}%</span></div>
      <el-progress :percentage="activeTasks[0].progress" :stroke-width="6" :show-text="false" />
    </div>

    <div class="ssl-stat-grid">
      <article class="domain-stat"><div class="domain-stat__label">证书总数</div><div class="domain-stat__value">{{ stats.total }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">有效证书</div><div class="domain-stat__value is-valid">{{ stats.valid }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">即将过期</div><div class="domain-stat__value is-warning">{{ stats.expiring }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">已过期</div><div class="domain-stat__value is-danger">{{ stats.expired }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">自动续签</div><div class="domain-stat__value">{{ stats.autoRenew }}</div></article>
    </div>

    <div class="domain-toolbar" role="search"><div class="domain-toolbar__filters">
      <el-input v-model="query.keyword" clearable placeholder="搜索证书名称 / 主域名" style="width:240px" @keyup.enter="load" />
      <el-select v-model="query.status" clearable placeholder="证书状态" style="width:150px"><el-option v-for="(meta,key) in statusMeta" :key="key" :label="meta[0]" :value="key" /></el-select>
      <el-select v-model="query.source" clearable placeholder="证书来源" style="width:150px"><el-option v-for="(label,key) in sourceLabels" :key="key" :label="label" :value="key" /></el-select>
      <el-select v-model="query.accountId" clearable placeholder="云账号" style="width:170px"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select>
      <el-button @click="load">查询</el-button>
    </div></div>

    <div class="domain-table-wrap ssl-table-wrap"><el-table v-loading="loading" :data="rows" border @row-dblclick="row=>router.push(`/domains/public/certificates/${row.id}`)">
      <el-table-column prop="name" label="证书名称" min-width="180" fixed="left"><template #default="{row}"><el-button link type="primary" @click="router.push(`/domains/public/certificates/${row.id}`)">{{ row.name }}</el-button></template></el-table-column>
      <el-table-column prop="mainDomain" label="主域名" min-width="160" /><el-table-column label="证书域名" min-width="220"><template #default="{row}"><span class="domain-mono">{{ row.domains.join(', ') }}</span></template></el-table-column>
      <el-table-column label="类型" width="120"><template #default="{row}">{{ typeLabels[row.type] || row.type }}</template></el-table-column><el-table-column label="来源" width="120"><template #default="{row}">{{ sourceLabels[row.source] || row.source }}</template></el-table-column>
      <el-table-column label="服务商" width="110"><template #default="{row}">{{ providerLabel(row.provider) }}</template></el-table-column><el-table-column prop="dnsAccountName" label="云账号" min-width="140" /><el-table-column prop="issuer" label="签发机构" min-width="180" show-overflow-tooltip />
      <el-table-column label="生效时间" width="170"><template #default="{row}">{{ formatDate(row.notBefore) }}</template></el-table-column><el-table-column label="到期时间" width="170"><template #default="{row}">{{ formatDate(row.notAfter) }}</template></el-table-column>
      <el-table-column label="剩余天数" width="100"><template #default="{row}"><strong :class="{'ssl-days-warning':row.remainingDays<=30}">{{ row.remainingDays }} 天</strong></template></el-table-column><el-table-column label="自动续签" width="100"><template #default="{row}"><el-tag :type="row.autoRenew?'success':'info'" effect="plain">{{ row.autoRenew?'开启':'关闭' }}</el-tag></template></el-table-column>
      <el-table-column label="云端同步" width="110"><template #default="{row}"><el-tag :type="row.cloudSyncStatus==='SYNCED'?'success':row.cloudSyncStatus==='FAILED'?'danger':'info'" effect="plain">{{ {SYNCED:'已同步',FAILED:'失败',PENDING:'同步中',LOCAL:'仅本地'}[row.cloudSyncStatus] }}</el-tag></template></el-table-column>
      <el-table-column label="证书状态" width="120"><template #default="{row}"><el-tag :type="statusType(row.status)" effect="light">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="170" fixed="right"><template #default="{row}"><el-button link type="primary" @click="router.push(`/domains/public/certificates/${row.id}`)">详情</el-button><el-dropdown trigger="click" @command="value=>command({command:value,row})"><el-button link>更多⌄</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item v-if="row.source==='ACME'" command="renew">立即续签</el-dropdown-item><el-dropdown-item command="sync">同步到云端</el-dropdown-item><el-dropdown-item divided command="delete" class="batch-danger-item">删除证书</el-dropdown-item></el-dropdown-menu></template></el-dropdown></template></el-table-column>
      <template #empty><div class="domain-empty"><strong>暂无 SSL 证书</strong><p>可以从云厂商同步、使用 ACME 申请，或上传已有 PEM 证书。</p><el-button type="primary" @click="resetUpload">上传证书</el-button></div></template>
    </el-table></div>
    <div class="domain-pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :total="total" @current-change="load" @size-change="load" /></div>

    <el-dialog v-model="syncVisible" title="从云端同步证书" width="min(520px, calc(100vw - 32px))"><el-form label-position="top"><el-form-item label="同步账号"><el-select v-model="syncAccountId" clearable placeholder="全部启用账号" style="width:100%"><el-option v-for="item in accounts" :key="item.id" :label="`${item.name} · ${providerLabel(item.provider)}`" :value="item.id" /></el-select></el-form-item><div class="domain-form-tip">同步只读取证书列表和元数据，不会为了同步列表下载云端 Private Key。</div></el-form><template #footer><el-button @click="syncVisible=false">取消</el-button><el-button type="primary" :loading="syncing" @click="submitSync">创建同步任务</el-button></template></el-dialog>

    <el-dialog v-model="applyVisible" title="申请 SSL 证书" width="min(680px, calc(100vw - 32px))"><el-alert v-if="!domainOptions.length" title="请先在公网域名页面同步域名，并确保域名已关联启用的 DNS 云账号。" type="warning" :closable="false" show-icon class="ssl-prerequisite"/><el-form label-width="116px">
      <el-form-item label="证书名称" required><el-input v-model="applyForm.name" /></el-form-item><el-form-item label="主域名" required><el-select v-model="applyForm.mainDomain" filterable :placeholder="domainOptions.length?'请选择公网主域名':'暂无可申请的公网域名'" style="width:100%"><el-option v-for="item in domainOptions" :key="`${item.accountId}-${item.domain}`" :label="item.domain" :value="item.domain" /></el-select></el-form-item>
      <el-form-item label="证书类型" required><el-radio-group v-model="applyForm.type"><el-radio-button value="SINGLE">单域名</el-radio-button><el-radio-button value="WILDCARD">泛域名</el-radio-button></el-radio-group></el-form-item><el-form-item label="证书域名" required><el-input v-model="applyForm.certificateDomain" class="domain-mono" /></el-form-item>
      <el-form-item v-if="applyForm.type==='WILDCARD'" label="根域名"><el-checkbox v-model="applyForm.includeRootDomain">同时包含 {{ applyForm.mainDomain || 'example.com' }}</el-checkbox></el-form-item><el-form-item label="自动续签"><el-switch v-model="applyForm.autoRenew" /></el-form-item><el-form-item label="提前续签"><el-input-number v-model="applyForm.renewBeforeDays" :min="1" :max="90" /> <span class="ssl-unit">天</span></el-form-item>
      <el-form-item label="ACME 环境"><el-radio-group v-model="applyForm.acmeEnvironment"><el-radio value="staging">测试环境</el-radio><el-radio value="production">生产环境</el-radio></el-radio-group><div class="domain-form-tip">建议先使用测试环境验证 DNS-01 流程，避免触发生产 CA 频率限制。</div></el-form-item><el-form-item label="联系邮箱"><el-input v-model="applyForm.acmeEmail" placeholder="留空时使用服务端 ACME 配置" /></el-form-item>
      <div v-if="applyForm.mainDomain" class="ssl-provider-summary"><span>DNS Provider</span><strong>{{ providerLabel(selectedDomainInfo(applyForm.mainDomain).provider) }}</strong><span>DNS Account</span><strong>{{ selectedDomainInfo(applyForm.mainDomain).accountName }}</strong><span>验证方式</span><strong>DNS-01</strong></div>
    </el-form><template #footer><el-button @click="applyVisible=false">取消</el-button><el-button type="primary" :loading="submitting" :disabled="!domainOptions.length" @click="submitApply">创建申请任务</el-button></template></el-dialog>

    <el-dialog v-model="uploadVisible" title="上传 SSL 证书" width="min(820px, calc(100vw - 32px))" top="5vh"><el-alert v-if="!domainOptions.length" title="请先同步公网域名；证书必须归属平台已管理的公网主域名。" type="warning" :closable="false" show-icon class="ssl-prerequisite"/><el-form label-position="top" class="ssl-upload-form"><div class="ssl-upload-grid"><el-form-item label="证书名称" required><el-input v-model="uploadForm.name" /></el-form-item><el-form-item label="平台公网主域名" required><el-select v-model="uploadForm.mainDomain" filterable :placeholder="domainOptions.length?'请选择公网主域名':'暂无已管理的公网域名'" style="width:100%"><el-option v-for="item in domainOptions" :key="`${item.accountId}-${item.domain}`" :label="item.domain" :value="item.domain" /></el-select></el-form-item></div><el-form-item label="Certificate PEM" required><el-input v-model="uploadForm.certificatePem" type="textarea" :rows="7" class="domain-mono" placeholder="-----BEGIN CERTIFICATE-----" /></el-form-item><el-form-item label="Private Key PEM" required><el-input v-model="uploadForm.privateKeyPem" type="textarea" :rows="7" class="domain-mono" placeholder="-----BEGIN PRIVATE KEY-----" /><div class="domain-form-tip">后端会校验证书与私钥是否匹配；Private Key 使用 AES-256-GCM 加密后存储。</div></el-form-item><el-form-item label="Certificate Chain / CA Bundle"><el-input v-model="uploadForm.certificateChain" type="textarea" :rows="5" class="domain-mono" /></el-form-item><el-checkbox v-model="uploadForm.syncToCloud">校验保存后同步到主域名对应的云账号</el-checkbox></el-form><template #footer><el-button @click="uploadVisible=false">取消</el-button><el-button type="primary" :loading="submitting" :disabled="!domainOptions.length" @click="submitUpload">校验并保存</el-button></template></el-dialog>

    <el-dialog v-model="deleteVisible" title="删除 SSL 证书" width="min(560px, calc(100vw - 32px))"><div class="ssl-delete-warning"><strong>删除前将执行安全检查</strong><p>未过期单域名证书会实时查询公网 DNS；仍有解析记录时禁止删除。未过期泛域名证书始终禁止删除。</p></div><el-checkbox v-if="deleteTarget?.cloudCertificateId" v-model="deleteCloud">同时删除 {{ providerLabel(deleteTarget?.provider) }} 云端证书</el-checkbox><template #footer><el-button @click="deleteVisible=false">取消</el-button><el-button type="danger" :loading="submitting" @click="submitDelete">确认删除</el-button></template></el-dialog>
  </section>
</template>

<style scoped>
.ssl-head-actions { display:flex; gap:8px; flex-wrap:wrap; }
.ssl-stat-grid { display:grid; grid-template-columns:repeat(5,minmax(0,1fr)); gap:12px; }
.domain-stat__value.is-valid { color:#047857; }.domain-stat__value.is-warning,.ssl-days-warning { color:#b45309; }.domain-stat__value.is-danger { color:#b91c1c; }
.ssl-task-strip { display:grid; grid-template-columns:minmax(260px,1fr) minmax(240px,2fr); align-items:center; gap:20px; padding:12px 14px; border:1px solid #bfdbfe; border-radius:12px; background:#eff6ff; }.ssl-task-strip div { display:flex; gap:10px; align-items:center; }.ssl-task-strip strong { color:#1d4ed8; }.ssl-task-strip span { color:#64748b; font-size:12px; }
.ssl-table-wrap .el-table { width:100%; }.ssl-unit { margin-left:8px; color:#64748b; }.ssl-provider-summary { display:grid; grid-template-columns:110px 1fr; gap:8px 16px; margin-left:116px; padding:14px; border:1px solid #dbeafe; border-radius:10px; background:#f8faff; }.ssl-provider-summary span { color:#64748b; }.ssl-provider-summary strong { color:#0f172a; }.ssl-upload-grid { display:grid; grid-template-columns:1fr 1fr; gap:14px; }.ssl-delete-warning { margin-bottom:16px; padding:14px; border:1px solid #fecaca; border-radius:10px; background:#fef2f2; }.ssl-delete-warning strong { color:#991b1b; }.ssl-delete-warning p { margin:6px 0 0; color:#b91c1c; line-height:1.6; }
.ssl-prerequisite { margin-bottom:18px; }
:global(.batch-danger-item:not(.is-disabled)) { color:#dc2626; }
@media(max-width:1000px){.ssl-stat-grid{grid-template-columns:repeat(2,minmax(0,1fr));}.ssl-task-strip{grid-template-columns:1fr;}.ssl-provider-summary{margin-left:0;}}
@media(max-width:640px){.ssl-head-actions,.ssl-head-actions .el-button{width:100%;}.ssl-stat-grid,.ssl-upload-grid{grid-template-columns:1fr;}}
</style>
