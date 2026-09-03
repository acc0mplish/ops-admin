<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { importFinOpsCosts, queryFinOpsAccounts, queryFinOpsSyncLogs, triggerFinOpsSync } from '../../../api/integration'
import { currentLocale } from '../../../utils/i18n-runtime'
import { ft } from '../../../utils/finops-i18n'
import './finops.css'

const accounts = ref([]), logs = ref([]), accountId = ref(''), importing = ref(false), syncing = ref(false), rawRecords = ref('[]'), syncRange = ref([]), syncResult = ref(null)
const money = value => Number(value || 0).toLocaleString(currentLocale.value === 'en-US' ? 'en-US' : 'ko-KR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const dateTime = value => value ? new Date(value).toLocaleString(currentLocale.value === 'en-US' ? 'en-US' : 'ko-KR', { hour12: false }) : '-'
const duration = value => value === null || value === undefined ? '-' : value < 60 ? ft('seconds', { count: value }) : ft('minutesSeconds', { minutes: Math.floor(value / 60), seconds: value % 60 })
const syncRangeReady = computed(() => syncRange.value?.length === 2)
const monthKey = date => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
function setRecentMonths(size) { const now = new Date(); syncRange.value = [monthKey(new Date(now.getFullYear(), now.getMonth() - size + 1, 1)), monthKey(now)] }
async function loadAccounts() { accounts.value = await queryFinOpsAccounts() || [] }
async function loadLogs() { logs.value = await queryFinOpsSyncLogs(accountId.value ? { accountId: accountId.value } : {}) || [] }
async function trigger() {
  if (!accountId.value) return ElMessage.warning(ft('selectAccountFirst'))
  if (!syncRangeReady.value) return ElMessage.warning(ft('selectSyncRange'))
  syncing.value = true; syncResult.value = null
  try {
    syncResult.value = await triggerFinOpsSync({ accountId: accountId.value, start_month: syncRange.value[0], end_month: syncRange.value[1] })
    const failed = syncResult.value?.months?.filter(item => item.status !== 'success').length || 0
    ElMessage[failed ? 'warning' : 'success'](failed ? ft('syncCompletedWithFailures', { count: failed }) : ft('syncCompleted'))
    await loadLogs()
  } finally { syncing.value = false }
}
async function importRecords() {
  if (!accountId.value) return ElMessage.warning(ft('selectAccountFirst'))
  let records
  try { records = JSON.parse(rawRecords.value) } catch (_) { return ElMessage.error(ft('invalidBillingJson')) }
  if (!Array.isArray(records)) return ElMessage.warning(ft('billingMustArray'))
  importing.value = true
  try { const result = await importFinOpsCosts({ accountId: accountId.value, records }); ElMessage.success(ft('importedRecords', { count: result?.recordCount || 0 })); await loadLogs() } finally { importing.value = false }
}
onMounted(async () => { setRecentMonths(6); await loadAccounts(); await loadLogs() })
</script>

<template>
  <div class="finops-page finops-sync-page">
    <section class="finops-panel">
      <div class="finops-head"><div><h2>{{ ft('billingSync') }}</h2><p>{{ ft('billingSyncDesc') }}</p></div><div class="finops-actions"><el-select v-model="accountId" clearable :placeholder="ft('selectCloudAccountShort')" style="width:200px" @change="loadLogs"><el-option v-for="item in accounts" :key="item.id" :label="`${item.name} · ${item.provider}`" :value="item.id" /></el-select><el-date-picker v-model="syncRange" type="monthrange" value-format="YYYY-MM" format="YYYY-MM" :clearable="false" :range-separator="ft('to')" :start-placeholder="ft('startBillingMonth')" :end-placeholder="ft('endBillingMonth')"/><el-button :loading="syncing" type="primary" @click="trigger">{{ ft('syncNow') }}</el-button></div></div>
      <div class="sync-presets"><span>{{ ft('quickRange') }}:</span><el-button size="small" @click="setRecentMonths(3)">{{ ft('recentMonths', { count: 3 }) }}</el-button><el-button size="small" @click="setRecentMonths(6)">{{ ft('recentMonths', { count: 6 }) }}</el-button><el-button size="small" @click="setRecentMonths(12)">{{ ft('recentMonths', { count: 12 }) }}</el-button></div>
      <el-alert type="info" :closable="false" show-icon :title="ft('aliyunEstimateHint')" />
    </section>
    <section v-if="syncResult" class="finops-panel finops-sync-result"><div class="finops-card-title"><div><h2>{{ ft('syncResult') }}</h2><p>{{ syncResult.startMonth }} {{ ft('to') }} {{ syncResult.endMonth }} · {{ syncResult.status }} · {{ ft('finalSnapshot') }}</p></div><b class="finops-money">¥ {{ money(syncResult.totalAmount) }}</b></div><el-table :data="syncResult.months" size="small" stripe><el-table-column prop="month" :label="ft('billingMonth')" width="110"/><el-table-column :label="ft('status')" width="100"><template #default="{row}"><el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status === 'success' ? ft('success') : ft('failure') }}</el-tag></template></el-table-column><el-table-column prop="sourceRecordCount" :label="ft('sourceRecords')" width="110"/><el-table-column prop="recordCount" :label="ft('storedRecords')" width="110"/><el-table-column :label="ft('storedAmount')" width="160"><template #default="{row}">¥ {{ money(row.totalAmount) }}</template></el-table-column><el-table-column :label="ft('dedupOverwrite')" width="130"><template #default="{row}">{{ ft('recordsUnit', { count: row.deduplicatedCount || 0 }) }}</template></el-table-column><el-table-column prop="error" :label="ft('errorInfo')" min-width="260"><template #default="{row}">{{ row.error || '-' }}</template></el-table-column></el-table></section>
    <section class="finops-panel"><div class="finops-head"><div><h2>{{ ft('manualImport') }}</h2><p>{{ ft('manualImportDesc') }}</p></div><el-button :loading="importing" type="primary" @click="importRecords">{{ ft('importBilling') }}</el-button></div><textarea v-model="rawRecords" class="finops-import" spellcheck="false" /></section>
    <section class="finops-panel finops-sync-history"><div class="finops-head"><div><h2>{{ ft('syncHistory') }}</h2><p>{{ ft('syncHistoryDesc') }}</p></div><el-button @click="loadLogs">{{ ft('refreshRecords') }}</el-button></div><el-alert type="info" :closable="false" show-icon :title="ft('oldSyncHint')" /><el-table :data="logs" stripe><el-table-column prop="accountName" :label="ft('cloudAccount')" min-width="150"><template #default="{row}"><b>{{ row.accountName || `#${row.accountId}` }}</b><small class="subline">{{ row.provider || '-' }}</small></template></el-table-column><el-table-column prop="trigger" :label="ft('triggerType')" width="110"/><el-table-column prop="billingMonth" :label="ft('billingMonth')" width="100"><template #default="{row}">{{ row.billingMonth || '-' }}</template></el-table-column><el-table-column :label="ft('status')" width="90"><template #default="{row}"><el-tag :type="row.status === 'success' ? 'success' : row.status === 'running' ? 'warning' : 'danger'">{{ row.status === 'success' ? ft('success') : row.status === 'running' ? ft('running') : ft('failure') }}</el-tag></template></el-table-column><el-table-column :label="ft('sourceRecords')" width="100" align="right"><template #default="{row}">{{ row.snapshotVerified ? row.sourceRecordCount : row.recordCount }}</template></el-table-column><el-table-column :label="ft('sourceAmount')" width="135" align="right"><template #default="{row}">¥ {{ money(row.snapshotVerified ? row.sourceTotalAmount : row.totalAmount) }}</template></el-table-column><el-table-column :label="ft('storedRecords')" width="110" align="right"><template #default="{row}">{{ row.snapshotVerified ? row.recordCount : ft('pendingResync') }}</template></el-table-column><el-table-column :label="ft('storedAmount')" width="150" align="right"><template #default="{row}"><span v-if="row.snapshotVerified">¥ {{ money(row.totalAmount) }}</span><el-tag v-else size="small" type="warning">{{ ft('oldRecordPending') }}</el-tag></template></el-table-column><el-table-column :label="ft('deduplicated')" width="85" align="right"><template #default="{row}">{{ row.snapshotVerified ? ft('recordsUnit', { count: row.deduplicatedCount || 0 }) : '-' }}</template></el-table-column><el-table-column :label="ft('executionDuration')" width="120"><template #default="{row}">{{ duration(row.durationSeconds) }}</template></el-table-column><el-table-column prop="message" :label="ft('executionInfo')" min-width="230" show-overflow-tooltip/><el-table-column :label="ft('startedAt')" width="175"><template #default="{row}">{{ dateTime(row.startedAt) }}</template></el-table-column><el-table-column :label="ft('finishedAt')" width="175"><template #default="{row}">{{ dateTime(row.finishedAt) }}</template></el-table-column></el-table><el-empty v-if="!logs.length" :description="ft('noSyncHistory')" /></section>
  </div>
</template>

<style scoped>
.subline{display:block;margin-top:4px;color:#8291a8;font-weight:400}.finops-sync-history{overflow:hidden}.sync-presets{display:flex;align-items:center;gap:8px;margin:0 0 16px;color:#64748b}.finops-sync-result{border-color:#cde9dc}
</style>
