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
        <h3>{{ page.t('k8sWorkloads') }}</h3>
        <p>{{ page.t('k8sWorkloadsHint') }}</p>
      </div>
      <div class="kuboard-head-actions">
        <el-button plain @click="page.refreshCurrentClusterData">{{ page.t('k8sRefresh') }}</el-button>
      </div>
    </div>

    <div class="kuboard-type-tabs">
      <button
        v-for="item in page.workloadTypeOptions"
        :key="item.value"
        type="button"
        class="kuboard-type-tab"
        :class="{ active: page.workloadTypeFilter === item.value }"
        @click="page.handleWorkloadTypeChange(item.value)"
      >
        <span>{{ item.label }}</span>
        <em>{{ item.count }}</em>
      </button>
    </div>

    <div class="kuboard-stat-row">
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sWorkloads') }}</span>
        <strong>{{ page.workloadSummary.total }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sReady') }}</span>
        <strong>{{ page.workloadSummary.healthy }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sNamespace') }}</span>
        <strong>{{ page.workloadSummary.namespaces }}</strong>
      </article>
      <article class="kuboard-stat-card">
        <span>{{ page.t('k8sRestart') }}</span>
        <strong>{{ page.workloadSummary.restartable }}</strong>
      </article>
    </div>

    <div class="kuboard-table-card">
      <el-table v-if="page.hasItems(page.kuboardWorkloadRows)" :data="page.kuboardWorkloadRows" class="kuboard-table">
        <el-table-column :label="page.t('k8sName')" min-width="220">
          <template #default="{ row }">
            <div class="kuboard-name-cell">
              <button type="button" class="kuboard-link-button" @click="page.openWorkloadPods(row)">
                {{ row.name }}
              </button>
              <small>{{ row.namespace }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="ready" :label="page.t('k8sReady')" width="110" />
        <el-table-column prop="updated" :label="page.t('k8sUpdated')" width="110" />
        <el-table-column prop="available" :label="page.t('k8sAvailable')" width="110" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="120" />
        <el-table-column :label="page.t('k8sImages')" min-width="320">
          <template #default="{ row }">
            <pre class="kuboard-images">{{ row.images || page.t('k8sNoImageData') }}</pre>
          </template>
        </el-table-column>
        <el-table-column :label="page.t('k8sActions')" min-width="280" fixed="right">
          <template #default="{ row }">
            <div class="kuboard-actions">
              <el-button link type="primary" @click="page.openWorkloadPods(row)">{{ page.t('k8sPods') }}</el-button>
              <el-button link type="primary" @click="page.openWorkloadDetail(row)">{{ page.t('k8sDetail') }}</el-button>
              <el-button link type="primary" @click="page.openWorkloadYAML(row)">{{ page.t('k8sYaml') }}</el-button>
              <el-button v-if="page.supportsScale(row)" link type="primary" @click="page.openScaleDialog(row)">{{ page.t('k8sScale') }}</el-button>
              <el-button v-if="page.supportsRestart(row)" link type="warning" @click="page.handleRestartWorkload(row)">{{ page.t('k8sRestart') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeWorkloadData')" />
    </div>
  </section>
</template>
