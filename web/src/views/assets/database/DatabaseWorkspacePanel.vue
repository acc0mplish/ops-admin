<script setup>
import { at } from '../../../utils/asset-i18n'

// I5-H — template/CSS child extracted from views/assets/DatabaseWorkbench.vue
// (i5-plan §3.8 — CW-WB: database/ 자식). Identifiers are rewired to the
// page bundle passed by the parent (:page 주입 패턴 승계 — §5 #11). All state
// and handlers remain owned by the parent view and its composables.
const props = defineProps({ page: { type: Object, required: true } })
</script>

<template>
              <div class="dbms-content page-card">
          <div class="panel-head">
            <div>
              <h3>Workspace</h3>
              <p v-if="page.selectedTable">{{ page.selectedSchema }} / {{ page.selectedTable }}</p>
              <p v-else>{{ at('workspaceIdleHint') }}</p>
            </div>
            <div v-if="page.selectedTable || page.isRedis" class="panel-actions">
              <el-button v-if="page.canManageRedisKeys" type="primary" plain @click="page.openRedisKeyCreate">{{ at('addKey') }}</el-button>
              <el-button v-else-if="page.selectedTable" type="primary" plain :disabled="!page.canEditRows" @click="page.openInsertRow">{{ at('addData') }}</el-button>
              <el-button @click="page.refreshSelectedData">{{ at('refreshData') }}</el-button>
            </div>
          </div>

          <el-tabs v-model="page.activeTab">
            <el-tab-pane label="Table Data" name="data">
              <div class="filter-row">
                <el-select v-model="page.tableFilter.key" clearable placeholder="Filter Column" style="width: 180px">
                  <el-option v-for="item in page.filterableColumns" :key="item" :label="item" :value="item" />
                </el-select>
                <el-input v-model="page.tableFilter.text" clearable :placeholder="at('filterValuePlaceholder')" style="width: 280px" />
              </div>
              <el-alert
                v-if="page.supportsResourceData"
                class="resource-readonly-alert"
                type="info"
                :closable="false"
                show-icon
                :title="page.isRedis && page.canManageRedisKeys ? at('redisKeyManageTitle') : at('resourceReadOnlyTitle')"
              >
                <template v-if="page.resourceType === 'collection'">{{ at('mongoFilterHint') }}</template>
                <template v-else-if="page.resourceType === 'key'">
                  <span v-if="page.canManageRedisKeys">{{ at('redisKeyWriteHint') }}</span>
                  <span v-else>{{ at('redisKeyReadOnlyHint') }}</span>
                </template>
                <template v-else>{{ at('pgTableDataHint') }}</template>
              </el-alert>
              <el-table v-if="page.supportsResourceData" v-loading="page.dataLoading" :data="page.selectedRows" border height="380">
                <el-table-column v-for="col in page.selectedColumns" :key="col.name" :label="col.name" min-width="180" show-overflow-tooltip>
                  <template #default="{ row }">{{ page.formatResourceValue(row[col.name]) }}</template>
                </el-table-column>
                <el-table-column v-if="page.isRedis" :label="at('actions')" width="150" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" :disabled="!page.canManageRedisKeys || row.type === 'stream'" @click="page.openRedisKeyEdit(row)">{{ at('edit') }}</el-button>
                    <el-button link type="danger" :disabled="!page.canManageRedisKeys" @click="page.deleteRedisKey(row)">{{ at('delete') }}</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <el-table v-else v-loading="page.dataLoading" :data="page.selectedRows" border height="380">
                <el-table-column v-for="(col, index) in page.selectedColumns" :key="col.name" :label="col.name" min-width="160">
                  <template #default="{ row, $index }">
                    <el-input
                      v-if="page.isEditingCell(row, col.name, $index)"
                      v-model="page.pendingCellValue"
                      size="small"
                      @keyup.enter="page.commitCellEdit(row, col.name)"
                      @blur="page.commitCellEdit(row, col.name)"
                    />
                    <button v-else type="button" class="cell-button" :class="{ disabled: !page.canEditRows }" @click="page.startCellEdit(row, col.name, $index)">
                      {{ row[col.name] ?? '-' }}
                    </button>
                  </template>
                </el-table-column>
                <el-table-column :label="at('actions')" width="150" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" :disabled="!page.canEditRows" @click="page.openEditRow(row)">{{ at('edit') }}</el-button>
                    <el-button link type="danger" :disabled="!page.canEditRows" @click="page.handleDeleteRow(row)">{{ at('delete') }}</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div v-if="page.supportsResourceData && page.resourceIndexes.length" class="resource-indexes">
                <strong>Collection Index</strong>
                <el-tag v-for="(item, index) in page.resourceIndexes" :key="index" effect="plain">
                  {{ page.formatResourceValue(item.name || item.key || item) }}
                </el-tag>
              </div>
              <div class="pager">
                <el-pagination
                  v-model:current-page="page.tableQuery.pageNum"
                  v-model:page-size="page.tableQuery.pageSize"
                  :total="page.selectedTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="page.refreshSelectedData"
                  @size-change="page.refreshSelectedData"
                />
              </div>
            </el-tab-pane>

            <el-tab-pane :label="at('sqlResultsTab')" name="result">
              <div class="filter-row result-filter-row">
                <el-select v-model="page.resultFilter.key" clearable placeholder="Filter Column" style="width: 180px">
                  <el-option v-for="item in page.resultColumns" :key="item" :label="item" :value="item" />
                </el-select>
                <el-input v-model="page.resultFilter.text" clearable placeholder="Result Set Filter" style="width: 280px" />
                <el-button :disabled="!page.filteredResultRows.length" @click="page.exportResultCSV">CSV Export</el-button>
              </div>
              <el-table :data="page.filteredResultRows" border height="380">
                <el-table-column v-for="col in page.resultColumns" :key="col" :prop="col" :label="col" min-width="160" />
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="Execution History" name="history">
              <el-table v-loading="page.historyLoading" :data="page.historyList" border height="380">
                <el-table-column prop="executionId" label="Execution ID" min-width="190" show-overflow-tooltip />
                <el-table-column prop="sqlType" label="Type" width="100" />
                <el-table-column prop="environment" label="Environment" width="90" />
                <el-table-column prop="schemaName" label="Database" width="120" />
                <el-table-column prop="tableName" label="Table" width="140" />
                <el-table-column prop="sqlText" label="SQL" min-width="320" show-overflow-tooltip />
                <el-table-column :label="at('status')" width="90">
                  <template #default="{ row }">
                    <el-tag :type="row.status === 1 ? 'success' : 'danger'" effect="light">
                      {{ row.status === 1 ? at('statusSuccess') : at('statusFailed') }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="rowsAffected" label="Affected Rows" width="110" />
                <el-table-column prop="durationMs" label="Duration (ms)" width="100" />
                <el-table-column prop="operator" label="Operator" width="110" />
                <el-table-column prop="clientIp" label="Client IP" width="140" />
                <el-table-column prop="createTime" label="Executed At" min-width="160" />
                <el-table-column label="Rollback SQL" width="150">
                  <template #default="{ row }">
                    <el-tag v-if="row.rollbackSql" :type="row.rollbackConfidence === 'high' ? 'success' : 'warning'" size="small" effect="plain">
                      {{ row.rollbackConfidence === 'high' ? at('highConfidence') : at('reviewRequired') }}
                    </el-tag>
                    <el-button link type="primary" :disabled="!row.rollbackSql" @click="page.openRollback(row)">{{ at('view') }}</el-button>
                    <el-button link type="primary" :disabled="!row.rollbackSql" @click="page.copyRollback(row)">{{ at('copy') }}</el-button>
                  </template>
                </el-table-column>
                <el-table-column :label="at('actions')" width="80" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="page.reuseHistorySQL(row)">{{ at('reuse') }}</el-button></template></el-table-column>
              </el-table>
              <div class="pager">
                <el-pagination
                  v-model:current-page="page.historyQuery.pageNum"
                  v-model:page-size="page.historyQuery.pageSize"
                  :total="page.historyTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="page.loadHistory"
                  @size-change="page.loadHistory"
                />
              </div>
            </el-tab-pane>

            <el-tab-pane label="Import / Export Task" name="tasks">
              <el-table v-loading="page.taskLoading" :data="page.taskList" border height="380">
                <el-table-column prop="taskType" label="Type" width="90">
                  <template #default="{ row }">{{ row.taskType === 'export' ? 'Export' : 'Import' }}</template>
                </el-table-column>
                <el-table-column label="Source" min-width="220">
                  <template #default="{ row }">
                    <span v-if="row.taskType === 'import'">{{ row.sourceDatabase }} / {{ row.sourceSchema }} / {{ row.sourceTable }}</span>
                    <span v-else>{{ row.databaseName }} / {{ row.schemaName }} / {{ row.tableName }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="Target" min-width="220">
                  <template #default="{ row }">
                    <span v-if="row.taskType === 'import'">{{ row.targetDatabase }} / {{ row.targetSchema }} / {{ row.targetTable }}</span>
                    <span v-else>-</span>
                  </template>
                </el-table-column>
                <el-table-column :label="at('status')" width="110">
                  <template #default="{ row }">
                    <el-tag :type="page.taskStatusType(row.status)" effect="light">{{ page.taskStatusText(row.status) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column :label="at('progress')" width="180">
                  <template #default="{ row }">
                    <el-progress :percentage="Number(row.progress || 0)" :status="row.status === 'failed' ? 'exception' : row.status === 'success' ? 'success' : ''" />
                  </template>
                </el-table-column>
                <el-table-column prop="rowsAffected" :label="at('rows')" width="90" />
                <el-table-column prop="message" :label="at('description')" min-width="180" show-overflow-tooltip />
                <el-table-column prop="createTime" label="Created At" min-width="160" />
                <el-table-column :label="at('actions')" width="120">
                  <template #default="{ row }">
                    <el-button
                      v-if="row.taskType === 'export' && row.status === 'success'"
                      link
                      type="primary"
                      @click="page.downloadTask(row)"
                    >
                      Download
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="pager">
                <el-pagination
                  v-model:current-page="page.taskQuery.pageNum"
                  v-model:page-size="page.taskQuery.pageSize"
                  :total="page.taskTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="page.loadTasks"
                  @size-change="page.loadTasks"
                />
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
</template>

<style scoped>
.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 14px;
}

.panel-head h3 {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
}

.panel-head p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
}

.panel-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.resource-readonly-alert {
  margin-bottom: 12px;
}

.resource-indexes {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.filter-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.cell-button {
  display: block;
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  text-align: left;
  color: inherit;
  cursor: pointer;
  min-height: 22px;
}

.cell-button:hover {
  color: var(--el-color-primary);
}

.cell-button.disabled {
  color: var(--el-text-color-secondary);
  cursor: not-allowed;
}

.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}
</style>
