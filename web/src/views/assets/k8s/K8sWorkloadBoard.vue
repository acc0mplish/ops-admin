<script setup>
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <section class="kuboard-section workload-workspace">
    <div class="workload-control-panel">
      <div class="workload-control-row">
        <div class="workload-filter-group">
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
            <strong>{{ page.namespaceFilter === '__all__' ? page.t('k8sAllNamespaces') : page.namespaceFilter }}</strong>
          </div>
        </div>
        <div class="kuboard-head-actions">
          <el-button plain :disabled="!page.workloadSelectionCount" @click="page.openImageVersionDialog">
            {{ page.t('k8sBatchUpdateImageVersion') }}
            <span v-if="page.workloadSelectionCount">({{ page.workloadSelectionCount }})</span>
          </el-button>
          <el-button plain @click="page.refreshCurrentClusterData">{{ page.t('k8sRefresh') }}</el-button>
        </div>
      </div>
      <div class="workload-type-row">
        <span>工作负载类型</span>
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
      </div>
    </div>

    <div class="kuboard-table-card">
      <el-table
        v-if="page.hasItems(page.kuboardWorkloadRows)"
        :data="page.kuboardWorkloadRows"
        class="kuboard-table"
        @selection-change="page.handleWorkloadSelectionChange"
      >
        <el-table-column type="selection" width="48" />
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
        <el-table-column prop="requests" label="Request" min-width="155" />
        <el-table-column prop="limits" label="Limit" min-width="155" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="120" />
        <el-table-column :label="page.t('k8sImages')" min-width="320">
          <template #default="{ row }">
            <pre class="kuboard-images">{{ row.images || page.t('k8sNoImageData') }}</pre>
          </template>
        </el-table-column>
        <el-table-column :label="page.t('k8sActions')" min-width="410" fixed="right">
          <template #default="{ row }">
            <div class="kuboard-actions workload-actions">
              <el-button link type="primary" @click="page.openWorkloadResourceSettings(row)">更新 Pod 设置</el-button>
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
