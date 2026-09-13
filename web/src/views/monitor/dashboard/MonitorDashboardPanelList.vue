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
      <section class="inspection-list">
        <div class="inspection-command-bar">
          <div class="inspection-filter">
            <button :class="{ active: page.inspectionFilter === 'all' }" @click="page.inspectionFilter = 'all'">{{ mt('filterAllCount', { count: page.panels.length }) }}</button>
            <button :class="{ active: page.inspectionFilter === 'danger' }" @click="page.inspectionFilter = 'danger'">{{ mt('filterProblemCount', { count: page.inspectionSummary.danger }) }}</button>
            <button :class="{ active: page.inspectionFilter === 'warning' }" @click="page.inspectionFilter = 'warning'">{{ mt('filterReviewCount', { count: page.inspectionSummary.warning }) }}</button>
            <button :class="{ active: page.inspectionFilter === 'healthy' }" @click="page.inspectionFilter = 'healthy'">{{ mt('filterHealthyCount', { count: page.inspectionSummary.healthy }) }}</button>
          </div>
          <div class="inspection-command-actions">
            <span>{{ mt('inspectionDisplayHint', { count: page.inspectionPanels.length }) }}</span>
            <el-button @click="page.refreshProblemPanels" :disabled="!page.activePanels.length">{{ mt('reinspectProblems') }}</el-button>
            <el-button type="primary" @click="page.refreshAllPanels" :disabled="!page.activePanels.length">{{ mt('runInspection') }}</el-button>
          </div>
        </div>
        <div class="inspection-head">
          <div>
            <h3>{{ uiT('inspectionPanel') }}</h3>
            <p>{{ mt('inspectionHeadDesc') }}</p>
          </div>
          <el-button @click="page.refreshAllPanels" :disabled="!page.activePanels.length">{{ mt('runInspection') }}</el-button>
        </div>
        <el-table :data="page.inspectionPanels" class="inspection-table" row-key="id" :empty-text="mt('noFilteredPanels')">
          <el-table-column :label="uiT('panel')" min-width="180">
            <template #default="{ row }">
              <div class="inspection-name">
                <strong>{{ row.title }}</strong>
                <span>{{ row.description || mt('noDescription') }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="mt('status')" width="110">
            <template #default="{ row }">
              <el-tag :type="page.panelStateType(row)" effect="light">{{ page.panelState(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="mt('currentValue')" width="150">
            <template #default="{ row }">
              <strong class="inspection-value">{{ page.panelDisplayValue(row) }}</strong>
            </template>
          </el-table-column>
          <el-table-column :label="mt('returnedSeries')" width="110">
            <template #default="{ row }">{{ page.panelResultCount(row) }}</template>
          </el-table-column>
          <el-table-column label="Datasource / Type" width="180">
            <template #default="{ row }">
              <div class="inspection-meta">
                <span>{{ page.datasourceOptions.find((item) => item.id === page.selectedDatasourceId)?.name || row.datasourceName || '-' }}</span>
                <b>{{ row.chartType }}</b>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="PromQL" min-width="320" show-overflow-tooltip>
            <template #default="{ row }">
              <code class="inspection-promql">{{ row.promql }}</code>
            </template>
          </el-table-column>
          <el-table-column :label="mt('actions')" width="190" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="page.refreshPanel(row)">Refresh</el-button>
              <el-button link type="primary" @click="page.openEditPanel(row)">{{ mt('edit') }}</el-button>
              <el-button link type="danger" @click="page.handleDeletePanel(row)">{{ mt('delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </section>
</template>

<style scoped>
.inspection-list {
  padding: 18px;
  border: 1px solid rgba(169, 190, 222, 0.72);
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(250, 253, 255, 0.92)),
    radial-gradient(circle at 90% 0%, rgba(37, 99, 235, 0.12), transparent 32%);
  box-shadow: 0 14px 32px rgba(31, 54, 92, 0.08);
}
.inspection-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.inspection-head h3 {
  margin: 0 0 6px;
  color: #10213f;
  font-size: 20px;
}
.inspection-head p {
  margin: 0;
  color: #7282a0;
}
.inspection-command-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin: -2px 0 18px;
  padding: 10px 12px;
  border: 1px solid #e0e9f6;
  border-radius: 12px;
  background: linear-gradient(100deg, #f8fbff, #f1f6ff);
}
.inspection-command-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  color: #7282a0;
  font-size: 12px;
}
.inspection-filter {
  display: inline-flex;
  padding: 3px;
  border: 1px solid #dce7f7;
  border-radius: 10px;
  background: #eef4fc;
}
.inspection-filter button {
  padding: 6px 9px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #667895;
  cursor: pointer;
  font-size: 12px;
}
.inspection-filter button:hover,
.inspection-filter button.active {
  background: #fff;
  color: #2463d4;
  box-shadow: 0 2px 6px rgba(36, 99, 212, 0.12);
}
.inspection-table {
  border-radius: 14px;
  overflow: hidden;
}
.inspection-name strong,
.inspection-name span,
.inspection-meta span,
.inspection-meta b {
  display: block;
}
.inspection-name strong {
  color: #10213f;
}
.inspection-name span {
  margin-top: 4px;
  color: #8a99b4;
  font-size: 12px;
}
.inspection-value {
  color: #1554d1;
  font-size: 18px;
}
.inspection-meta span {
  color: #334155;
}
.inspection-meta b {
  margin-top: 3px;
  color: #8a99b4;
  font-size: 12px;
  font-weight: 600;
}
.inspection-promql {
  display: inline-block;
  max-width: 100%;
  padding: 5px 8px;
  border-radius: 8px;
  background: #0f172a;
  color: #b9d6ff;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
}
@media (max-width: 760px) {
  .inspection-command-bar {
    align-items: flex-start;
    flex-direction: column;
  }
  .inspection-command-actions { justify-content: flex-start; }
}
</style>
