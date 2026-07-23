<script setup>
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <section class="kuboard-section">
    <div class="kuboard-resource-head">
      <div>
        <h3>{{ page.t('k8sNamespaces') }}</h3>
        <p>{{ page.t('k8sNamespacesHint') }}</p>
      </div>
      <div class="kuboard-head-actions">
        <el-button plain @click="page.refreshCurrentClusterData">{{ page.t('k8sRefresh') }}</el-button>
        <el-button type="primary" @click="page.openNamespaceCreate">{{ page.t('k8sCreateNamespace') }}</el-button>
      </div>
    </div>

    <div class="kuboard-stat-row">
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sNamespacesTotal') }}</span>
        <strong>{{ page.namespaceSummary.total }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sNamespaceStatusActive') }}</span>
        <strong>{{ page.namespaceSummary.active }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sPodsCount') }}</span>
        <strong>{{ page.namespaceSummary.pods }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sServicesCount') }}</span>
        <strong>{{ page.namespaceSummary.services }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sWorkloadsCount') }}</span>
        <strong>{{ page.namespaceSummary.workloads }}</strong>
      </article>
    </div>

    <div class="kuboard-list-toolbar">
      <el-input
        :model-value="page.namespaceKeyword"
        clearable
        :placeholder="page.t('k8sSearchResources')"
        class="resource-search"
        @update:model-value="page.handleNamespaceKeywordChange"
      />
    </div>

    <div class="kuboard-table-card">
      <el-table v-if="page.hasItems(page.filteredKuboardNamespaceRows)" :data="page.filteredKuboardNamespaceRows" class="kuboard-table">
        <el-table-column :label="page.t('k8sName')" min-width="220">
          <template #default="{ row }">
            <div class="kuboard-name-cell">
              <button type="button" class="kuboard-link-button" @click="page.openNamespaceWorkloads(row)">
                {{ row.name }}
              </button>
              <small>{{ page.t('k8sOpenNamespace') }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="page.t('k8sStatus')" width="140">
          <template #default="{ row }">
            <el-tag size="small" :type="String(row.phase).toLowerCase() === 'active' ? 'success' : 'info'" effect="light">
              {{ row.phase }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="podsCount" :label="page.t('k8sPodsCount')" width="110" />
        <el-table-column prop="servicesCount" :label="page.t('k8sServicesCount')" width="110" />
        <el-table-column prop="workloadsCount" :label="page.t('k8sWorkloadsCount')" width="120" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="120" />
        <el-table-column :label="page.t('k8sActions')" width="210" fixed="right">
          <template #default="{ row }">
            <div class="kuboard-actions">
              <el-button link type="primary" @click="page.openNamespaceWorkloads(row)">{{ page.t('k8sWorkloads') }}</el-button>
              <el-button link type="primary" @click="page.openNamespaceDetail(row)">{{ page.t('k8sDetail') }}</el-button>
              <el-button link type="primary" @click="page.openNamespaceYAML(row)">{{ page.t('k8sYaml') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeNamespaceData')" />
    </div>
  </section>
</template>
