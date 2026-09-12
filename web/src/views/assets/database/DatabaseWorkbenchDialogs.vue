<script setup>
import { at } from '../../../utils/asset-i18n'

// I5-H — template/CSS child extracted from views/assets/DatabaseWorkbench.vue
// (i5-plan §3.8 — CW-WB: database/ 자식). Identifiers are rewired to the
// page bundle passed by the parent (:page 주입 패턴 승계 — §5 #11). All state
// and handlers remain owned by the parent view and its composables.
const props = defineProps({ page: { type: Object, required: true } })
</script>

<template>
          <el-dialog v-model="page.createDatabaseVisible" :title="at('createObjectTitle', { label: page.createDatabaseObjectLabel })" width="520px" destroy-on-close>
      <el-alert
        :title="page.isPostgres ? at('postgresCreateAlert') : at('mysqlCreateAlert')"
        type="info"
        :closable="false"
        show-icon
      />
      <el-form label-width="112px" class="create-database-form">
        <el-form-item :label="at('objectNameLabel', { label: page.createDatabaseObjectLabel })" required>
          <el-input v-model="page.createDatabaseForm.name" maxlength="63" show-word-limit :placeholder="at('identifierPlaceholder')" @keyup.enter="page.submitCreateDatabase" />
        </el-form-item>
        <template v-if="!page.isPostgres">
          <el-form-item label="Character Set">
            <el-select v-model="page.createDatabaseForm.charset" style="width: 100%" :loading="page.charsetOptionsLoading" @change="page.onCreateDatabaseCharsetChange">
              <el-option v-for="item in page.mysqlCharsetOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="Collation">
            <el-select v-model="page.createDatabaseForm.collation" style="width: 100%" :loading="page.charsetOptionsLoading" :disabled="page.charsetOptionsLoading">
              <el-option v-for="item in page.availableCollations" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
        </template>
        <p class="form-help">{{ at('identifierRuleHint') }}</p>
      </el-form>
      <template #footer>
        <el-button @click="page.createDatabaseVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" :loading="page.creatingDatabase" @click="page.submitCreateDatabase">{{ at('confirmCreate') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="page.sqlConfirmVisible" :title="at('sqlWriteConfirmTitle')" width="780px">
      <div v-if="page.sqlAnalysis" class="sql-risk-panel">
        <div class="sql-risk-summary">
          <el-tag :type="page.riskTagType(page.sqlAnalysis.riskLevel)" effect="dark">
            {{ page.sqlAnalysis.riskLevel === 'high' ? 'High Risk' : page.sqlAnalysis.riskLevel === 'medium' ? 'Medium Risk' : 'Low Risk' }}
          </el-tag>
          <strong>{{ page.sqlAnalysis.databaseName }} / {{ page.sqlAnalysis.schema }}</strong>
          <span>{{ page.sqlAnalysis.environment || at('environmentUnassigned') }}</span>
        </div>
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item label="SQL Type">{{ page.sqlAnalysis.sqlType }}</el-descriptions-item>
          <el-descriptions-item :label="at('statementCount')">{{ page.sqlAnalysis.statementCount }}</el-descriptions-item>
          <el-descriptions-item label="Access Mode">{{ page.sqlAnalysis.accessMode === 'readonly' ? 'Read-only' : 'Read / Write' }}</el-descriptions-item>
        </el-descriptions>
        <ul class="risk-reasons">
          <li v-for="item in page.sqlAnalysis.reasons" :key="item">{{ item }}</li>
        </ul>
        <pre class="confirm-sql">{{ page.pendingSQL }}</pre>
        <el-alert
          v-if="page.sqlAnalysis.accessMode === 'readonly'"
          :title="at('readOnlyRejectAlert')"
          type="error"
          :closable="false"
          show-icon
        />
        <el-alert v-else :title="at('writeConfirmAlert')" type="warning" :closable="false" show-icon />
        <el-form-item class="sql-acknowledgement" :label="at('enterConfirmationPhrase', { phrase: page.sqlConfirmationText() })">
          <el-input v-model="page.sqlAcknowledgement" :placeholder="page.sqlConfirmationText()" autocomplete="off" />
        </el-form-item>
      </div>
      <template #footer>
        <el-button @click="page.sqlConfirmVisible = false">{{ at('cancel') }}</el-button>
        <el-button
          type="danger"
          :loading="page.sqlRunning"
          :disabled="page.sqlAnalysis?.accessMode === 'readonly' || page.sqlAcknowledgement !== page.sqlConfirmationText()"
          @click="page.confirmSQLExecution"
        >
          {{ at('execConfirmPhrase') }}
        </el-button>
      </template>
    </el-dialog>
          <el-dialog v-model="page.rowDialogVisible" :title="page.rowDialogMode === 'insert' ? at('addRowTitle') : at('editRowTitle')" width="720px">
      <el-form label-width="140px">
        <el-form-item v-for="col in page.selectedColumns" :key="col.name" :label="`${col.name} (${col.columnType})`">
          <el-input v-model="page.rowForm[col.name]" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="page.rowDialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" @click="page.submitRow">{{ at('save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="page.rollbackDialogVisible" title="Rollback SQL" width="860px">
      <el-alert
        :title="page.rollbackConfidence === 'high' ? at('highConfidenceAlert') : at('limitedConfidenceAlert')"
        :type="page.rollbackConfidence === 'high' ? 'success' : 'warning'"
        :closable="false"
        show-icon
        class="rollback-alert"
      />
      <pre class="rollback-box">{{ page.rollbackSQL || at('noRollbackSQL') }}</pre>
    </el-dialog>

    <el-dialog v-model="page.importDialogVisible" :title="at('createImportTaskTitle')" width="680px">
      <el-form label-width="120px">
        <el-form-item label="Source Database">
          <el-select v-model="page.importForm.sourceDatabaseId" filterable style="width: 100%">
              <el-option
                v-for="item in page.importDatabaseOptions"
                :key="item.id"
                :label="`${item.name} (${String(item.dbType || 'mysql').toUpperCase()} · ${item.host}:${item.port})`"
                :value="item.id"
              />
          </el-select>
        </el-form-item>
        <el-form-item label="Source Database">
          <el-select v-model="page.importForm.sourceSchema" filterable style="width: 100%">
            <el-option v-for="item in page.sourceSchemas" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Source Table">
          <el-select v-model="page.importForm.sourceTable" filterable style="width: 100%">
            <el-option v-for="item in page.sourceTables" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Target Table">
          <div>{{ page.selectedSchema || '-' }} / {{ page.selectedTable || '-' }}</div>
        </el-form-item>
        <el-form-item :label="at('autoCreateTable')">
          <el-switch v-model="page.importForm.createIfMissing" />
        </el-form-item>
        <el-form-item :label="at('truncateTargetTable')">
          <el-switch v-model="page.importForm.truncateTarget" />
        </el-form-item>
        <el-form-item label="Precheck">
          <el-button :loading="page.importPrechecking" @click="page.runImportPrecheck">{{ at('runPrecheck') }}</el-button>
        </el-form-item>
        <div v-if="page.importPrecheck" class="import-precheck" :class="{ danger: !page.importPrecheck.ready }">
          <div class="precheck-head">
            <strong>{{ page.importPrecheck.ready ? at('precheckPassed') : at('precheckFailed') }}</strong>
            <el-tag :type="page.importPrecheck.ready ? 'success' : 'danger'">{{ at('estimatedRowsTag', { count: page.importPrecheck.estimatedRows }) }}</el-tag>
          </div>
          <p>{{ at('columnMappingSummary', { mapped: page.importPrecheck.commonColumns?.length || 0, missing: page.importPrecheck.missingColumns?.length || 0 }) }}</p>
          <p v-if="page.importPrecheck.missingColumns?.length">{{ at('missingColumns', { columns: page.importPrecheck.missingColumns.join(', ') }) }}</p>
          <ul v-if="page.importPrecheck.warnings?.length">
            <li v-for="item in page.importPrecheck.warnings" :key="item">{{ item }}</li>
          </ul>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="page.importDialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" :disabled="!page.importPrecheck?.ready" @click="page.submitImportTask">{{ at('startImport') }}</el-button>
      </template>
    </el-dialog>
</template>

<style scoped>
.create-database-form {
  margin-top: 18px;
}

.form-help {
  margin: -8px 0 0 112px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.sql-risk-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sql-risk-summary,
.precheck-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sql-risk-summary span {
  color: var(--el-text-color-secondary);
}

.risk-reasons {
  margin: 0;
  padding-left: 20px;
  color: #a16207;
}

.confirm-sql {
  max-height: 260px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-family: Consolas, 'Courier New', monospace;
  line-height: 1.6;
  white-space: pre-wrap;
}

.rollback-box {
  margin: 0;
  min-height: 240px;
  max-height: 520px;
  overflow: auto;
  padding: 16px;
  border-radius: 12px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 13px;
  line-height: 1.7;
  font-family: Consolas, 'Courier New', monospace;
  white-space: pre-wrap;
}

.rollback-alert {
  margin-bottom: 12px;
}

.sql-acknowledgement {
  margin: 16px 0 0;
}
</style>
