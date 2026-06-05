<script setup>
import { Connection, Grid, Histogram, Monitor, Promotion, SetUp } from '@element-plus/icons-vue'

defineProps({
  page: {
    type: Object,
    required: true
  }
})

const iconMap = {
  overview: Histogram,
  nodes: Monitor,
  namespaces: Grid,
  workloads: SetUp,
  pods: Promotion,
  services: Connection,
  ingresses: Connection,
  'advanced-network': Connection,
  'config-storage': Grid
}
</script>

<template>
  <div class="kuboard-shell">
    <section class="kuboard-main">
      <header class="kuboard-main-head">
        <div class="kuboard-head-main">
          <div class="kuboard-head-title">
            <h2>{{ page.currentSection.title }}</h2>
            <p>{{ page.currentSection.description }}</p>
          </div>

          <div class="kuboard-head-tools">
            <el-select
              :model-value="page.cluster?.id"
              :placeholder="page.t('k8sSelectCluster')"
              filterable
              class="kuboard-cluster-select"
              :disabled="!page.clusterOptions.length"
              :loading="page.switching"
              @update:model-value="page.handleClusterChange"
            >
              <el-option v-for="item in page.clusterOptions" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>

            <div v-if="page.cluster" class="kuboard-head-chips">
              <span class="kuboard-chip">
                <em>{{ page.t('k8sApiServer') }}</em>
                <b>{{ page.cluster.apiServer }}</b>
              </span>
              <span class="kuboard-chip">
                <em>{{ page.t('k8sVersion') }}</em>
                <b>{{ page.cluster.version }}</b>
              </span>
              <span class="kuboard-chip">
                <em>{{ page.t('k8sNodeCount') }}</em>
                <b>{{ page.t('k8sNodeCountTotal', { count: page.cluster.nodeCount }) }}</b>
              </span>
            </div>
          </div>
        </div>

        <div v-if="page.hasCluster && page.shouldShowNamespaceFilter(page.currentTab)" class="kuboard-global-toolbar">
          <el-select
            :model-value="page.namespaceFilter"
            filterable
            class="namespace-select"
            :placeholder="page.t('k8sAllNamespaces')"
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
          <div class="inline-scope-card">
            <span>{{ page.t('k8sCurrentScope') }}</span>
            <strong>
              {{ page.namespaceFilter === '__all__' ? page.t('k8sAllNamespaces') : page.namespaceFilter }}
            </strong>
          </div>
        </div>
      </header>

      <main class="kuboard-workspace">
        <slot />
      </main>
    </section>
  </div>
</template>
