<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import FinOpsHeader from './FinOpsHeader.vue'
import { deleteFinOpsRecommendation, generateFinOpsRecommendations, queryAIModels, queryFinOpsAccounts, queryFinOpsRecommendations } from '../../../api/integration'
import { ft } from '../../../utils/finops-i18n'
import './finops.css'

const loading = ref(false)
const generating = ref(false)
const strategyDialogVisible = ref(false)
const strategy = ref('default')
const modelId = ref(0)
const models = ref([])
const rows = ref([])
const accounts = ref([])
const analysisAccountId = ref('')
const analysisMonth = ref(new Date().toISOString().slice(0, 7))
const money = value => Number(value || 0).toLocaleString('ko-KR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

async function load() {
  loading.value = true
  try {
    rows.value = await queryFinOpsRecommendations({ ...(analysisAccountId.value ? { account_id: analysisAccountId.value } : {}), ...(analysisMonth.value ? { month: analysisMonth.value } : {}) }) || []
  } finally {
    loading.value = false
  }
}

async function loadModels() {
  models.value = (await queryAIModels() || []).filter(item => Number(item.status) === 1)
  const defaultModel = models.value.find(item => item.isDefault) || models.value[0]
  if (!modelId.value && defaultModel) modelId.value = defaultModel.id
}

async function loadAccounts() {
  accounts.value = await queryFinOpsAccounts() || []
}

async function generate() {
  if (strategy.value === 'ai' && !modelId.value) {
    ElMessage.warning(ft('warnSelectModel'))
    return
  }
  generating.value = true
  try {
    const result = await generateFinOpsRecommendations({ strategy: strategy.value, modelId: strategy.value === 'ai' ? (modelId.value || 0) : 0, account_id: analysisAccountId.value || 0, month: analysisMonth.value || '' })
    if (result?.mode === 'ai') {
      ElMessage.success(ft('aiGenerated', { count: result?.count || 0 }))
    } else if (result?.mode === 'ai_fallback') {
      ElMessage.warning(ft('aiFallbackGenerated', { count: result?.count || 0 }))
    } else {
      ElMessage.success(ft('defaultGenerated', { count: result?.count || 0 }))
    }
    await load()
    strategyDialogVisible.value = false
  } finally {
    generating.value = false
  }
}

function openGenerateDialog() {
  strategy.value = 'default'
  strategyDialogVisible.value = true
}

async function remove(row) {
  await ElMessageBox.confirm(ft('deleteConfirm', { title: row.title }), ft('deleteTitle'), { type: 'warning' })
  await deleteFinOpsRecommendation(row.id)
  ElMessage.success(ft('deletedMsg'))
  await load()
}

function escapeHtml(value) {
  return String(value || '').replace(/[&<>'"]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char]))
}

function strategyLabel(row) {
  if (row.strategy === 'ai_fallback') return ft('strategyAiFallback')
  if (row.strategy === 'ai' || row.category === 'ai_finops') return ft('strategyAi', { model: row.modelName ? ` · ${row.modelName}` : '' })
  return ft('strategyDefault')
}

function recommendationName(row) {
  const accountName = accounts.value.find(item => String(item.id) === String(row.analysisAccountId))?.name || (Number(row.analysisAccountId) ? `Cloud Account #${row.analysisAccountId}` : ft('allCloudAccounts'))
  const month = row.analysisMonth || ft('noBillingMonth')
  return ft('recommendationTitle', { account: accountName, month, strategy: strategyLabel(row) })
}

function reportSections(description) {
  return String(description || '').split(/\n\n(?=## )/).map((block, index) => {
    const lines = block.replace(/^##\s*/, '').split('\n')
    const title = lines.shift() || ft('analysisSectionFallback', { index: index + 1 })
    const numbered = lines.filter(line => /^\d+\.\s/.test(line.trim()))
    const content = numbered.length
      ? `${numbered.slice(0, 4).join('\n')}${numbered.length > 4 ? `\n${ft('moreInExport', { count: numbered.length - 4 })}` : ''}`
      : lines.join('\n')
    return `<article class="section"><div class="section-index">${String(index + 1).padStart(2, '0')}</div><div><h2>${escapeHtml(title)}</h2><p>${escapeHtml(content).replace(/\n/g, '<br>')}</p></div></article>`
  }).join('')
}

function openReport(row, autoPrint = false) {
  const report = window.open('', '_blank')
  if (!report) {
    ElMessage.warning(ft('popupBlocked'))
    return
  }
  const savingRate = Number(row.currentCost) > 0 ? Math.min(Number(row.saving || 0) / Number(row.currentCost) * 100, 100) : 0
  const risk = savingRate >= 25 ? ft('riskHigh') : savingRate >= 10 ? ft('riskMedium') : ft('riskLow')
  const riskClass = savingRate >= 25 ? 'high' : savingRate >= 10 ? 'medium' : 'low'
  report.document.open()
  report.document.write(`<!doctype html><html lang="ko-KR"><head><meta charset="utf-8"><title>${escapeHtml(recommendationName(row))} - ${ft('reportTitleSuffix')}</title><style>*{box-sizing:border-box}body{margin:0;padding:28px;background:linear-gradient(135deg,#edf3ff,#f7f3ff);font-family:"Malgun Gothic",Arial,sans-serif;color:#142b55}.report{max-width:1440px;margin:auto}.report-title{margin:0 0 16px;font-size:22px}.kpis{display:grid;grid-template-columns:repeat(4,1fr);gap:18px}.kpi,.panel,.conclusion{background:rgba(255,255,255,.94);border:1px solid #e7edfa;border-radius:22px;box-shadow:0 14px 38px rgba(41,77,140,.10)}.kpi{padding:23px 24px;min-height:132px}.kpi i{display:grid;place-items:center;width:52px;height:52px;float:left;margin-right:15px;border-radius:17px;background:#4168f5;color:#fff;font-style:normal;font-size:25px}.kpi:nth-child(2) i{background:#8948ee}.kpi:nth-child(3) i{background:#13b982}.kpi:nth-child(4) i{background:#ff6265}.label{font-size:14px;color:#687ba0}.metric{font-size:29px;font-weight:800;line-height:1.25;margin-top:8px;color:#142b55}.metric.green{color:#12a875}.metric.high{color:#f05a60}.metric.medium{color:#f09b39}.metric.low{color:#13a976}.hint{clear:both;padding-top:10px;font-size:12px;color:#7e8eaf}.layout{display:grid;grid-template-columns:1.25fr .85fr 1fr;gap:18px;margin-top:20px}.panel{padding:24px}.panel h2{font-size:19px;margin:0 0 4px}.sub{font-size:13px;color:#7890b5;margin-bottom:18px}.strategy{display:grid;gap:14px}.strategy-row{display:grid;grid-template-columns:38px 1fr;gap:12px;align-items:center;padding:12px 0;border-bottom:1px solid #edf1f8}.strategy-row:last-child{border:0}.strategy-icon{width:34px;height:34px;border-radius:11px;background:#eaf0ff;color:#4168f5;display:grid;place-items:center;font-weight:800}.strategy-row b{font-size:14px}.strategy-row span{display:block;color:#7385a6;font-size:12px;margin-top:3px}.ring{width:160px;height:160px;border-radius:50%;margin:15px auto;background:conic-gradient(#12ba83 ${savingRate}%,#eaf0fa 0);display:grid;place-items:center}.ring div{width:116px;height:116px;border-radius:50%;background:#fff;display:grid;place-items:center;text-align:center;font-size:13px;color:#7485a6}.ring strong{display:block;font-size:24px;color:#142b55}.bar{height:10px;border-radius:10px;background:#eaf0fa;overflow:hidden;margin:12px 0 7px}.bar i{display:block;height:100%;width:${savingRate}%;background:linear-gradient(90deg,#4168f5,#13b982);border-radius:10px}.sections{display:grid;gap:11px}.section{display:grid;grid-template-columns:32px 1fr;gap:11px;padding:13px;border:1px solid #e7edf8;border-radius:14px;background:#fbfcff}.section-index{width:28px;height:28px;border-radius:9px;background:#eaf0ff;color:#4168f5;display:grid;place-items:center;font-size:11px;font-weight:800}.section h2{font-size:14px;margin:0 0 5px}.section p{margin:0;color:#647798;font-size:12px;line-height:1.65}.conclusion{margin-top:20px;padding:24px 28px;display:grid;grid-template-columns:52px 1fr auto;gap:17px;align-items:center}.check{width:48px;height:48px;border-radius:16px;background:#5a57ef;color:#fff;font-size:28px;display:grid;place-items:center}.conclusion h2{margin:0 0 5px;font-size:20px}.conclusion p{margin:0;color:#7184a8;font-size:13px}.annual{text-align:right}.annual b{display:block;font-size:22px;color:#12a875}.actions{text-align:center;margin:22px}.actions button{border:0;background:#4168f5;color:#fff;padding:11px 22px;border-radius:10px;font-weight:700;cursor:pointer}@media(max-width:900px){.kpis,.layout{grid-template-columns:1fr 1fr}.layout .panel:last-child{grid-column:span 2}}@media(max-width:580px){body{padding:12px}.kpis,.layout{grid-template-columns:1fr}.layout .panel:last-child{grid-column:auto}.conclusion{grid-template-columns:48px 1fr}.annual{grid-column:2;text-align:left}}@media print{body{padding:0;background:#fff}.kpi,.panel,.conclusion{box-shadow:none}.actions{display:none}}</style></head><body><main class="report"><h1 class="report-title">${escapeHtml(recommendationName(row))}</h1><section class="kpis"><article class="kpi"><i>¥</i><div class="label">${ft('kpiAnalyzedCost')}</div><div class="metric">¥ ${money(row.currentCost)}</div><div class="hint">${escapeHtml(row.provider || 'multi-cloud')} · ${ft('thisRun')}</div></article><article class="kpi"><i>◎</i><div class="label">${ft('analysisStrategy')}</div><div class="metric" style="font-size:20px">${escapeHtml(strategyLabel(row))}</div><div class="hint">Priority ${escapeHtml(row.priority || 'P2')}</div></article><article class="kpi"><i>↘</i><div class="label">${ft('kpiEstimatedSaving')}</div><div class="metric green">¥ ${money(row.saving)}</div><div class="hint">${ft('kpiPercentOfCost', { rate: savingRate.toFixed(1) })}</div></article><article class="kpi"><i>!</i><div class="label">${ft('kpiBudgetRisk')}</div><div class="metric ${riskClass}">${risk}</div><div class="hint">${ft('hintVerifyIdle')}</div></article></section><section class="layout"><article class="panel"><h2>${ft('panelStrategyScope')}</h2><div class="sub">${ft('panelSubPath')}</div><div class="strategy"><div class="strategy-row"><div class="strategy-icon">1</div><div><b>${ft('strat1')}</b><span>${ft('strat1Desc')}</span></div></div><div class="strategy-row"><div class="strategy-icon">2</div><div><b>${ft('strat2')}</b><span>${ft('strat2Desc')}</span></div></div><div class="strategy-row"><div class="strategy-icon">3</div><div><b>${ft('strat3')}</b><span>${ft('strat3Desc')}</span></div></div><div class="strategy-row"><div class="strategy-icon">4</div><div><b>${ft('strat4')}</b><span>${ft('strat4Desc')}</span></div></div></div></article><article class="panel"><h2>${ft('savingsPotential')}</h2><div class="sub">${ft('panelSubRate')}</div><div class="ring"><div><strong>${savingRate.toFixed(1)}%</strong>${ft('savingsPotential')}</div></div><div class="label">${ft('estimatedMonthlySaving', { amount: money(row.saving) })}</div><div class="bar"><i></i></div><div class="sub">${ft('subVerifyFirst')}</div></article><article class="panel"><h2>${ft('panelRecommendations')}</h2><div class="sub">${ft('panelSubConclusion')}</div><div class="sections">${reportSections(row.description)}</div></article></section><section class="conclusion"><div class="check">✓</div><div><h2>${ft('conclusionTitle')}</h2><p>${ft('conclusionBody')}</p></div><div class="annual"><div class="label">${ft('annualSaving')}</div><b>¥ ${money(Number(row.saving || 0) * 12)}</b></div></section><div class="actions"><button onclick="window.print()">${ft('downloadPdfSave')}</button></div></main></body></html>`)
  report.document.close()
  const polish = report.document.createElement('style')
  polish.textContent = '.layout{align-items:start}.panel{min-height:0}.sections{gap:9px}.section{padding:11px 12px}.section p{line-height:1.55}.strategy-row{padding:10px 0}.conclusion{margin-top:18px}'
  report.document.head.appendChild(polish)
  if (autoPrint) setTimeout(() => report.print(), 300)
}

onMounted(async () => {
  await Promise.all([load(), loadModels(), loadAccounts()])
})
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section class="finops-panel" v-loading="loading">
      <div class="finops-head">
        <div><h2>{{ ft('recommendationsPageTitle') }}</h2><p>{{ ft('recommendationsPageDesc') }}</p></div>
        <div class="finops-actions">
          <el-select v-model="analysisAccountId" clearable :placeholder="ft('allCloudAccounts')" style="width: 170px" @change="load">
            <el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-date-picker v-model="analysisMonth" type="month" value-format="YYYY-MM" format="YYYY-MM" clearable :placeholder="ft('analysisMonthPlaceholder')" style="width: 145px" @change="load" />
          <el-button type="primary" @click="openGenerateDialog">{{ ft('generateBtn') }}</el-button>
        </div>
      </div>
      <el-table :data="rows" stripe>
        <el-table-column :label="ft('colReport')" min-width="300"><template #default="{ row }">{{ recommendationName(row) }}</template></el-table-column>
        <el-table-column prop="provider" label="Cloud Provider" width="110" />
        <el-table-column :label="ft('analysisStrategy')" width="180"><template #default="{ row }">{{ strategyLabel(row) }}</template></el-table-column>
        <el-table-column :label="ft('colMonthlySaving')" width="135"><template #default="{ row }"><b class="finops-money">¥ {{ money(row.saving) }}</b></template></el-table-column>
        <el-table-column prop="description" :label="ft('colDesc')" min-width="280" show-overflow-tooltip />
        <el-table-column :label="ft('actions')" width="190" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openReport(row)">{{ ft('query') }}</el-button><el-button link type="primary" @click="openReport(row, true)">PDF Download</el-button><el-button link type="danger" @click="remove(row)">{{ ft('deleteAction') }}</el-button></template></el-table-column>
      </el-table>
      <el-empty v-if="!rows.length" :description="ft('emptyRecommendations')" />
    </section>
    <el-dialog v-model="strategyDialogVisible" :title="ft('dialogTitle')" width="480px" :close-on-click-modal="false">
      <el-radio-group v-model="strategy" class="strategy-options">
        <el-radio value="default">{{ ft('radioDefault') }}</el-radio>
        <el-radio value="ai" :disabled="!models.length">{{ ft('radioAi') }}</el-radio>
      </el-radio-group>
      <p class="strategy-hint">{{ strategy === 'ai' ? ft('hintAi') : ft('hintDefault') }}</p>
      <el-select v-if="strategy === 'ai'" v-model="modelId" :placeholder="ft('selectModelPlaceholder')" style="width: 100%"><el-option v-for="item in models" :key="item.id" :label="`${item.name} · ${item.model}`" :value="item.id" /></el-select>
      <template #footer><el-button @click="strategyDialogVisible = false">{{ ft('cancel') }}</el-button><el-button :loading="generating" type="primary" @click="generate">{{ ft('startGenerate') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.strategy-options { display: flex; flex-direction: column; gap: 14px; }
.strategy-hint { color: #64748b; font-size: 13px; margin: 18px 0; }
</style>
