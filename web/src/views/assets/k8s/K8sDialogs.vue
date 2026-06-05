<script setup>
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <el-dialog v-model="page.yamlDialogVisible" :title="page.yamlEditor.title" width="1180px" class="yaml-editor-dialog">
    <div class="yaml-workspace">
      <section class="yaml-pane editor">
        <div class="yaml-pane-head">
          <strong>{{ page.t('k8sYamlEditor') }}</strong>
          <span>{{ page.t('k8sYamlTotalLines', { count: page.yamlEditor.yaml.split('\n').length }) }}</span>
        </div>
        <div class="yaml-search-bar">
          <el-input
            v-model="page.yamlSearch.keyword"
            :placeholder="page.t('k8sYamlSearch')"
            clearable
            @input="page.runYAMLSearch(false)"
            @clear="page.runYAMLSearch(false)"
          />
          <span class="yaml-search-summary">
            {{ page.yamlSearch.matches.length ? `${page.yamlSearch.activeIndex + 1}/${page.yamlSearch.matches.length}` : '0/0' }}
          </span>
          <el-button :disabled="!page.yamlSearch.matches.length" @click="page.searchYAMLPrev">{{ page.t('k8sPrevious') }}</el-button>
          <el-button :disabled="!page.yamlSearch.matches.length" @click="page.searchYAMLNext">{{ page.t('k8sNext') }}</el-button>
        </div>
        <div class="yaml-editor-shell">
          <div class="yaml-line-gutter">
            <div class="yaml-line-gutter-inner" :style="{ transform: `translateY(-${page.yamlEditorScrollTop}px)` }">
              <div
                v-for="line in page.yamlLineNumbers"
                :key="line"
                :class="['yaml-line-number', { active: line === page.yamlCurrentLine }]"
              >
                {{ line }}
              </div>
            </div>
          </div>
          <div class="yaml-editor-stage">
            <div class="yaml-current-line" :style="{ top: page.yamlCurrentLineOffset }"></div>
            <textarea
              :ref="(el) => (page.yamlTextareaRef = el)"
              v-model="page.yamlEditor.yaml"
              class="yaml-native-textarea"
              spellcheck="false"
              :placeholder="page.t('k8sEditYamlHere')"
              @input="page.handleYAMLInput"
              @click="page.updateYAMLCurrentLine"
              @keyup="page.updateYAMLCurrentLine"
              @mouseup="page.updateYAMLCurrentLine"
              @scroll="page.handleYAMLScroll"
            ></textarea>
          </div>
        </div>
      </section>
      <section class="yaml-pane preview">
        <div class="yaml-diff-head">
          <strong>{{ page.t('k8sDiffPreview') }}</strong>
          <span>+{{ page.yamlChangeSummary.added }} / -{{ page.yamlChangeSummary.removed }}</span>
        </div>
        <div class="yaml-preview-toolbar">
          <span class="yaml-preview-hint">
            {{ page.yamlChangeSummary.changed ? page.t('k8sChangedLinesHint') : page.t('k8sNoChangesYet') }}
          </span>
        </div>
        <div class="yaml-preview-shell">
          <div class="yaml-line-gutter preview">
            <div class="yaml-line-gutter-inner">
              <div
                v-for="line in page.yamlPreviewLineNumbers"
                :key="`preview-${line}`"
                class="yaml-line-number"
              >
                {{ line }}
              </div>
            </div>
          </div>
          <div class="yaml-diff-panel">
            <div
              v-for="(item, index) in page.yamlDiffLines"
              :key="`${index}-${item.type}`"
              :class="['yaml-diff-line', item.type]"
            >
              <span class="marker">
                {{ item.type === 'added' ? '+' : item.type === 'removed' ? '-' : ' ' }}
              </span>
              <code>{{ item.text || ' ' }}</code>
            </div>
          </div>
        </div>
      </section>
    </div>
    <template #footer>
      <el-button @click="page.yamlDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.yamlSaving" @click="page.submitYAMLUpdate">{{ page.t('save') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.scaleDialogVisible" :title="page.t('k8sScaleWorkload')" width="420px">
    <el-form label-width="90px">
      <el-form-item :label="page.t('k8sNamespace')">
        <el-input :model-value="page.scaleForm.namespace" readonly />
      </el-form-item>
      <el-form-item :label="page.t('k8sWorkload')">
        <el-input :model-value="`${page.scaleForm.workloadType} / ${page.scaleForm.workloadName}`" readonly />
      </el-form-item>
      <el-form-item :label="page.t('k8sReplicas')">
        <el-input-number v-model="page.scaleForm.replicas" :min="0" :max="999" controls-position="right" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="page.scaleDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.scaleLoading" @click="page.submitScale">{{ page.t('save') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog
    v-model="page.istioCreateDialogVisible"
    :title="page.t('k8sCreateIstioResourceTitle', { resource: page.yamlResourceLabel(page.istioCreateForm.resourceType) })"
    width="980px"
  >
    <div class="istio-create-dialog">
      <p class="dialog-tip">{{ page.t('k8sIstioCreateHint') }}</p>
      <el-input
        v-model="page.istioCreateForm.yaml"
        type="textarea"
        :rows="22"
        resize="none"
        class="istio-create-textarea"
      />
    </div>
    <template #footer>
      <el-button @click="page.istioCreateDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.istioCreateSaving" @click="page.submitIstioCreate">
        {{ page.t('save') }}
      </el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.trafficDialogVisible" :title="page.t('k8sAdjustTrafficTitle')" width="640px">
    <div class="traffic-dialog">
      <div class="traffic-dialog-head">
        <div>
          <strong>{{ page.trafficForm.name }}</strong>
          <span>{{ page.trafficForm.namespace }}</span>
        </div>
        <el-tag :type="page.trafficTotalWeight === 100 ? 'success' : 'warning'" effect="light">
          {{ page.t('k8sTrafficTotal', { total: page.trafficTotalWeight }) }}
        </el-tag>
      </div>
      <p class="dialog-tip">{{ page.t('k8sTrafficDialogHint') }}</p>
      <div class="traffic-route-list">
        <div v-for="item in page.trafficForm.routes" :key="item.index" class="traffic-route-item">
          <div class="traffic-route-meta">
            <strong>{{ item.label }}</strong>
            <span>{{ item.host }}</span>
          </div>
          <el-input-number v-model="item.weight" :min="0" :max="100" controls-position="right" />
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="page.trafficDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.trafficSaving" @click="page.submitTrafficAdjust">
        {{ page.t('save') }}
      </el-button>
    </template>
  </el-dialog>
</template>
