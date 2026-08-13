<script setup>
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <section class="cluster-header">
    <div class="cluster-toolbar">
      <div class="cluster-selector-block">
        <span class="field-label">{{ page.t('k8sClusterSelector') }}</span>
        <el-select
          :model-value="page.cluster?.id"
          :placeholder="page.t('k8sSelectCluster')"
          size="large"
          filterable
          class="cluster-select"
          :disabled="!page.clusterOptions.length"
          :loading="page.switching"
          @update:model-value="page.handleClusterChange"
        >
          <el-option v-for="item in page.clusterOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </div>

      <div v-if="page.cluster" class="cluster-brief">
        <div class="cluster-identity">
          <strong>{{ page.cluster.name }}</strong>
          <el-tag :type="page.statusType" effect="light">{{ page.clusterStatusText(page.cluster.status) }}</el-tag>
        </div>
        <div class="cluster-meta">
          <span class="meta-chip">
            <em>{{ page.t('k8sApiServer') }}</em>
            <b>{{ page.cluster.apiServer }}</b>
          </span>
          <span class="meta-chip">
            <em>{{ page.t('k8sVersion') }}</em>
            <b>{{ page.cluster.version }}</b>
          </span>
          <span class="meta-chip">
            <em>{{ page.t('k8sNodeCount') }}</em>
            <b>{{ page.t('k8sNodeCountTotal', { count: page.cluster.nodeCount }) }}</b>
          </span>
        </div>
      </div>
    </div>

    <div v-if="!page.cluster" class="empty-cluster-state">
      <h3>{{ page.t('k8sNoClusterTitle') }}</h3>
      <p>{{ page.t('k8sNoClusterDesc') }}</p>
    </div>
  </section>

  <section class="section-tabs">
    <el-tabs :model-value="page.currentTab" @tab-change="page.handleTabChange">
      <el-tab-pane v-for="tab in page.sectionTabs" :key="tab.key" :name="tab.key" :label="page.t(tab.labelKey)" />
    </el-tabs>
  </section>

  <section v-if="page.hasCluster && page.shouldShowNamespaceFilter(page.currentTab)" class="filter-toolbar">
    <el-select
      :model-value="page.namespaceFilter"
      filterable
      :placeholder="page.t('k8sAllNamespaces')"
      class="namespace-select"
      @update:model-value="page.handleNamespaceFilterChange"
    >
      <el-option
        v-for="item in page.namespaceOptions"
        :key="item.value"
        :label="item.label"
        :value="item.value"
      />
    </el-select>
    <el-input
      :model-value="page.resourceKeyword"
      clearable
      :placeholder="page.t('k8sSearchResources')"
      class="resource-search"
      @update:model-value="page.handleResourceKeywordChange"
    />
  </section>
</template>
