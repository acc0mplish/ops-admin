<script setup>
import { at } from '../../../utils/asset-i18n'

// I5-H — template/CSS child extracted from views/assets/DatabaseWorkbench.vue
// (i5-plan §3.8 — CW-WB: database/ 자식). Identifiers are rewired to the
// page bundle passed by the parent (:page 주입 패턴 승계 — §5 #11). All state
// and handlers remain owned by the parent view and its composables.
const props = defineProps({ page: { type: Object, required: true } })
</script>

<template>
              <div class="dbms-editor page-card">
          <div class="panel-head">
            <div>
              <h3>{{ page.supportsSQL ? 'SQL Editor' : page.isRedis ? 'Redis Command Console' : `${page.connection?.dbType?.toUpperCase() || 'Database'} Resource Explorer` }}</h3>
              <p v-if="page.supportsSQL">{{ at('sqlEditorDescription') }}</p>
              <p v-else-if="page.isRedis">{{ at('redisEditorDescription') }}</p>
              <p v-else>{{ at('genericEditorDescription') }}</p>
            </div>
            <div class="panel-actions">
              <el-button v-if="page.supportsSQL" :loading="page.sqlRunning" type="primary" @click="page.runSQL">{{ at('runSQL') }}</el-button>
              <el-button v-if="page.supportsSQL" @click="page.formatSQL">Format</el-button>
              <el-button v-if="page.supportsSQL" @click="page.saveCurrentSQL">SQL Favorite</el-button>
              <el-dropdown v-if="page.supportsSQL && page.sqlFavorites.length" trigger="click">
                <el-button>Favorite {{ page.sqlFavorites.length }}</el-button>
                <template #dropdown>
                  <el-dropdown-menu class="sql-favorite-menu">
                    <el-dropdown-item v-for="item in page.sqlFavorites" :key="item.id" class="sql-favorite-item">
                      <span @click="page.applySqlFavorite(item)">{{ item.name }}</span>
                      <el-button link type="danger" size="small" @click.stop="page.removeSqlFavorite(item)">{{ at('delete') }}</el-button>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <el-button v-if="!page.supportsSQL && page.isRedis" :loading="page.redisRunning" type="primary" @click="page.runRedisCommand">{{ at('runRedisCommand') }}</el-button>
              <el-button :disabled="!page.supportsExport || !page.selectedTable" @click="page.createExportTask">Export Task</el-button>
              <el-button :disabled="!page.supportsImport || page.isReadOnly" @click="page.openImportDialog">Import Task</el-button>
            </div>
          </div>

          <div v-if="page.supportsSQL" class="snippet-row">
            <el-button v-for="item in page.sqlSnippets" :key="item.label" size="small" plain @click="page.insertSnippet(item.text)">
              {{ item.label }}
            </el-button>
              <span class="snippet-hint">{{ at('snippetHint') }}</span>
          </div>

          <div v-else-if="page.isRedis" class="redis-command-console">
            <div class="redis-command-head">
              <strong>Redis Command Console</strong>
              <span>{{ at('redisConsoleHint') }}</span>
            </div>
            <div class="redis-command-snippets">
              <el-button v-for="item in page.redisCommandSnippets" :key="item.label" size="small" plain @click="page.redisCommandText = item.text">
                {{ item.label }}
              </el-button>
            </div>
            <el-input
              v-model="page.redisCommandText"
              class="redis-command-input"
              type="textarea"
              :rows="6"
              spellcheck="false"
              :placeholder="at('redisCommandPlaceholder')"
              @keydown.ctrl.enter.prevent="page.runRedisCommand"
              @keydown.meta.enter.prevent="page.runRedisCommand"
            />
            <div class="redis-command-hint">{{ at('redisCommandHint') }}</div>
          </div>

          <el-alert v-if="!page.supportsSQL && !page.isRedis" class="database-capability-alert" type="info" :closable="false" show-icon :title="at('nonSqlDatabaseAlert')">
            {{ at('nonSqlDatabaseAlertDesc') }}
          </el-alert>

          <div v-if="page.supportsSQL" :ref="(el) => (page.editorWrapRef = el)" class="editor-shell">
            <div class="line-gutter" :style="{ transform: `translateY(-${page.sqlScrollTop}px)` }">
              <div v-for="line in page.sqlLines" :key="line" class="line-number" :class="{ active: line === page.currentLine }">
                {{ line }}
              </div>
            </div>
            <div class="editor-layer">
              <pre class="sql-highlight" :style="{ transform: `translate(${-page.sqlScrollLeft}px, ${-page.sqlScrollTop}px)` }" v-html="page.highlightedSQL + '\n'"></pre>
              <textarea
                :ref="(el) => (page.sqlEditorRef = el)"
                v-model="page.sqlText"
                class="sql-editor"
                spellcheck="false"
                :placeholder="at('sqlEditorPlaceholder')"
                @input="page.updateAutocomplete"
                @click="page.updateAutocomplete"
                @keyup="page.updateAutocomplete"
                @keydown="page.onEditorKeydown"
                @scroll="page.syncEditorMetrics"
                @blur="setTimeout(() => (page.showSuggestions = false), 150)"
              />
            </div>
            <div v-if="page.showSuggestions && page.suggestions.length" class="autocomplete-panel">
              <button
                v-for="(item, index) in page.suggestions"
                :key="item"
                type="button"
                class="autocomplete-item"
                :class="{ active: index === page.activeSuggestionIndex }"
                @mousedown.prevent="page.applySuggestion(item)"
              >
                {{ item }}
              </button>
            </div>
          </div>

          <div v-if="page.execMeta.sqlType" class="exec-meta">
            <span>Type: {{ page.execMeta.sqlType }}</span>
            <span>Affected Rows: {{ page.execMeta.rowsAffected }}</span>
            <span>Duration: {{ page.execMeta.durationMs }} ms</span>
          </div>
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

.snippet-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 12px;
}

.database-capability-alert {
  margin-top: 16px;
}

.redis-command-console {
  margin-top: 16px;
  padding: 16px;
  border: 1px solid #253653;
  border-radius: 10px;
  background: #0f172a;
}

.redis-command-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
  margin-bottom: 12px;
  color: #e2e8f0;
}

.redis-command-head span,
.redis-command-hint {
  color: #94a3b8;
  font-size: 12px;
}

.redis-command-snippets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.redis-command-input :deep(.el-textarea__inner) {
  min-height: 132px;
  border-color: #334155;
  background: #111827;
  color: #e2e8f0;
  font-family: Consolas, 'Courier New', monospace;
  line-height: 1.65;
}

.redis-command-input :deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #3b82f6 inset;
}

.redis-command-hint {
  margin-top: 10px;
}

.snippet-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: auto;
}

.editor-shell {
  position: relative;
  display: grid;
  grid-template-columns: 56px minmax(0, 1fr);
  min-height: 260px;
  max-height: 380px;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  border-radius: 14px;
  background: #0f172a;
}

.line-gutter {
  padding-top: 14px;
  background: rgba(15, 23, 42, 0.92);
  border-right: 1px solid rgba(148, 163, 184, 0.16);
}

.line-number {
  height: 22px;
  padding: 0 12px 0 0;
  text-align: right;
  color: #64748b;
  font-size: 13px;
  line-height: 22px;
  font-family: Consolas, 'Courier New', monospace;
}

.line-number.active {
  color: #f8fafc;
}

.editor-layer {
  position: relative;
  overflow: hidden;
}

.sql-highlight,
.sql-editor {
  margin: 0;
  padding: 14px 16px;
  font-size: 14px;
  line-height: 22px;
  font-family: Consolas, 'Courier New', monospace;
  white-space: pre;
}

.sql-highlight {
  position: absolute;
  inset: 0;
  overflow: hidden;
  color: #e2e8f0;
  pointer-events: none;
}

:deep(.token-keyword) {
  color: #60a5fa;
  font-weight: 600;
}

:deep(.token-string) {
  color: #fbbf24;
}

:deep(.token-number) {
  color: #34d399;
}

:deep(.token-comment) {
  color: #64748b;
}

.sql-editor {
  position: relative;
  z-index: 1;
  width: 100%;
  min-height: 260px;
  height: 100%;
  border: none;
  resize: none;
  outline: none;
  background: transparent;
  color: transparent;
  caret-color: #f8fafc;
  overflow: auto;
}

.autocomplete-panel {
  position: absolute;
  left: 76px;
  top: calc(100% - 6px);
  width: 280px;
  max-height: 240px;
  overflow: auto;
  background: #111827;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 12px;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.38);
  z-index: 30;
}

.autocomplete-item {
  width: 100%;
  display: block;
  border: none;
  background: transparent;
  text-align: left;
  padding: 10px 12px;
  color: #e5e7eb;
  cursor: pointer;
}

.autocomplete-item:hover,
.autocomplete-item.active {
  background: rgba(59, 130, 246, 0.18);
}

.exec-meta {
  margin-top: 12px;
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  color: var(--el-text-color-secondary);
}
</style>
