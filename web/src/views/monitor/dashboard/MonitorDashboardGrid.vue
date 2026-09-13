<script setup>
// I5-I (i5-plan §3.8·CW-DB) — MonitorDashboard.vue 2,600행 분할 자식(그리드+카드 chrome).
import { mt } from '../../../utils/monitor-i18n'
import MonitorDashboardPanelChart from './MonitorDashboardPanelChart.vue'
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
      <section class="dashboard-grid-shell">
        <div class="dashboard-grid-toolbar">
          <div class="dashboard-grid-status">
            <span class="status-chip healthy"><i></i>{{ mt('filterHealthyCount', { count: page.inspectionSummary.healthy }) }}</span>
            <span class="status-chip warning"><i></i>{{ mt('needCheckCount', { count: page.inspectionSummary.warning }) }}</span>
            <span class="status-chip danger"><i></i>{{ mt('filterProblemCount', { count: page.inspectionSummary.danger }) }}</span>
            <span class="status-chip muted"><i></i>{{ mt('disabledCount', { count: page.inspectionSummary.disabled }) }}</span>
          </div>
          <div class="dashboard-grid-actions">
            <span>{{ mt('lastRefresh', { time: page.lastRefreshText }) }}</span>
            <el-button size="small" @click="page.refreshProblemPanels" :disabled="!page.activePanels.length">{{ mt('reinspectProblems') }}</el-button>
          </div>
        </div>
        <div class="panel-grid" :class="{ 'k8s-panel-grid': page.isK8sDashboard }">
        <div
          v-for="panel in page.panels"
          :key="panel.id"
          class="metric-panel chart-panel"
           :class="[`panel-${panel.chartType}`, { disabled: panel.status !== 1 }]"
           :style="{ gridColumn: `span ${page.panelSpan(panel)}` }"
           v-loading="page.panelPending[panel.id]"
           :element-loading-background="page.isFullscreen ? 'rgba(8, 15, 28, 0.82)' : 'rgba(255, 255, 255, 0.72)'"
        >
          <div class="panel-glow"></div>
          <div class="panel-head">
            <div class="panel-identity">
              <div class="panel-title-row">
                <i class="panel-signal"></i>
                <strong>{{ panel.title }}</strong>
              </div>
              <span>{{ page.panelChartLabel(panel.chartType) }} · {{ mt('seriesCount', { count: page.panelResultCount(panel) }) }} · {{ page.currentDatasourceName }}</span>
            </div>
            <div class="panel-actions">
              <span class="panel-state" :class="`is-${page.panelStateType(panel)}`"><i></i>{{ page.panelState(panel) }}</span>
              <el-button link type="primary" @click="page.refreshPanel(panel)">Refresh</el-button>
              <el-button link type="primary" @click="page.openEditPanel(panel)">{{ mt('edit') }}</el-button>
              <el-button link type="danger" @click="page.handleDeletePanel(panel)">{{ mt('delete') }}</el-button>
            </div>
          </div>

          <div v-if="page.panelResults[panel.id]?.error" class="panel-error">{{ page.panelResults[panel.id].error }}</div>

          <MonitorDashboardPanelChart v-else :page="page" :panel="panel" />
        </div>
        </div>
      </section>
</template>

<style scoped>
.panel-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(240px, 1fr));
  gap: 14px;
}
.dashboard-grid-shell {
  min-width: 0;
}
.dashboard-grid-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(169, 190, 222, 0.72);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.74);
  backdrop-filter: blur(12px);
}
.dashboard-grid-status,
.dashboard-grid-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.dashboard-grid-actions {
  justify-content: flex-end;
  color: #71829e;
  font-size: 12px;
}
.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 9px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}
.status-chip i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.status-chip.healthy { color: #16834b; background: #eaf8ef; }
.status-chip.warning { color: #a56608; background: #fff7df; }
.status-chip.danger { color: #c24141; background: #fff0f0; }
.status-chip.muted { color: #64748b; background: #eef2f7; }
.k8s-dashboard-main {
  background:
    radial-gradient(circle at 10% 8%, rgba(34, 197, 94, 0.12), transparent 24%),
    radial-gradient(circle at 78% 0%, rgba(59, 130, 246, 0.16), transparent 26%),
    linear-gradient(180deg, #0b1117 0%, #111820 100%);
  border: 1px solid #26313c;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04), 0 18px 34px rgba(2, 6, 23, 0.28);
}
.k8s-dashboard-main .dashboard-hero,
.k8s-dashboard-main .dashboard-summary {
  border-color: #26313c;
  background:
    linear-gradient(180deg, rgba(20, 29, 38, 0.96), rgba(15, 23, 31, 0.94)),
    radial-gradient(circle at right top, rgba(34, 197, 94, 0.12), transparent 32%);
  box-shadow: none;
}
.k8s-dashboard-main .eyebrow {
  color: #93c5fd;
}
.k8s-dashboard-main .dashboard-hero h2,
.k8s-dashboard-main .panel-head strong,
.k8s-dashboard-main .summary-card strong {
  color: #dbeafe;
}
.k8s-dashboard-main .dashboard-hero p,
.k8s-dashboard-main .layout-hint,
.k8s-dashboard-main .panel-head span,
.k8s-dashboard-main .summary-card span {
  color: #94a3b8;
}
.k8s-dashboard-main .layout-hint {
  background: rgba(34, 197, 94, 0.1);
  color: #86efac;
}
.k8s-dashboard-main .summary-card {
  border-color: #26313c;
  background: linear-gradient(180deg, rgba(18, 27, 36, 0.92), rgba(12, 18, 26, 0.92));
}
.k8s-dashboard-main .summary-card::after {
  background: linear-gradient(90deg, #22c55e, #eab308, #3b82f6);
  opacity: 0.72;
}
.k8s-panel-grid {
  grid-template-columns: repeat(6, minmax(180px, 1fr));
  gap: 6px;
}
.metric-panel {
  position: relative;
  min-height: 260px;
  padding: 18px;
  border: 1px solid rgba(169, 190, 222, 0.72);
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(250, 253, 255, 0.92)),
    radial-gradient(circle at 80% 0%, rgba(37, 99, 235, 0.16), transparent 32%);
  box-shadow: 0 14px 32px rgba(31, 54, 92, 0.08);
  overflow: hidden;
}
.metric-panel::before {
  content: '';
  position: absolute;
  inset: 0;
  border-top: 3px solid rgba(37, 99, 235, 0.72);
  pointer-events: none;
}
.panel-glow {
  position: absolute;
  right: -36px;
  top: -46px;
  width: 128px;
  height: 128px;
  border-radius: 50%;
  background: rgba(37, 99, 235, 0.12);
  filter: blur(2px);
  pointer-events: none;
}
.panel-gauge::before {
  border-top-color: rgba(34, 197, 94, 0.78);
}
.panel-bar::before {
  border-top-color: rgba(14, 165, 233, 0.82);
}
.panel-line::before {
  border-top-color: rgba(99, 102, 241, 0.82);
}
.panel-table::before {
  border-top-color: rgba(100, 116, 139, 0.76);
}
.k8s-metric-panel {
  min-height: 168px;
  padding: 11px;
  border-radius: 4px;
  border-color: #293541;
  background:
    linear-gradient(180deg, rgba(21, 30, 38, 0.96), rgba(13, 19, 26, 0.98)),
    radial-gradient(circle at 80% 0%, rgba(34, 197, 94, 0.08), transparent 34%);
  box-shadow: none;
}
.k8s-metric-panel::before {
  border-top-width: 2px;
  border-top-color: #22c55e;
}
.k8s-metric-panel.panel-gauge::before {
  border-top-color: #ef4444;
}
.k8s-metric-panel.panel-bar::before {
  border-top-color: #eab308;
}
.k8s-metric-panel.panel-line::before {
  border-top-color: #3b82f6;
}
.k8s-metric-panel .panel-glow {
  display: none;
}
.k8s-metric-panel .panel-head {
  margin-bottom: 10px;
}
.k8s-metric-panel .panel-head strong {
  font-size: 13px;
}
.k8s-metric-panel .panel-head span {
  margin-top: 3px;
  font-size: 11px;
}
.k8s-metric-panel .panel-actions {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.k8s-metric-panel:hover .panel-actions {
  opacity: 1;
}
.k8s-metric-panel .stat-row {
  display: block;
}
.k8s-metric-panel .stat-value {
  color: #4ade80;
  font-size: 32px;
  text-shadow: 0 0 18px rgba(74, 222, 128, 0.18);
}
.k8s-metric-panel.panel-gauge .stat-value,
.k8s-metric-panel.panel-gauge .gauge strong {
  color: #f87171;
}
.k8s-metric-panel .stat-caption {
  margin-top: 10px;
  color: #94a3b8;
  background: rgba(148, 163, 184, 0.1);
}
.k8s-metric-panel .promql {
  margin-top: 10px;
  padding: 7px 9px;
  border: 1px solid #26313c;
  border-radius: 4px;
  background: #070b10;
  color: #93c5fd;
  font-size: 11px;
}
.k8s-metric-panel .bar-chart {
  gap: 8px;
}
.k8s-metric-panel .bar-row {
  grid-template-columns: 118px 1fr 64px;
  gap: 8px;
  color: #a9b7c6;
  font-size: 11px;
}
.k8s-metric-panel .bar-row div {
  height: 8px;
  background: #1d2732;
}
.k8s-metric-panel .bar-row i {
  background: linear-gradient(90deg, #22c55e, #eab308, #ef4444);
  box-shadow: none;
}
.k8s-metric-panel .bar-row b {
  color: #dbeafe;
}
.k8s-metric-panel .gauge-wrap {
  min-height: 118px;
}
.k8s-metric-panel .gauge {
  width: 118px;
  height: 118px;
  background: conic-gradient(#ef4444 var(--value), #1d2732 0deg);
  box-shadow: none;
}
.k8s-metric-panel .gauge > div {
  width: 82px;
  height: 82px;
  background: #0e141b;
}
.k8s-metric-panel .gauge strong {
  font-size: 23px;
}
.k8s-metric-panel .sparkline {
  height: 72px;
  margin-top: 10px;
}
.k8s-metric-panel .sparkline polyline {
  stroke: #4ade80;
  stroke-width: 2.5;
}
.k8s-metric-panel .el-table {
  --el-table-bg-color: #0e141b;
  --el-table-tr-bg-color: #0e141b;
  --el-table-header-bg-color: #111c27;
  --el-table-border-color: #26313c;
  --el-table-text-color: #cbd5e1;
  --el-table-header-text-color: #93c5fd;
}
.k8s-metric-panel.panel-table {
  min-height: 260px;
}
.k8s-dashboard-main:fullscreen {
  background: #080d13;
}
.k8s-dashboard-main:fullscreen .k8s-metric-panel {
  background: linear-gradient(180deg, rgba(21, 30, 38, 0.96), rgba(13, 19, 26, 0.98));
  border-color: #293541;
}
.metric-panel.disabled {
  opacity: 0.55;
}
.panel-head {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 18px;
}
.panel-head strong {
  display: block;
  color: #10213f;
  font-size: 16px;
}
.panel-head span {
  display: block;
  margin-top: 5px;
  color: #8391aa;
  font-size: 12px;
}
.panel-actions {
  display: flex;
  gap: 4px;
  white-space: nowrap;
}
.panel-error {
  padding: 12px;
  border-radius: 10px;
  color: #b91c1c;
  background: #fff1f2;
  word-break: break-all;
}
.panel-grid {
  gap: 10px;
}
.chart-panel {
  min-height: 238px;
  padding: 0 0 34px;
  border: 1px solid #d8e0eb;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.035);
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}
.chart-panel:hover {
  border-color: #b9c8dc;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.07);
}
.chart-panel::before {
  display: none;
}
.chart-panel .panel-glow {
  display: none;
}
.chart-panel .panel-head {
  align-items: center;
  min-height: 55px;
  margin: 0;
  padding: 10px 12px;
  border-bottom: 1px solid #e4e9f0;
  background: #fbfcfe;
}
.panel-identity {
  min-width: 0;
}
.panel-title-row {
  display: flex;
  align-items: center;
  gap: 7px;
}
.panel-title-row strong {
  overflow: hidden;
  color: #172033;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.panel-signal {
  width: 3px;
  height: 15px;
  border-radius: 2px;
  background: #3b82f6;
}
.chart-panel.panel-gauge .panel-signal { background: #f59e0b; }
.chart-panel.panel-bar .panel-signal { background: #14b8a6; }
.chart-panel.panel-table .panel-signal { background: #64748b; }
.chart-panel .panel-head span {
  margin-top: 3px;
  color: #8491a5;
  font-size: 11px;
}
.chart-panel .panel-actions {
  align-items: center;
  opacity: 1;
}
.chart-panel .panel-actions :deep(.el-button) {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.chart-panel:hover .panel-actions :deep(.el-button) {
  opacity: 1;
}
.panel-state {
  display: inline-flex !important;
  align-items: center;
  gap: 5px;
  margin: 0 5px 0 0 !important;
  color: #64748b !important;
  white-space: nowrap;
}
.panel-state i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #94a3b8;
}
.panel-state.is-success i { background: #22c55e; }
.panel-state.is-warning i { background: #f59e0b; }
.panel-state.is-danger i { background: #ef4444; }
.chart-panel.panel-table {
  padding-bottom: 10px;
}
.observability-canvas:fullscreen .panel-grid {
  gap: 6px;
}
.observability-canvas:fullscreen .chart-panel {
  min-height: 224px;
  padding-bottom: 8px;
  border-color: #283442;
  background: #111923;
  box-shadow: none;
}
.observability-canvas:fullscreen .chart-panel:hover {
  border-color: #3b4c60;
  box-shadow: none;
}
.observability-canvas:fullscreen .chart-panel .panel-head {
  min-height: 50px;
  padding: 8px 10px;
  border-color: #283442;
  background: #151e29;
}
.observability-canvas:fullscreen .chart-panel .panel-actions :deep(.el-button) {
  display: none;
}
.observability-canvas:fullscreen .panel-error {
  border: 1px solid rgba(239, 68, 68, 0.26);
  background: rgba(127, 29, 29, 0.2);
  color: #fca5a5;
}
.observability-canvas:fullscreen .dashboard-grid-toolbar {
  margin-bottom: 6px;
  padding: 5px 8px;
  border-color: rgba(96, 165, 250, 0.24);
  background: rgba(15, 23, 42, 0.9);
}
.observability-canvas:fullscreen .status-chip {
  padding: 3px 7px;
  font-size: 10px;
}
.observability-canvas:fullscreen .panel-grid {
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 5px;
}
.observability-canvas:fullscreen .chart-panel {
  min-height: 164px;
}
.observability-canvas:fullscreen .chart-panel.panel-table {
  grid-column: span 3 !important;
  min-height: 174px;
}
.observability-canvas:fullscreen .chart-panel .panel-head {
  min-height: 40px;
  padding: 6px 8px;
}
.observability-canvas:fullscreen .panel-title-row {
  gap: 5px;
}
.observability-canvas:fullscreen .panel-title-row strong {
  font-size: 12px;
}
.observability-canvas:fullscreen .panel-signal {
  height: 13px;
}
.observability-canvas:fullscreen .panel-identity > span {
  display: none;
}
.observability-canvas:fullscreen .panel-state {
  margin-right: 0 !important;
  font-size: 9px !important;
}
@media (max-width: 1280px) {
  .panel-grid {
    grid-template-columns: repeat(2, minmax(220px, 1fr));
  }
  .dashboard-grid-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .dashboard-grid-actions { justify-content: flex-start; }
}
.dashboard-main:fullscreen .metric-panel {
  background: rgba(15, 23, 42, 0.92);
  border-color: rgba(96, 165, 250, 0.28);
  color: #dbeafe;
}
.chart-panel .panel-error {
  margin: 14px;
}
.observability-canvas:fullscreen .panel-title-row strong {
  color: #e5edf6;
}
.observability-canvas:fullscreen .chart-panel .panel-head span {
  color: #8493a7;
}
</style>
