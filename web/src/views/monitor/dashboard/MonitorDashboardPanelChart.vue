<script setup>
// I5-I (i5-plan §3.8·CW-DB) — MonitorDashboard.vue 2,600행 분할 자식(차트 본체 5종).
import { mt } from '../../../utils/monitor-i18n'
defineProps({
  page: {
    type: Object,
    required: true
  },
  panel: {
    type: Object,
    required: true
  }
})
</script>

<template>
          <template v-if="panel.chartType === 'table'">
            <el-table :data="page.panelRows(panel)" size="small" :height="page.isFullscreen ? 126 : 250">
              <el-table-column label="Metric" min-width="240" show-overflow-tooltip>
                <template #default="{ row }">{{ page.metricText(row.metric) }}</template>
              </el-table-column>
              <el-table-column label="Value" width="130">
                <template #default="{ row }">{{ row.value?.[1] ?? '-' }}</template>
              </el-table-column>
            </el-table>
          </template>

          <template v-else-if="panel.chartType === 'bar'">
            <div class="bar-chart">
              <div v-for="item in page.barRows(panel)" :key="item.name" class="bar-row">
                <span>{{ item.name }}</span>
                <div><i :style="{ width: `${item.percent}%` }"></i></div>
                <b>{{ item.displayValue }}</b>
              </div>
            </div>
            <div class="panel-query" :title="panel.promql">{{ panel.promql }}</div>
          </template>

          <template v-else-if="panel.chartType === 'gauge'">
            <div class="gauge-wrap">
              <div class="gauge" :style="{ '--value': `${page.gaugePercent(panel) * 3.6}deg` }">
                <div>
                  <strong>{{ page.panelValue(panel) }}</strong>
                  <span>{{ page.unitText(panel.unit) }}</span>
                </div>
              </div>
            </div>
            <div class="panel-query" :title="panel.promql">{{ panel.promql }}</div>
          </template>

          <template v-else-if="panel.chartType === 'line'">
            <div class="trend-summary">
              <div class="trend-current">
                <span>{{ mt('currentValue') }}</span>
                <strong>{{ page.panelDisplayValue(panel) }}</strong>
              </div>
              <div class="trend-stats">
                <span>{{ mt('minLabel') }} <b>{{ page.panelStats(panel).min }}</b></span>
                <span>{{ mt('avgLabel') }} <b>{{ page.panelStats(panel).avg }}</b></span>
                <span>{{ mt('maxLabel') }} <b>{{ page.panelStats(panel).max }}</b></span>
              </div>
            </div>
            <div class="trend-chart">
              <svg class="sparkline" viewBox="0 0 100 52" preserveAspectRatio="none">
                <defs>
                  <linearGradient :id="`trend-area-${panel.id}`" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#4f8cff" stop-opacity="0.28" />
                    <stop offset="100%" stop-color="#4f8cff" stop-opacity="0.02" />
                  </linearGradient>
                </defs>
                <line v-for="y in [8, 18, 28, 38, 48]" :key="y" x1="0" :y1="y" x2="100" :y2="y" class="chart-grid-line" />
                <polygon :points="page.sparklineAreaPoints(panel)" :fill="`url(#trend-area-${panel.id})`" />
                <polyline
                  v-for="(series, index) in page.panelLineSeries(panel)"
                  :key="`${series.name}-${index}`"
                  class="line-series"
                  :points="series.points"
                  :style="{ stroke: series.color }"
                />
              </svg>
              <div class="trend-axis"><span>{{ mt('startLabel') }}</span><span>{{ mt('nowLabel') }}</span></div>
            </div>
            <div v-if="page.panelLineSeries(panel).length > 1" class="trend-legend">
              <span v-for="(series, index) in page.panelLineSeries(panel).slice(0, 4)" :key="`${series.name}-legend-${index}`">
                <i :style="{ background: series.color }"></i>{{ series.name }}
              </span>
              <span v-if="page.panelLineSeries(panel).length > 4">+{{ page.panelLineSeries(panel).length - 4 }}</span>
            </div>
            <div class="panel-query" :title="panel.promql">{{ panel.promql }}</div>
          </template>

          <template v-else>
            <div class="stat-row">
              <div>
                <div class="stat-value">{{ page.panelValue(panel) }}<small>{{ page.unitText(panel.unit) }}</small></div>
                <div class="stat-caption">
                  <span></span>
                  {{ mt('liveSampling') }}
                </div>
              </div>
              <svg v-if="panel.chartType === 'line'" class="sparkline" viewBox="0 0 100 52" preserveAspectRatio="none">
                <polyline :points="page.sparklinePoints(panel)" />
              </svg>
            </div>
            <div class="promql">{{ panel.promql }}</div>
          </template>
</template>

<style scoped>
.stat-row {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 180px 1fr;
  align-items: center;
  gap: 18px;
}
.stat-value {
  color: #1554d1;
  font-size: 44px;
  font-weight: 850;
  line-height: 1.1;
  text-shadow: 0 8px 24px rgba(37, 99, 235, 0.18);
}
.stat-value small {
  margin-left: 6px;
  color: #73829f;
  font-size: 14px;
}
.stat-caption {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin-top: 12px;
  padding: 5px 10px;
  border-radius: 999px;
  color: #64748b;
  background: #eef5ff;
  font-size: 12px;
}
.stat-caption span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 0 4px rgba(34, 197, 94, 0.14);
}
.sparkline {
  width: 100%;
  height: 92px;
}
.sparkline polyline {
  fill: none;
  stroke: #3b82f6;
  stroke-width: 4;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.promql {
  position: relative;
  z-index: 1;
  margin-top: 18px;
  padding: 10px;
  border-radius: 10px;
  background: #0f172a;
  color: #b9d6ff;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.bar-chart {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.bar-row {
  display: grid;
  grid-template-columns: 160px 1fr 84px;
  align-items: center;
  gap: 10px;
  color: #566781;
  font-size: 13px;
}
.bar-row span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bar-row div {
  height: 11px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(226, 235, 247, 0.9);
}
.bar-row i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2563eb, #06b6d4 48%, #22c55e);
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.18);
}
.bar-row b {
  color: #10213f;
  text-align: right;
}
.gauge-wrap {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  min-height: 185px;
}
.gauge {
  position: relative;
  display: grid;
  place-items: center;
  width: 180px;
  height: 180px;
  border-radius: 50%;
  background: conic-gradient(#2563eb var(--value), #e5edf8 0deg);
  box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.08), 0 18px 32px rgba(37, 99, 235, 0.16);
}
.gauge::before {
  content: '';
  position: absolute;
}
.gauge > div {
  display: grid;
  place-items: center;
  width: 126px;
  height: 126px;
  border-radius: 50%;
  background: #fff;
}
.gauge strong {
  color: #10213f;
  font-size: 34px;
}
.gauge span {
  color: #75859f;
  font-size: 13px;
}
.chart-panel .bar-chart {
  max-height: 172px;
  gap: 9px;
  overflow: auto;
}
.chart-panel .bar-row {
  grid-template-columns: minmax(90px, 132px) 1fr 76px;
  font-size: 12px;
}
.chart-panel .bar-row div {
  height: 7px;
  border-radius: 2px;
  background: #edf1f6;
}
.chart-panel .bar-row i {
  border-radius: 2px;
  background: linear-gradient(90deg, #2563eb, #14b8a6);
  box-shadow: none;
}
.chart-panel .gauge-wrap {
  min-height: 142px;
}
.chart-panel .gauge {
  width: 138px;
  height: 138px;
  box-shadow: none;
}
.chart-panel .gauge > div {
  width: 100px;
  height: 100px;
}
.chart-panel .gauge strong {
  font-size: 28px;
}
.chart-panel .stat-row {
  display: block;
}
.chart-panel .stat-value {
  font-size: 36px;
  text-shadow: none;
}
.trend-summary {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px 0;
}
.trend-current span,
.trend-current strong {
  display: block;
}
.trend-current span {
  color: #7c899d;
  font-size: 11px;
}
.trend-current strong {
  margin-top: 2px;
  color: #172033;
  font-size: 24px;
}
.trend-stats {
  display: flex;
  gap: 14px;
  color: #8a96a8;
  font-size: 10px;
}
.trend-stats b {
  margin-left: 3px;
  color: #475569;
  font-weight: 600;
}
.trend-chart {
  padding: 5px 14px 0;
}
.trend-chart .sparkline {
  display: block;
  height: 104px;
}
.trend-chart .sparkline polyline {
  stroke-width: 1.8;
  vector-effect: non-scaling-stroke;
}
.chart-grid-line {
  stroke: #e7ecf3;
  stroke-width: 0.5;
  vector-effect: non-scaling-stroke;
}
.trend-axis {
  display: flex;
  justify-content: space-between;
  margin-top: 2px;
  color: #a0aaba;
  font-size: 9px;
}
.trend-legend {
  display: flex;
  gap: 10px;
  min-height: 16px;
  padding: 2px 14px 0;
  overflow: hidden;
  color: #64748b;
  font-size: 9px;
  white-space: nowrap;
}
.trend-legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.trend-legend i {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 50%;
}
.panel-query,
.chart-panel .promql {
  position: absolute;
  right: 12px;
  bottom: 8px;
  left: 12px;
  z-index: 1;
  margin: 0;
  padding: 4px 7px;
  overflow: hidden;
  border: 1px solid #e0e7f0;
  border-radius: 3px;
  background: #f6f8fb;
  color: #64748b;
  font-family: Consolas, Monaco, monospace;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chart-panel.panel-table .el-table {
  padding: 10px 12px 0;
}
.observability-canvas:fullscreen .trend-stats b {
  color: #bdc9d8;
}
.observability-canvas:fullscreen .chart-panel .stat-value {
  color: #60a5fa;
}
.observability-canvas:fullscreen .chart-panel .stat-value small {
  color: #7f8ea3;
}
.observability-canvas:fullscreen .chart-panel.panel-stat .stat-row {
  margin-top: 34px;
}
.observability-canvas:fullscreen .chart-panel .stat-caption {
  color: #9aabc0;
  background: #182330;
}
.observability-canvas:fullscreen .chart-panel .gauge {
  background: conic-gradient(#3b82f6 var(--value), #263241 0deg);
}
.observability-canvas:fullscreen .chart-panel .gauge > div {
  background: #111923;
}
.observability-canvas:fullscreen .chart-panel .gauge strong {
  color: #e8eef6;
}
.observability-canvas:fullscreen .chart-panel .gauge span {
  color: #7f8ea3;
}
.observability-canvas:fullscreen .chart-panel .bar-row div {
  background: #263241;
}
.observability-canvas:fullscreen .chart-grid-line {
  stroke: #263241;
}
.observability-canvas:fullscreen .panel-query,
.observability-canvas:fullscreen .chart-panel .promql {
  display: none;
}
.observability-canvas:fullscreen .chart-panel .el-table {
  --el-table-bg-color: #111923;
  --el-table-tr-bg-color: #111923;
  --el-table-header-bg-color: #151e29;
  --el-table-border-color: #283442;
  --el-table-text-color: #c7d2df;
  --el-table-header-text-color: #8fa0b5;
}
.observability-canvas:fullscreen .chart-panel.panel-stat .stat-row,
.observability-canvas:fullscreen .chart-panel .stat-row {
  margin: 20px 10px 8px;
}
.observability-canvas:fullscreen .chart-panel .stat-value {
  font-size: 30px;
}
.observability-canvas:fullscreen .chart-panel .stat-caption {
  margin-top: 7px;
  padding: 3px 7px;
  font-size: 9px;
}
.observability-canvas:fullscreen .chart-panel .gauge-wrap {
  min-height: 112px;
  margin: 6px;
}
.observability-canvas:fullscreen .chart-panel .gauge {
  width: 96px;
  height: 96px;
}
.observability-canvas:fullscreen .chart-panel .gauge > div {
  width: 68px;
  height: 68px;
}
.observability-canvas:fullscreen .chart-panel .gauge strong {
  font-size: 20px;
}
.observability-canvas:fullscreen .chart-panel .gauge span {
  font-size: 10px;
}
.observability-canvas:fullscreen .chart-panel .bar-chart {
  max-height: 112px;
  margin: 8px 10px;
  gap: 5px;
}
.observability-canvas:fullscreen .chart-panel .bar-row {
  grid-template-columns: minmax(70px, 112px) 1fr 58px;
  gap: 6px;
  font-size: 10px;
}
.observability-canvas:fullscreen .chart-panel .bar-row div {
  height: 5px;
}
.observability-canvas:fullscreen .trend-summary {
  padding: 7px 9px 0;
}
.observability-canvas:fullscreen .trend-current strong {
  font-size: 17px;
}
.observability-canvas:fullscreen .trend-stats {
  gap: 7px;
  font-size: 8px;
}
.observability-canvas:fullscreen .trend-chart {
  padding: 2px 9px 0;
}
.observability-canvas:fullscreen .trend-chart .sparkline {
  height: 70px;
}
.observability-canvas:fullscreen .trend-legend {
  display: none;
}
.observability-canvas:fullscreen .chart-panel.panel-table .el-table {
  padding: 5px 7px 0;
  font-size: 10px;
}
.chart-panel .bar-chart,
.chart-panel .gauge-wrap,
.chart-panel .stat-row {
  margin: 14px;
}
.observability-canvas:fullscreen .trend-current strong,
.observability-canvas:fullscreen .chart-panel .bar-row b {
  color: #e5edf6;
}
.observability-canvas:fullscreen .trend-current span,
.observability-canvas:fullscreen .trend-stats,
.observability-canvas:fullscreen .trend-axis,
.observability-canvas:fullscreen .trend-legend,
.observability-canvas:fullscreen .chart-panel .bar-row {
  color: #8493a7;
}
</style>
