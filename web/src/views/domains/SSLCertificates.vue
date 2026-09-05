<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
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
import { dt } from '../../utils/domain-i18n'

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
  PENDING: [dt('sslStatusPending'), 'info'], APPLYING: [dt('sslStatusApplying'), 'warning'], DNS_PENDING: [dt('sslStatusDnsPending'), 'warning'], VALIDATING: [dt('sslStatusValidating'), 'warning'], ISSUING: [dt('sslStatusIssuing'), 'warning'], ISSUED: [dt('sslStatusIssued'), 'success'], NORMAL: [dt('sslStatusNormal'), 'success'], EXPIRING: [dt('sslStatusExpiring'), 'warning'], EXPIRED: [dt('sslStatusExpired'), 'danger'], RENEWING: [dt('sslStatusRenewing'), 'warning'], RENEW_FAILED: [dt('sslStatusRenewFailed'), 'danger'], APPLY_FAILED: [dt('sslStatusApplyFailed'), 'danger'], SYNC_FAILED: [dt('sslStatusSyncFailed'), 'danger'], REVOKED: [dt('sslStatusRevoked'), 'info']
}
const sourceLabels = { ALIYUN: dt('sourceAliyunSync'), TENCENT: dt('sourceTencentSync'), MANUAL: dt('sourceManualUpload'), ACME: dt('sourcePlatformIssued') }
const typeLabels = { SINGLE: dt('certTypeSingle'), WILDCARD: dt('certTypeWildcard'), SAN: dt('certTypeSan') }
const activeTasks = computed(() => tasks.value.filter((item) => ['PENDING', 'RUNNING'].includes(item.status)))

function statusLabel(value) { return statusMeta[value]?.[0] || value || '-' }
function statusType(value) { return statusMeta[value]?.[1] || 'info' }
function providerLabel(value) { return value === 'aliyun' ? 'Alibaba Cloud' : value === 'tencent' ? 'Tencent Cloud' : '-' }
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
  if (!applyForm.mainDomain || !applyForm.certificateDomain || !applyForm.name.trim()) { ElMessage.warning(dt('warnApplyFields')); return }
  submitting.value = true
  try { await applySSLCertificate(applyForm); ElMessage.success(dt('applyTaskCreated')); applyVisible.value = false; await load(); startPolling() } finally { submitting.value = false }
}

async function submitUpload() {
  if (!uploadForm.name.trim() || !uploadForm.mainDomain || !uploadForm.certificatePem.trim() || !uploadForm.privateKeyPem.trim()) { ElMessage.warning(dt('warnUploadFields')); return }
  submitting.value = true
  try { await uploadSSLCertificate(uploadForm); ElMessage.success(dt('certVerifiedSaved')); uploadVisible.value = false; await load(); if (uploadForm.syncToCloud) startPolling() } finally { submitting.value = false }
}

async function submitSync() { syncing.value = true; try { await syncSSLCertificates(Number(syncAccountId.value) || 0); ElMessage.success(dt('cloudSyncTaskCreated')); syncVisible.value = false; startPolling() } finally { syncing.value = false } }
async function renew(row) { await ElMessageBox.confirm(dt('renewConfirm', { name: row.name }), dt('renewNow'), { type: 'warning' }); await renewSSLCertificate(row.id); ElMessage.success(dt('renewTaskCreated')); startPolling() }
async function resync(row) { await resyncSSLCertificate(row.id); ElMessage.success(dt('resyncTaskCreated')); startPolling() }
function openDelete(row) { deleteTarget.value = row; deleteCloud.value = Boolean(row.cloudCertificateId); deleteVisible.value = true }
async function submitDelete() { submitting.value = true; try { const data = await deleteSSLCertificate({ id: deleteTarget.value.id, deleteCloud: deleteCloud.value }); deleteVisible.value = false; if (data.taskId) { ElMessage.success(dt('deleteTaskCreated')); startPolling() } else { ElMessage.success(dt('certDeleted')); await load() } } finally { submitting.value = false } }
async function command({ command, row }) { if (command === 'renew') await renew(row); if (command === 'sync') await resync(row); if (command === 'delete') openDelete(row) }

onMounted(async () => { const [accountList, domains] = await Promise.all([queryDNSAccountOptions(), querySSLCertificateDomainOptions()]); accounts.value = accountList || []; domainOptions.value = domains || []; await Promise.all([load(), loadTasks()]); if (activeTasks.value.length) startPolling() })
onBeforeUnmount(() => { if (pollTimer) clearInterval(pollTimer) })
</script>

<template>
  <section class="domain-page domain-panel page-card ssl-page">
    <div class="domain-page-head">
      <div><div class="domain-eyebrow">{{ uiT('tlsCertificateInventory') }}</div><h2>{{ dt('sslPageTitle') }}</h2><p>{{ dt('sslPageDesc') }}</p></div>
      <div class="ssl-head-actions">
        <el-button v-permission="'domains:ssl:sync'" @click="syncVisible=true">{{ dt('syncFromCloud') }}</el-button>
        <el-button v-permission="'domains:ssl:apply'" @click="resetApply">{{ dt('applyCert') }}</el-button>
        <el-button v-permission="'domains:ssl:upload'" type="primary" @click="resetUpload">{{ dt('uploadCert') }}</el-button>
      </div>
    </div>

    <div v-if="activeTasks.length" class="ssl-task-strip">
      <div><strong>{{ dt('tasksRunning', { count: activeTasks.length }) }}</strong><span>{{ activeTasks[0].stage }} · {{ activeTasks[0].progress }}%</span></div>
      <el-progress :percentage="activeTasks[0].progress" :stroke-width="6" :show-text="false" />
    </div>

    <div class="ssl-stat-grid">
      <article class="domain-stat"><div class="domain-stat__label">{{ dt('sslStatTotal') }}</div><div class="domain-stat__value">{{ stats.total }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">{{ dt('sslStatValid') }}</div><div class="domain-stat__value is-valid">{{ stats.valid }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">{{ dt('sslStatusExpiring') }}</div><div class="domain-stat__value is-warning">{{ stats.expiring }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">{{ dt('sslStatusExpired') }}</div><div class="domain-stat__value is-danger">{{ stats.expired }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">{{ dt('autoRenewLabel') }}</div><div class="domain-stat__value">{{ stats.autoRenew }}</div></article>
    </div>

    <div class="domain-toolbar" role="search"><div class="domain-toolbar__filters">
      <el-input v-model="query.keyword" clearable :placeholder="dt('sslSearchPlaceholder')" style="width:240px" @keyup.enter="load" />
      <el-select v-model="query.status" clearable :placeholder="dt('certStatusFilter')" style="width:150px"><el-option v-for="(meta,key) in statusMeta" :key="key" :label="meta[0]" :value="key" /></el-select>
      <el-select v-model="query.source" clearable :placeholder="dt('certSourceFilter')" style="width:150px"><el-option v-for="(label,key) in sourceLabels" :key="key" :label="label" :value="key" /></el-select>
      <el-select v-model="query.accountId" clearable placeholder="Cloud Account" style="width:170px"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select>
      <el-button @click="load">{{ dt('queryBtn') }}</el-button>
    </div></div>

    <div class="domain-table-wrap ssl-table-wrap"><el-table v-loading="loading" :data="rows" border @row-dblclick="row=>router.push(`/domains/public/certificates/${row.id}`)">
      <el-table-column prop="name" :label="dt('certName')" min-width="180" fixed="left"><template #default="{row}"><el-button link type="primary" @click="router.push(`/domains/public/certificates/${row.id}`)">{{ row.name }}</el-button></template></el-table-column>
      <el-table-column prop="mainDomain" label="Main Domain" min-width="160" /><el-table-column :label="dt('certDomains')" min-width="220"><template #default="{row}"><span class="domain-mono">{{ row.domains.join(', ') }}</span></template></el-table-column>
      <el-table-column :label="dt('type')" width="120"><template #default="{row}">{{ typeLabels[row.type] || row.type }}</template></el-table-column><el-table-column label="Source" width="120"><template #default="{row}">{{ sourceLabels[row.source] || row.source }}</template></el-table-column>
      <el-table-column label="Provider" width="110"><template #default="{row}">{{ providerLabel(row.provider) }}</template></el-table-column><el-table-column prop="dnsAccountName" label="Cloud Account" min-width="140" /><el-table-column prop="issuer" :label="dt('issuer')" min-width="180" show-overflow-tooltip />
      <el-table-column :label="dt('notBeforeLabel')" width="170"><template #default="{row}">{{ formatDate(row.notBefore) }}</template></el-table-column><el-table-column :label="dt('notAfterLabel')" width="170"><template #default="{row}">{{ formatDate(row.notAfter) }}</template></el-table-column>
      <el-table-column :label="dt('remainingDaysLabel')" width="100"><template #default="{row}"><strong :class="{'ssl-days-warning':row.remainingDays<=30}">{{ row.remainingDays }} {{ dt('dayUnit') }}</strong></template></el-table-column><el-table-column :label="dt('autoRenewLabel')" width="100"><template #default="{row}"><el-tag :type="row.autoRenew?'success':'info'" effect="plain">{{ row.autoRenew?dt('onLabel'):dt('offLabel') }}</el-tag></template></el-table-column>
      <el-table-column :label="dt('cloudSync')" width="110"><template #default="{row}"><el-tag :type="row.cloudSyncStatus==='SYNCED'?'success':row.cloudSyncStatus==='FAILED'?'danger':'info'" effect="plain">{{ {SYNCED:dt('syncStateSynced'),FAILED:dt('syncStateFailedList'),PENDING:dt('syncStatePending'),LOCAL:dt('syncStateLocal')}[row.cloudSyncStatus] }}</el-tag></template></el-table-column>
      <el-table-column :label="dt('certStatusFilter')" width="120"><template #default="{row}"><el-tag :type="statusType(row.status)" effect="light">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
      <el-table-column :label="dt('actions')" width="170" fixed="right"><template #default="{row}"><el-button link type="primary" @click="router.push(`/domains/public/certificates/${row.id}`)">{{ dt('detail') }}</el-button><el-dropdown trigger="click" @command="value=>command({command:value,row})"><el-button link>{{ dt('more') }}</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item v-if="row.source==='ACME'" command="renew">{{ dt('renewNow') }}</el-dropdown-item><el-dropdown-item command="sync">{{ dt('syncToCloud') }}</el-dropdown-item><el-dropdown-item divided command="delete" class="batch-danger-item">{{ dt('deleteCert') }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown></template></el-table-column>
      <template #empty><div class="domain-empty"><strong>{{ dt('sslEmptyTitle') }}</strong><p>{{ dt('sslEmptyDesc') }}</p><el-button type="primary" @click="resetUpload">{{ dt('uploadCert') }}</el-button></div></template>
    </el-table></div>
    <div class="domain-pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :total="total" @current-change="load" @size-change="load" /></div>

    <el-dialog v-model="syncVisible" :title="dt('syncDialogTitle')" width="min(520px, calc(100vw - 32px))"><el-form label-position="top"><el-form-item :label="dt('syncAccountLabel')"><el-select v-model="syncAccountId" clearable :placeholder="dt('allActiveAccounts')" style="width:100%"><el-option v-for="item in accounts" :key="item.id" :label="`${item.name} · ${providerLabel(item.provider)}`" :value="item.id" /></el-select></el-form-item><div class="domain-form-tip">{{ dt('syncTip') }}</div></el-form><template #footer><el-button @click="syncVisible=false">{{ dt('cancel') }}</el-button><el-button type="primary" :loading="syncing" @click="submitSync">{{ dt('createSyncTask') }}</el-button></template></el-dialog>

    <el-dialog v-model="applyVisible" :title="dt('applyDialogTitle')" width="min(680px, calc(100vw - 32px))"><el-alert v-if="!domainOptions.length" :title="dt('applyPrerequisite')" type="warning" :closable="false" show-icon class="ssl-prerequisite"/><el-form label-width="116px">
      <el-form-item :label="dt('certName')" required><el-input v-model="applyForm.name" /></el-form-item><el-form-item label="Main Domain" required><el-select v-model="applyForm.mainDomain" filterable :placeholder="domainOptions.length?dt('selectPublicMainDomain'):dt('noIssuableDomains')" style="width:100%"><el-option v-for="item in domainOptions" :key="`${item.accountId}-${item.domain}`" :label="item.domain" :value="item.domain" /></el-select></el-form-item>
      <el-form-item :label="dt('certTypeLabel')" required><el-radio-group v-model="applyForm.type"><el-radio-button value="SINGLE">{{ dt('certTypeSingle') }}</el-radio-button><el-radio-button value="WILDCARD">{{ dt('certTypeWildcard') }}</el-radio-button></el-radio-group></el-form-item><el-form-item :label="dt('certDomains')" required><el-input v-model="applyForm.certificateDomain" class="domain-mono" /></el-form-item>
      <el-form-item v-if="applyForm.type==='WILDCARD'" label="Root Domain"><el-checkbox v-model="applyForm.includeRootDomain">{{ dt('includeRootDomain', { domain: applyForm.mainDomain || 'example.com' }) }}</el-checkbox></el-form-item><el-form-item :label="dt('autoRenewLabel')"><el-switch v-model="applyForm.autoRenew" /></el-form-item><el-form-item :label="dt('renewBeforeLabel')"><el-input-number v-model="applyForm.renewBeforeDays" :min="1" :max="90" /> <span class="ssl-unit">{{ dt('dayUnit') }}</span></el-form-item>
      <el-form-item label="ACME Environment"><el-radio-group v-model="applyForm.acmeEnvironment"><el-radio value="staging">Test Environment</el-radio><el-radio value="production">Production Environment</el-radio></el-radio-group><div class="domain-form-tip">{{ dt('acmeTip') }}</div></el-form-item><el-form-item :label="dt('contactEmail')"><el-input v-model="applyForm.acmeEmail" :placeholder="dt('emailPlaceholder')" /></el-form-item>
      <div v-if="applyForm.mainDomain" class="ssl-provider-summary"><span>DNS Provider</span><strong>{{ providerLabel(selectedDomainInfo(applyForm.mainDomain).provider) }}</strong><span>DNS Account</span><strong>{{ selectedDomainInfo(applyForm.mainDomain).accountName }}</strong><span>{{ dt('validationMethod') }}</span><strong>DNS-01</strong></div>
    </el-form><template #footer><el-button @click="applyVisible=false">{{ dt('cancel') }}</el-button><el-button type="primary" :loading="submitting" :disabled="!domainOptions.length" @click="submitApply">{{ dt('createApplyTask') }}</el-button></template></el-dialog>

    <el-dialog v-model="uploadVisible" :title="dt('uploadDialogTitle')" width="min(820px, calc(100vw - 32px))" top="5vh"><el-alert v-if="!domainOptions.length" :title="dt('uploadPrerequisite')" type="warning" :closable="false" show-icon class="ssl-prerequisite"/><el-form label-position="top" class="ssl-upload-form"><div class="ssl-upload-grid"><el-form-item :label="dt('certName')" required><el-input v-model="uploadForm.name" /></el-form-item><el-form-item :label="dt('platformPublicMainDomain')" required><el-select v-model="uploadForm.mainDomain" filterable :placeholder="domainOptions.length?dt('selectPublicMainDomain'):dt('noManagedDomains')" style="width:100%"><el-option v-for="item in domainOptions" :key="`${item.accountId}-${item.domain}`" :label="item.domain" :value="item.domain" /></el-select></el-form-item></div><el-form-item label="Certificate PEM" required><el-input v-model="uploadForm.certificatePem" type="textarea" :rows="7" class="domain-mono" placeholder="-----BEGIN CERTIFICATE-----" /></el-form-item><el-form-item label="Private Key PEM" required><el-input v-model="uploadForm.privateKeyPem" type="textarea" :rows="7" class="domain-mono" placeholder="-----BEGIN PRIVATE KEY-----" /><div class="domain-form-tip">{{ dt('uploadKeyTip') }}</div></el-form-item><el-form-item label="Certificate Chain / CA Bundle"><el-input v-model="uploadForm.certificateChain" type="textarea" :rows="5" class="domain-mono" /></el-form-item><el-checkbox v-model="uploadForm.syncToCloud">{{ dt('syncAfterUpload') }}</el-checkbox></el-form><template #footer><el-button @click="uploadVisible=false">{{ dt('cancel') }}</el-button><el-button type="primary" :loading="submitting" :disabled="!domainOptions.length" @click="submitUpload">{{ dt('verifyAndSave') }}</el-button></template></el-dialog>

    <el-dialog v-model="deleteVisible" :title="dt('deleteDialogTitle')" width="min(560px, calc(100vw - 32px))"><div class="ssl-delete-warning"><strong>{{ dt('deleteSafetyTitle') }}</strong><p>{{ dt('deleteSafetyDesc') }}</p></div><el-checkbox v-if="deleteTarget?.cloudCertificateId" v-model="deleteCloud">{{ dt('deleteCloudCheckbox', { provider: providerLabel(deleteTarget?.provider) }) }}</el-checkbox><template #footer><el-button @click="deleteVisible=false">{{ dt('cancel') }}</el-button><el-button type="danger" :loading="submitting" @click="submitDelete">{{ dt('confirmDelete') }}</el-button></template></el-dialog>
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
