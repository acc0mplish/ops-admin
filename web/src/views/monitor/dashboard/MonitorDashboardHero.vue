<script setup>
// I5-I (i5-plan §3.8·CW-DB) — MonitorDashboard.vue 2,600행 분할 자식.
// 원본 좌표는 각 블록 주석과 부모 커밋 메시지 참조. `page` 주입(§5 #11 승계):
// reactive(page)로 ref/computed가 언랩되어 v-model 재배선이 동작한다.
import { uiT } from '../../../utils/english-hardcoding-i18n'
import { mt } from '../../../utils/monitor-i18n'
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
      <section class="dashboard-hero">
        <div>
          <div class="eyebrow">{{ uiT('monitoringDashboard') }}</div>
          <h2>{{ page.activeDashboard?.name || page.pageTitle }}</h2>
          <p>{{ page.activeDashboard?.description || page.pageDescription }}</p>
          <div v-if="page.activeDashboard" class="layout-hint">
            {{ page.isListLayout ? mt('inspectionLayoutHint') : mt('gridLayoutHint') }}
          </div>
          <div v-if="page.activeDashboard" class="dashboard-context">
            <span><i class="health-dot"></i>{{ page.currentDatasourceName }}</span>
            <span>{{ mt('lastUpdated', { time: page.lastRefreshText }) }}</span>
            <span>{{ mt('activePanelCount', { count: page.activePanels.length }) }}</span>
          </div>
        </div>
		<div class="hero-actions">
          <el-select v-model="page.selectedDatasourceId" :placeholder="mt('selectDatasource')" style="width: 180px" @change="page.handleDatasourceChange">
            <el-option v-for="item in page.datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
		  </el-select>
		  <el-select v-model="page.timeRangeSeconds" style="width: 130px" @change="page.refreshAllPanels">
			<el-option :label="mt('last15Minutes')" :value="900" />
			<el-option :label="mt('lastHour')" :value="3600" />
			<el-option :label="mt('last6Hours')" :value="21600" />
			<el-option :label="mt('last24Hours')" :value="86400" />
		  </el-select>
          <el-select v-model="page.autoRefreshSeconds" style="width: 130px" @change="page.restartAutoRefresh">
            <el-option :label="mt('autoRefreshOff')" :value="0" />
            <el-option :label="mt('refresh10s')" :value="10" />
            <el-option :label="mt('refresh30s')" :value="30" />
            <el-option :label="mt('refresh60s')" :value="60" />
          </el-select>
          <el-button @click="page.refreshAllPanels" :disabled="!page.activePanels.length">{{ mt('refreshAll') }}</el-button>
          <el-button v-if="!page.isListLayout" @click="page.toggleFullscreen" :disabled="!page.activeDashboard">{{ page.isFullscreen ? mt('exitFullscreen') : mt('fullscreenDashboard') }}</el-button>
          <el-button v-else @click="page.exportInspectionReportPdf" :disabled="!page.activeDashboard">{{ uiT('inspectionReportPdfExport') }}</el-button>
          <el-button v-if="!page.isFullscreen" @click="page.openEditDashboard" :disabled="!page.activeDashboard">{{ mt('editTitle', { title: page.pageTitle }) }}</el-button>
          <el-button v-if="!page.isFullscreen" type="danger" plain @click="page.handleDeleteDashboard" :disabled="!page.activeDashboard">{{ mt('deleteTitle', { title: page.pageTitle }) }}</el-button>
          <el-button v-if="!page.isFullscreen" type="primary" @click="page.openCreatePanel">{{ mt('addPanel') }}</el-button>
        </div>
      </section>

      <section class="dashboard-summary">
        <div class="summary-card">
          <span>{{ mt('panelCount') }}</span>
          <strong>{{ page.panels.length }}</strong>
        </div>
        <div class="summary-card">
          <span>{{ mt('activePanels') }}</span>
          <strong>{{ page.activePanels.length }}</strong>
        </div>
        <div class="summary-card">
          <span>{{ mt('refreshInterval') }}</span>
          <strong>{{ page.autoRefreshSeconds ? `${page.autoRefreshSeconds}s` : mt('off') }}</strong>
        </div>
        <div class="summary-card">
          <span>{{ mt('dashboardStatus') }}</span>
          <strong :class="`state-${page.dashboardHealth.type}`">{{ page.dashboardHealth.text }}</strong>
        </div>
      </section>
</template>

<style scoped>
.dashboard-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 24px;
  margin-bottom: 16px;
  border: 1px solid #dce7f7;
  border-radius: 16px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96) 0%, rgba(248, 251, 255, 0.92) 58%, rgba(230, 241, 255, 0.92) 100%),
    radial-gradient(circle at right top, rgba(37, 99, 235, 0.18), transparent 36%);
}
.eyebrow {
  margin-bottom: 8px;
  color: #2f63bf;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.dashboard-hero h2 {
  margin: 0 0 8px;
  color: #0f1f3d;
  font-size: 30px;
}
.dashboard-hero p {
  margin: 0;
  color: #6f7f9b;
}
.layout-hint {
  display: inline-flex;
  margin-top: 12px;
  padding: 6px 10px;
  border-radius: 999px;
  color: #2f63bf;
  background: rgba(47, 99, 191, 0.09);
  font-size: 12px;
  font-weight: 700;
}
.hero-actions {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 520px;
}
.dashboard-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(160px, 1fr));
  gap: 14px;
  margin-bottom: 16px;
}
.summary-card {
  position: relative;
  padding: 16px;
  border: 1px solid #dce7f7;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
  overflow: hidden;
}
.summary-card::after {
  content: '';
  position: absolute;
  right: 14px;
  bottom: 12px;
  width: 54px;
  height: 8px;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(37, 99, 235, 0.16), rgba(34, 197, 94, 0.22));
}
.summary-card span {
  display: block;
  color: #7888a6;
}
.summary-card strong {
  display: block;
  margin-top: 8px;
  color: #0f1f3d;
  font-size: 24px;
}
.state-success { color: #16a34a !important; }
.state-danger { color: #dc2626 !important; }
.state-info { color: #64748b !important; }
.observability-canvas .dashboard-hero {
  align-items: flex-start;
  padding: 16px 18px;
  margin-bottom: 10px;
  border-color: #d9e1ec;
  border-radius: 6px;
  background: #fff;
  box-shadow: none;
}
.observability-canvas .eyebrow {
  margin-bottom: 4px;
  color: #2563eb;
  font-size: 10px;
  letter-spacing: 0.06em;
}
.observability-canvas .dashboard-hero h2 {
  margin-bottom: 4px;
  font-size: 22px;
  letter-spacing: 0;
}
.observability-canvas .dashboard-hero p {
  font-size: 13px;
}
.observability-canvas .layout-hint {
  display: none;
}
.dashboard-context {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 10px;
  color: #64748b;
  font-size: 12px;
}
.dashboard-context span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.health-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.12);
}
.observability-canvas .hero-actions {
  align-items: center;
  min-width: 0;
  max-width: 760px;
  gap: 6px;
}
.observability-canvas .hero-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
.observability-canvas .dashboard-summary {
  grid-template-columns: repeat(4, minmax(130px, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}
.observability-canvas .summary-card {
  min-height: 72px;
  padding: 12px 14px;
  border-color: #d9e1ec;
  border-radius: 6px;
  background: #fff;
}
.observability-canvas .summary-card::after {
  right: 12px;
  bottom: 11px;
  width: 34px;
  height: 3px;
  background: #3b82f6;
  opacity: 0.55;
}
.observability-canvas .summary-card span {
  font-size: 12px;
}
.observability-canvas .summary-card strong {
  margin-top: 5px;
  font-size: 21px;
}
.observability-canvas:fullscreen .dashboard-hero {
  align-items: center;
  min-height: 94px;
  padding: 12px 16px;
  border-color: #283442;
  background: #111923;
}
.observability-canvas:fullscreen .eyebrow {
  color: #60a5fa;
}
.observability-canvas:fullscreen .dashboard-hero h2 {
  color: #f1f5f9;
}
.observability-canvas:fullscreen .dashboard-hero p,
.observability-canvas:fullscreen .dashboard-context {
  color: #8291a5;
}
.observability-canvas:fullscreen .hero-actions {
  max-width: none;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-input__wrapper),
.observability-canvas:fullscreen .hero-actions :deep(.el-select__wrapper) {
  border: 1px solid #334155;
  background: #0c131c;
  box-shadow: none;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-input__inner),
.observability-canvas:fullscreen .hero-actions :deep(.el-select__selected-item),
.observability-canvas:fullscreen .hero-actions :deep(.el-select__placeholder) {
  color: #cbd5e1;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-button) {
  border-color: #334155;
  background: #17202c;
  color: #d6e0eb;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-button:hover) {
  border-color: #4f8cff;
  background: #1c2a3d;
  color: #fff;
}
.observability-canvas:fullscreen .dashboard-summary {
  gap: 6px;
}
.observability-canvas:fullscreen .summary-card {
  min-height: 64px;
  border-color: #283442;
  background: #111923;
}
.observability-canvas:fullscreen .summary-card::after {
  display: none;
}
.observability-canvas:fullscreen .summary-card span {
  color: #7f8ea3;
}
.observability-canvas:fullscreen .summary-card strong {
  color: #e7edf5;
}
.observability-canvas:fullscreen .dashboard-hero {
  min-height: 62px;
  margin-bottom: 6px;
  padding: 8px 12px;
}
.observability-canvas:fullscreen .dashboard-hero p,
.observability-canvas:fullscreen .eyebrow {
  display: none;
}
.observability-canvas:fullscreen .dashboard-hero h2 {
  margin: 0;
  font-size: 18px;
}
.observability-canvas:fullscreen .dashboard-context {
  gap: 10px;
  margin-top: 5px;
  font-size: 10px;
}
.observability-canvas:fullscreen .hero-actions {
  flex-wrap: nowrap;
  gap: 4px;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-select) {
  width: 128px !important;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-select:first-child) {
  width: 150px !important;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-select__wrapper),
.observability-canvas:fullscreen .hero-actions :deep(.el-button) {
  min-height: 30px;
  height: 30px;
  padding-top: 4px;
  padding-bottom: 4px;
  font-size: 12px;
}
.observability-canvas:fullscreen .dashboard-summary {
  display: none;
}
@media (max-width: 1280px) {
  .hero-actions {
    min-width: 0;
  }
  .dashboard-summary {
    grid-template-columns: repeat(2, minmax(220px, 1fr));
  }
}
.dashboard-main:fullscreen .dashboard-hero,
.dashboard-main:fullscreen .dashboard-summary {
  background: rgba(15, 23, 42, 0.92);
  border-color: rgba(96, 165, 250, 0.28);
  color: #dbeafe;
}
</style>
