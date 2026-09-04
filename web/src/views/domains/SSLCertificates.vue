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
  PENDING: ['발급 대기', 'info'], APPLYING: ['신청 중', 'warning'], DNS_PENDING: ['DNS 검증 대기', 'warning'], VALIDATING: ['검증 중', 'warning'], ISSUING: ['발급 중', 'warning'], ISSUED: ['발급됨', 'success'], NORMAL: ['정상', 'success'], EXPIRING: ['만료 임박', 'warning'], EXPIRED: ['만료', 'danger'], RENEWING: ['갱신 중', 'warning'], RENEW_FAILED: ['갱신 실패', 'danger'], APPLY_FAILED: ['발급 실패', 'danger'], SYNC_FAILED: ['동기화 실패', 'danger'], REVOKED: ['폐기됨', 'info']
}
const sourceLabels = { ALIYUN: 'Alibaba Cloud 동기화', TENCENT: 'Tencent Cloud 동기화', MANUAL: '수동 업로드', ACME: '플랫폼 발급' }
const typeLabels = { SINGLE: '단일 Domain', WILDCARD: '와일드카드 Domain', SAN: 'SAN 다중 Domain' }
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
  if (!applyForm.mainDomain || !applyForm.certificateDomain || !applyForm.name.trim()) { ElMessage.warning('발급 정보를 모두 입력하십시오.'); return }
  submitting.value = true
  try { await applySSLCertificate(applyForm); ElMessage.success('인증서 발급 Task를 생성했습니다.'); applyVisible.value = false; await load(); startPolling() } finally { submitting.value = false }
}

async function submitUpload() {
  if (!uploadForm.name.trim() || !uploadForm.mainDomain || !uploadForm.certificatePem.trim() || !uploadForm.privateKeyPem.trim()) { ElMessage.warning('인증서 이름, Main Domain, Certificate와 Private Key를 입력하십시오.'); return }
  submitting.value = true
  try { await uploadSSLCertificate(uploadForm); ElMessage.success('인증서를 검증하고 안전하게 저장했습니다.'); uploadVisible.value = false; await load(); if (uploadForm.syncToCloud) startPolling() } finally { submitting.value = false }
}

async function submitSync() { syncing.value = true; try { await syncSSLCertificates(Number(syncAccountId.value) || 0); ElMessage.success('클라우드 인증서 동기화 Task를 생성했습니다.'); syncVisible.value = false; startPolling() } finally { syncing.value = false } }
async function renew(row) { await ElMessageBox.confirm(`${row.name}의 DNS-01 검증을 다시 실행하고 새 인증서를 발급합니다. 계속하시겠습니까?`, '즉시 갱신', { type: 'warning' }); await renewSSLCertificate(row.id); ElMessage.success('갱신 Task를 생성했습니다.'); startPolling() }
async function resync(row) { await resyncSSLCertificate(row.id); ElMessage.success('클라우드 동기화 Task를 생성했습니다.'); startPolling() }
function openDelete(row) { deleteTarget.value = row; deleteCloud.value = Boolean(row.cloudCertificateId); deleteVisible.value = true }
async function submitDelete() { submitting.value = true; try { const data = await deleteSSLCertificate({ id: deleteTarget.value.id, deleteCloud: deleteCloud.value }); deleteVisible.value = false; if (data.taskId) { ElMessage.success('인증서 삭제 Task를 생성했습니다.'); startPolling() } else { ElMessage.success('인증서를 삭제했습니다.'); await load() } } finally { submitting.value = false } }
async function command({ command, row }) { if (command === 'renew') await renew(row); if (command === 'sync') await resync(row); if (command === 'delete') openDelete(row) }

onMounted(async () => { const [accountList, domains] = await Promise.all([queryDNSAccountOptions(), querySSLCertificateDomainOptions()]); accounts.value = accountList || []; domainOptions.value = domains || []; await Promise.all([load(), loadTasks()]); if (activeTasks.value.length) startPolling() })
onBeforeUnmount(() => { if (pollTimer) clearInterval(pollTimer) })
</script>

<template>
  <section class="domain-page domain-panel page-card ssl-page">
    <div class="domain-page-head">
      <div><div class="domain-eyebrow">{{ uiT('tlsCertificateInventory') }}</div><h2>SSL 인증서</h2><p>클라우드 인증서와 플랫폼 ACME 인증서를 통합 관리합니다. Private Key는 암호화해 저장하며 일반 Query API는 Key 내용을 절대 반환하지 않습니다.</p></div>
      <div class="ssl-head-actions">
        <el-button v-permission="'domains:ssl:sync'" @click="syncVisible=true">클라우드에서 동기화</el-button>
        <el-button v-permission="'domains:ssl:apply'" @click="resetApply">인증서 발급</el-button>
        <el-button v-permission="'domains:ssl:upload'" type="primary" @click="resetUpload">인증서 업로드</el-button>
      </div>
    </div>

    <div v-if="activeTasks.length" class="ssl-task-strip">
      <div><strong>{{ activeTasks.length }}개의 인증서 Task가 실행 중입니다</strong><span>{{ activeTasks[0].stage }} · {{ activeTasks[0].progress }}%</span></div>
      <el-progress :percentage="activeTasks[0].progress" :stroke-width="6" :show-text="false" />
    </div>

    <div class="ssl-stat-grid">
      <article class="domain-stat"><div class="domain-stat__label">전체 인증서</div><div class="domain-stat__value">{{ stats.total }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">유효 인증서</div><div class="domain-stat__value is-valid">{{ stats.valid }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">만료 임박</div><div class="domain-stat__value is-warning">{{ stats.expiring }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">만료</div><div class="domain-stat__value is-danger">{{ stats.expired }}</div></article>
      <article class="domain-stat"><div class="domain-stat__label">자동 갱신</div><div class="domain-stat__value">{{ stats.autoRenew }}</div></article>
    </div>

    <div class="domain-toolbar" role="search"><div class="domain-toolbar__filters">
      <el-input v-model="query.keyword" clearable placeholder="인증서 이름 / Main Domain 검색" style="width:240px" @keyup.enter="load" />
      <el-select v-model="query.status" clearable placeholder="인증서 상태" style="width:150px"><el-option v-for="(meta,key) in statusMeta" :key="key" :label="meta[0]" :value="key" /></el-select>
      <el-select v-model="query.source" clearable placeholder="인증서 Source" style="width:150px"><el-option v-for="(label,key) in sourceLabels" :key="key" :label="label" :value="key" /></el-select>
      <el-select v-model="query.accountId" clearable placeholder="Cloud Account" style="width:170px"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select>
      <el-button @click="load">조회</el-button>
    </div></div>

    <div class="domain-table-wrap ssl-table-wrap"><el-table v-loading="loading" :data="rows" border @row-dblclick="row=>router.push(`/domains/public/certificates/${row.id}`)">
      <el-table-column prop="name" label="인증서 이름" min-width="180" fixed="left"><template #default="{row}"><el-button link type="primary" @click="router.push(`/domains/public/certificates/${row.id}`)">{{ row.name }}</el-button></template></el-table-column>
      <el-table-column prop="mainDomain" label="Main Domain" min-width="160" /><el-table-column label="인증서 Domain" min-width="220"><template #default="{row}"><span class="domain-mono">{{ row.domains.join(', ') }}</span></template></el-table-column>
      <el-table-column label="유형" width="120"><template #default="{row}">{{ typeLabels[row.type] || row.type }}</template></el-table-column><el-table-column label="Source" width="120"><template #default="{row}">{{ sourceLabels[row.source] || row.source }}</template></el-table-column>
      <el-table-column label="Provider" width="110"><template #default="{row}">{{ providerLabel(row.provider) }}</template></el-table-column><el-table-column prop="dnsAccountName" label="Cloud Account" min-width="140" /><el-table-column prop="issuer" label="발급 기관" min-width="180" show-overflow-tooltip />
      <el-table-column label="적용 시각" width="170"><template #default="{row}">{{ formatDate(row.notBefore) }}</template></el-table-column><el-table-column label="만료 시각" width="170"><template #default="{row}">{{ formatDate(row.notAfter) }}</template></el-table-column>
      <el-table-column label="잔여 일수" width="100"><template #default="{row}"><strong :class="{'ssl-days-warning':row.remainingDays<=30}">{{ row.remainingDays }} 일</strong></template></el-table-column><el-table-column label="자동 갱신" width="100"><template #default="{row}"><el-tag :type="row.autoRenew?'success':'info'" effect="plain">{{ row.autoRenew?'켜짐':'꺼짐' }}</el-tag></template></el-table-column>
      <el-table-column label="클라우드 동기화" width="110"><template #default="{row}"><el-tag :type="row.cloudSyncStatus==='SYNCED'?'success':row.cloudSyncStatus==='FAILED'?'danger':'info'" effect="plain">{{ {SYNCED:'동기화됨',FAILED:'실패',PENDING:'동기화 중',LOCAL:'로컬 전용'}[row.cloudSyncStatus] }}</el-tag></template></el-table-column>
      <el-table-column label="인증서 상태" width="120"><template #default="{row}"><el-tag :type="statusType(row.status)" effect="light">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
      <el-table-column label="작업" width="170" fixed="right"><template #default="{row}"><el-button link type="primary" @click="router.push(`/domains/public/certificates/${row.id}`)">상세</el-button><el-dropdown trigger="click" @command="value=>command({command:value,row})"><el-button link>더보기⌄</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item v-if="row.source==='ACME'" command="renew">즉시 갱신</el-dropdown-item><el-dropdown-item command="sync">클라우드로 동기화</el-dropdown-item><el-dropdown-item divided command="delete" class="batch-danger-item">인증서 삭제</el-dropdown-item></el-dropdown-menu></template></el-dropdown></template></el-table-column>
      <template #empty><div class="domain-empty"><strong>SSL 인증서가 없습니다</strong><p>클라우드 Provider에서 동기화하거나 ACME로 발급하거나 기존 PEM 인증서를 업로드할 수 있습니다.</p><el-button type="primary" @click="resetUpload">인증서 업로드</el-button></div></template>
    </el-table></div>
    <div class="domain-pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :total="total" @current-change="load" @size-change="load" /></div>

    <el-dialog v-model="syncVisible" title="클라우드에서 인증서 동기화" width="min(520px, calc(100vw - 32px))"><el-form label-position="top"><el-form-item label="동기화 Account"><el-select v-model="syncAccountId" clearable placeholder="전체 활성 Account" style="width:100%"><el-option v-for="item in accounts" :key="item.id" :label="`${item.name} · ${providerLabel(item.provider)}`" :value="item.id" /></el-select></el-form-item><div class="domain-form-tip">동기화는 인증서 목록과 Metadata만 읽으며 목록 동기화를 위해 클라우드 Private Key를 내려받지 않습니다.</div></el-form><template #footer><el-button @click="syncVisible=false">취소</el-button><el-button type="primary" :loading="syncing" @click="submitSync">동기화 Task 생성</el-button></template></el-dialog>

    <el-dialog v-model="applyVisible" title="SSL 인증서 발급" width="min(680px, calc(100vw - 32px))"><el-alert v-if="!domainOptions.length" title="먼저 퍼블릭 도메인 Page에서 도메인을 동기화하고 도메인에 활성화된 DNS Cloud Account가 연결되어 있는지 확인하십시오." type="warning" :closable="false" show-icon class="ssl-prerequisite"/><el-form label-width="116px">
      <el-form-item label="인증서 이름" required><el-input v-model="applyForm.name" /></el-form-item><el-form-item label="Main Domain" required><el-select v-model="applyForm.mainDomain" filterable :placeholder="domainOptions.length?'퍼블릭 Main Domain을 선택하십시오':'발급 가능한 퍼블릭 도메인이 없습니다'" style="width:100%"><el-option v-for="item in domainOptions" :key="`${item.accountId}-${item.domain}`" :label="item.domain" :value="item.domain" /></el-select></el-form-item>
      <el-form-item label="인증서 유형" required><el-radio-group v-model="applyForm.type"><el-radio-button value="SINGLE">단일 Domain</el-radio-button><el-radio-button value="WILDCARD">와일드카드 Domain</el-radio-button></el-radio-group></el-form-item><el-form-item label="인증서 Domain" required><el-input v-model="applyForm.certificateDomain" class="domain-mono" /></el-form-item>
      <el-form-item v-if="applyForm.type==='WILDCARD'" label="Root Domain"><el-checkbox v-model="applyForm.includeRootDomain">동시에 {{ applyForm.mainDomain || 'example.com' }} 포함</el-checkbox></el-form-item><el-form-item label="자동 갱신"><el-switch v-model="applyForm.autoRenew" /></el-form-item><el-form-item label="만료 전 갱신 일수"><el-input-number v-model="applyForm.renewBeforeDays" :min="1" :max="90" /> <span class="ssl-unit">일</span></el-form-item>
      <el-form-item label="ACME Environment"><el-radio-group v-model="applyForm.acmeEnvironment"><el-radio value="staging">Test Environment</el-radio><el-radio value="production">Production Environment</el-radio></el-radio-group><div class="domain-form-tip">먼저 Test Environment에서 DNS-01 Flow를 검증해 Production CA 빈도 제한 발생을 피하십시오.</div></el-form-item><el-form-item label="연락 이메일"><el-input v-model="applyForm.acmeEmail" placeholder="비워두면 Server 측 ACME Config를 사용합니다" /></el-form-item>
      <div v-if="applyForm.mainDomain" class="ssl-provider-summary"><span>DNS Provider</span><strong>{{ providerLabel(selectedDomainInfo(applyForm.mainDomain).provider) }}</strong><span>DNS Account</span><strong>{{ selectedDomainInfo(applyForm.mainDomain).accountName }}</strong><span>검증 방식</span><strong>DNS-01</strong></div>
    </el-form><template #footer><el-button @click="applyVisible=false">취소</el-button><el-button type="primary" :loading="submitting" :disabled="!domainOptions.length" @click="submitApply">발급 Task 생성</el-button></template></el-dialog>

    <el-dialog v-model="uploadVisible" title="SSL 인증서 업로드" width="min(820px, calc(100vw - 32px))" top="5vh"><el-alert v-if="!domainOptions.length" title="먼저 퍼블릭 도메인을 동기화하십시오. 인증서는 플랫폼이 관리하는 퍼블릭 Main Domain에 속해야 합니다." type="warning" :closable="false" show-icon class="ssl-prerequisite"/><el-form label-position="top" class="ssl-upload-form"><div class="ssl-upload-grid"><el-form-item label="인증서 이름" required><el-input v-model="uploadForm.name" /></el-form-item><el-form-item label="플랫폼 퍼블릭 Main Domain" required><el-select v-model="uploadForm.mainDomain" filterable :placeholder="domainOptions.length?'퍼블릭 Main Domain을 선택하십시오':'관리 중인 퍼블릭 도메인이 없습니다'" style="width:100%"><el-option v-for="item in domainOptions" :key="`${item.accountId}-${item.domain}`" :label="item.domain" :value="item.domain" /></el-select></el-form-item></div><el-form-item label="Certificate PEM" required><el-input v-model="uploadForm.certificatePem" type="textarea" :rows="7" class="domain-mono" placeholder="-----BEGIN CERTIFICATE-----" /></el-form-item><el-form-item label="Private Key PEM" required><el-input v-model="uploadForm.privateKeyPem" type="textarea" :rows="7" class="domain-mono" placeholder="-----BEGIN PRIVATE KEY-----" /><div class="domain-form-tip">Backend가 인증서와 Private Key 일치 여부를 검증합니다. Private Key는 AES-256-GCM으로 암호화해 저장합니다.</div></el-form-item><el-form-item label="Certificate Chain / CA Bundle"><el-input v-model="uploadForm.certificateChain" type="textarea" :rows="5" class="domain-mono" /></el-form-item><el-checkbox v-model="uploadForm.syncToCloud">검증 후 저장하며 Main Domain에 해당하는 Cloud Account에 동기화</el-checkbox></el-form><template #footer><el-button @click="uploadVisible=false">취소</el-button><el-button type="primary" :loading="submitting" :disabled="!domainOptions.length" @click="submitUpload">검증 후 저장</el-button></template></el-dialog>

    <el-dialog v-model="deleteVisible" title="SSL 인증서 삭제" width="min(560px, calc(100vw - 32px))"><div class="ssl-delete-warning"><strong>삭제 전 안전 검사를 실행합니다</strong><p>만료되지 않은 단일 Domain 인증서는 퍼블릭 DNS를 실시간으로 조회합니다. DNS 레코드가 남아 있으면 삭제가 금지됩니다. 만료되지 않은 와일드카드 Domain 인증서는 항상 삭제가 금지됩니다.</p></div><el-checkbox v-if="deleteTarget?.cloudCertificateId" v-model="deleteCloud">{{ providerLabel(deleteTarget?.provider) }} 클라우드 인증서도 함께 삭제</el-checkbox><template #footer><el-button @click="deleteVisible=false">취소</el-button><el-button type="danger" :loading="submitting" @click="submitDelete">삭제 확인</el-button></template></el-dialog>
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
