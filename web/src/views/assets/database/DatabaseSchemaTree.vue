<script setup>
import { at } from '../../../utils/asset-i18n'
import { uiT } from '../../../utils/english-hardcoding-i18n'

// I5-H — template/CSS child extracted from views/assets/DatabaseWorkbench.vue
// (i5-plan §3.8 — CW-WB: database/ 자식). Identifiers are rewired to the
// page bundle passed by the parent (:page 주입 패턴 승계 — §5 #11). All state
// and handlers remain owned by the parent view and its composables.
const props = defineProps({ page: { type: Object, required: true } })
</script>

<template>
              <section class="sidebar-section schema-section">
          <div class="sidebar-section-title">
            <strong>{{ uiT('databaseTableStructure') }}</strong>
            <div class="schema-title-actions">
              <el-button v-if="page.supportsCreateDatabase" type="primary" link @click="page.openCreateDatabase">
                {{ at('createObjectButton', { label: page.createDatabaseObjectLabel }) }}
              </el-button>
              <span>{{ page.connection?.dbName || at('allDatabases') }}</span>
            </div>
          </div>
          <div class="sidebar-top">
            <el-input v-model="page.treeKeyword" clearable :placeholder="at('searchDatabaseOrTable')" />
            <el-button @click="page.loadTree">{{ at('refresh') }}</el-button>
          </div>
          <el-tree
            :ref="(el) => (page.treeRef = el)"
            v-loading="page.treeLoading"
            node-key="id"
            class="schema-tree"
            :data="page.pagedSchemaTree"
            :props="{ label: 'label', children: 'children' }"
            :filter-node-method="page.treeFilterMethod"
            default-expand-all
            :expand-on-click-node="false"
            @node-click="page.onTreeNodeClick"
          >
            <template #default="{ data }">
              <div class="tree-node">
                <span>{{ data.label }}</span>
                <small v-if="data.isTable && Number.isFinite(data.rows)">{{ data.rows }}</small>
                <template v-else-if="data.isSchema">
                  <span class="schema-node-meta" @click.stop>
                    <small>{{ data.visibleTableCount ?? data.tableCount }}</small>
                    <span v-if="data.totalPages > 1" class="schema-pagination">
                      <el-button
                        text
                        size="small"
                        :disabled="data.currentPage <= 1"
                        :title="at('previousPage')"
                        @click.stop="page.changeSchemaTablePage(data, data.currentPage - 1)"
                      >{{ at('previousPage') }}</el-button>
                      <span>{{ data.currentPage }}/{{ data.totalPages }}</span>
                      <el-button
                        text
                        size="small"
                        :disabled="data.currentPage >= data.totalPages"
                        :title="at('nextPage')"
                        @click.stop="page.changeSchemaTablePage(data, data.currentPage + 1)"
                      >{{ at('nextPage') }}</el-button>
                    </span>
                  </span>
                </template>
              </div>
            </template>
          </el-tree>
        </section>
</template>

<style scoped>
.schema-section {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 12px;
}

.sidebar-section-title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.sidebar-section-title strong {
  color: var(--el-text-color-primary);
  font-size: 15px;
}

.sidebar-section-title span {
  overflow: hidden;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-title-actions {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.schema-title-actions .el-button {
  flex: none;
  padding: 0;
}

.sidebar-top {
  display: flex;
  gap: 10px;
}

.schema-tree {
  flex: 1;
  overflow: auto;
}

.tree-node {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.tree-node small {
  color: var(--el-text-color-secondary);
}

.schema-node-meta,
.schema-pagination {
  display: inline-flex;
  align-items: center;
}

.schema-node-meta {
  margin-left: auto;
  gap: 6px;
}

.schema-pagination {
  gap: 2px;
  color: var(--el-text-color-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.schema-pagination .el-button {
  min-width: auto;
  height: 20px;
  padding: 0 2px;
  font-size: 11px;
}
</style>
