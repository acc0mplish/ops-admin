<script setup>
import { kt } from '../../../utils/k8s-extra-i18n'

defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <el-dialog
    v-model="page.serviceEditVisible"
    :title="kt('serviceEditTitle', { name: page.serviceEditForm.name || '-' })"
    width="980px"
    class="service-edit-dialog"
    destroy-on-close
  >
    <div v-loading="page.serviceEditLoading" class="service-edit-content">
      <div class="service-edit-summary">
        <div><span>{{ kt('serviceNameLabel') }}</span><strong>{{ page.serviceEditForm.name || '-' }}</strong></div>
        <div><span>Namespace</span><strong>{{ page.serviceEditForm.namespace || '-' }}</strong></div>
        <p>{{ kt('serviceEditDesc') }}</p>
      </div>

      <el-form label-position="top" class="service-edit-form">
        <section class="service-edit-section service-metadata-section">
          <div class="service-edit-section-head"><strong>Metadata</strong><span>{{ kt('metadataHint') }}</span></div>
          <div class="service-metadata-grid">
            <div class="service-metadata-block">
              <div class="service-metadata-block-head"><strong>Label</strong><el-button link type="primary" @click="page.addServiceMetadataEntry('labels')">{{ kt('addEntry') }}</el-button></div>
              <div v-if="!page.serviceEditForm.labels.length" class="service-edit-empty">{{ kt('noLabels') }}</div>
              <div v-else class="service-metadata-list">
                <div v-for="(item, index) in page.serviceEditForm.labels" :key="index" class="service-metadata-row">
                  <el-input v-model.trim="item.key" :placeholder="kt('labelKeyPlaceholder')" aria-label="Label Key" />
                  <el-input v-model.trim="item.value" :placeholder="kt('labelValuePlaceholder')" :aria-label="kt('labelValuePlaceholder')" />
                  <el-button link type="danger" :aria-label="kt('deleteLabelAria')" @click="page.removeServiceMetadataEntry('labels', index)">{{ kt('delete') }}</el-button>
                </div>
              </div>
            </div>
            <div class="service-metadata-block">
              <div class="service-metadata-block-head"><strong>Annotation</strong><el-button link type="primary" @click="page.addServiceMetadataEntry('annotations')">{{ kt('addEntry') }}</el-button></div>
              <div v-if="!page.serviceEditForm.annotations.length" class="service-edit-empty">{{ kt('noAnnotations') }}</div>
              <div v-else class="service-metadata-list">
                <div v-for="(item, index) in page.serviceEditForm.annotations" :key="index" class="service-metadata-row annotation-metadata-row">
                  <el-input v-model.trim="item.key" :placeholder="kt('annotationKeyPlaceholder')" aria-label="Annotation Key" />
                  <el-input v-model="item.value" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" :placeholder="kt('annotationValuePlaceholder')" :aria-label="kt('annotationValuePlaceholder')" />
                  <el-button link type="danger" :aria-label="kt('deleteAnnotationAria')" @click="page.removeServiceMetadataEntry('annotations', index)">{{ kt('delete') }}</el-button>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="service-edit-section">
          <div class="service-edit-section-head"><strong>Service Type</strong><span>{{ kt('serviceTypeHint') }}</span></div>
          <el-radio-group v-model="page.serviceEditForm.type" class="service-type-radio-group">
            <el-radio-button value="ClusterIP">ClusterIP</el-radio-button>
            <el-radio-button value="Headless">Headless</el-radio-button>
            <el-radio-button value="NodePort">NodePort</el-radio-button>
            <el-radio-button value="LoadBalancer">LoadBalancer</el-radio-button>
            <el-radio-button value="ExternalName">ExternalName</el-radio-button>
          </el-radio-group>
          <div v-if="page.serviceEditForm.type === 'Headless'" class="service-headless-option"><div><strong>Headless Service</strong><span>{{ kt('headlessHint') }}</span></div></div>
          <el-form-item v-if="page.serviceEditForm.type === 'ExternalName'" :label="kt('externalNameLabel')" required>
            <el-input v-model.trim="page.serviceEditForm.externalName" :placeholder="kt('externalNamePlaceholder')" />
            <div class="service-edit-hint">{{ kt('externalNameHint') }}</div>
          </el-form-item>
        </section>

        <section v-if="page.serviceEditForm.type !== 'ExternalName'" class="service-edit-section">
          <div class="service-edit-section-head with-action"><div><strong>Selector</strong><span>{{ kt('selectorHint') }}</span></div><el-button link type="primary" @click="page.addServiceSelector">{{ kt('addSelector') }}</el-button></div>
          <div v-if="!page.serviceEditForm.selectors.length" class="service-edit-empty">{{ kt('noSelectors') }}</div>
          <div v-else class="service-selector-list">
            <div v-for="(item, index) in page.serviceEditForm.selectors" :key="index" class="service-selector-row">
              <el-input v-model.trim="item.key" :placeholder="kt('selectorKeyPlaceholder')" :aria-label="kt('selectorKeyAria')" />
              <el-input v-model.trim="item.value" :placeholder="kt('selectorValuePlaceholder')" :aria-label="kt('selectorValueAria')" />
              <el-button link type="danger" :aria-label="kt('deleteSelectorAria')" @click="page.removeServiceSelector(index)">{{ kt('delete') }}</el-button>
            </div>
          </div>
        </section>

        <section v-if="page.serviceEditForm.type !== 'ExternalName'" class="service-edit-section">
          <div class="service-edit-section-head with-action"><div><strong>Port Mapping</strong><span>{{ kt('portMappingHint') }}</span></div><el-button link type="primary" @click="page.addServicePort">{{ kt('addPort') }}</el-button></div>
          <div class="service-port-table-head" :class="{ 'has-node-port': page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer' }"><span>{{ kt('nameLabel') }}</span><span>Protocol</span><span>Service Port</span><span>Target Port</span><span v-if="page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer'">NodePort</span><span></span></div>
          <div v-for="(port, index) in page.serviceEditForm.ports" :key="index" class="service-port-row" :class="{ 'has-node-port': page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer' }">
            <el-input v-model.trim="port.name" :placeholder="kt('portNamePlaceholder')" :aria-label="kt('portNameAria')" />
            <el-select v-model="port.protocol" aria-label="Port Protocol"><el-option label="TCP" value="TCP" /><el-option label="UDP" value="UDP" /><el-option label="SCTP" value="SCTP" /></el-select>
            <el-input-number v-model="port.port" :min="1" :max="65535" controls-position="right" aria-label="Service Port" />
            <el-input v-model.trim="port.targetPort" :placeholder="kt('targetPortPlaceholder')" :aria-label="kt('targetPortAria')" />
            <el-input-number v-if="page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer'" v-model="port.nodePort" :min="1" :max="65535" controls-position="right" :placeholder="kt('autoAssignPlaceholder')" aria-label="NodePort" />
            <el-button link type="danger" :disabled="page.serviceEditForm.ports.length === 1" :aria-label="kt('deletePortAria')" @click="page.removeServicePort(index)">{{ kt('delete') }}</el-button>
          </div>
        </section>
      </el-form>
    </div>
    <template #footer>
      <el-button @click="page.serviceEditVisible = false">{{ kt('cancel') }}</el-button>
      <el-button type="primary" :loading="page.serviceEditSaving" @click="page.submitServiceEdit">{{ kt('saveService') }}</el-button>
    </template>
  </el-dialog>

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

  <el-dialog v-model="page.namespaceCreateVisible" :title="page.t('k8sCreateNamespace')" width="480px" destroy-on-close>
    <el-form label-position="top">
      <el-form-item :label="page.t('k8sNamespaceName')" required>
        <el-input v-model="page.namespaceCreateForm.name" maxlength="63" show-word-limit :placeholder="kt('namespacePlaceholder')" />
      </el-form-item>
      <div class="dialog-tip">{{ page.t('k8sCreateNamespaceHint') }}</div>
    </el-form>
    <template #footer>
      <el-button @click="page.namespaceCreateVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.namespaceCreateSaving" @click="page.submitNamespaceCreate">{{ page.t('k8sCreate') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog
    v-model="page.configStorageCreateVisible"
    :title="page.configStorageCreateTitle()"
    width="920px"
    class="config-storage-create-dialog"
    destroy-on-close
  >
    <el-form label-width="96px" class="config-storage-create-form">
      <template v-if="page.configStorageCreateForm.kind === 'pvc'">
        <div class="config-storage-section-head pvc-storage-section-head">
          <strong>{{ kt('storageSectionTitle') }}</strong>
          <span>{{ kt('storageSectionHint') }}</span>
        </div>
        <el-form-item label="StorageClass" required>
          <el-select v-model="page.configStorageCreateForm.storageClass" filterable :placeholder="kt('selectStorageClass')">
            <el-option
              v-for="item in page.pvcStorageClassOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
          <div class="config-storage-field-hint">{{ kt('storageClassHint') }}</div>
        </el-form-item>
      </template>

      <el-form-item :label="page.t('k8sNamespace')" required>
        <el-select
          v-model="page.configStorageCreateForm.namespace"
          filterable
          :placeholder="kt('selectNamespace')"
          :disabled="page.configStorageEditing || (page.configStorageCreateForm.kind === 'pvc' && page.pvcNamespaceLocked)"
        >
          <el-option
            v-for="item in page.configStorageCreateForm.kind === 'pvc' ? page.pvcNamespaceOptions : page.namespaceOptions.filter((option) => option.value !== '__all__')"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </el-select>
        <div class="config-storage-field-hint">
          {{ page.configStorageCreateForm.kind === 'pvc' && page.pvcNamespaceLocked
            ? kt('pvcNamespaceLockedHint')
            : kt('namespaceScopeHint') }}
        </div>
      </el-form-item>

      <el-form-item :label="page.t('k8sName')" required>
        <div class="config-storage-name-wrap">
          <el-input v-model="page.configStorageCreateForm.name" maxlength="63" :disabled="page.configStorageEditing" :placeholder="kt('kindNamePlaceholder', { kind: page.configStorageCreateForm.kind === 'secret' ? 'Secret' : page.configStorageCreateForm.kind === 'pvc' ? 'Storage' : 'ConfigMap' })"" />
          <div class="config-storage-field-hint">{{ kt('nameRuleHint') }}</div>
        </div>
      </el-form-item>

      <template v-if="page.configStorageCreateForm.kind === 'pvc'">
        <div class="config-storage-section-head"><strong>{{ kt('requestSpecTitle') }}</strong><span>{{ kt('requestSpecHint') }}</span></div>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item :label="kt('storageCapacityLabel')" required><el-input v-model="page.configStorageCreateForm.capacity" :placeholder="kt('capacity1GiPlaceholder')" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Access Mode"><el-input :model-value="page.pvcAccessModeLabel" readonly /></el-form-item></el-col>
        </el-row>
      </template>

      <template v-else>
        <el-form-item v-if="page.configStorageCreateForm.kind === 'secret'" label="Secret Type">
          <el-radio-group v-model="page.configStorageCreateForm.secretType" class="config-storage-radio-group">
            <el-radio-button label="Opaque">Opaque</el-radio-button>
            <el-radio-button label="kubernetes.io/tls">TLS Certificate</el-radio-button>
            <el-radio-button label="kubernetes.io/dockerconfigjson">Docker Config</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <div class="config-storage-section-head">
          <strong>{{ kt('configEntriesTitle') }}</strong>
          <span>{{ page.configStorageCreateForm.kind === 'secret' ? kt('secretEntriesHint') : kt('configMapEntriesHint') }}</span>
        </div>
        <el-form-item :label="kt('contentLabel')" required>
          <div class="config-storage-entry-list">
            <div class="config-storage-entry-table-head"><span>{{ kt('varName') }}</span><span>{{ kt('varValue') }}</span><span></span></div>
            <div v-for="(entry, index) in page.configStorageCreateForm.entries" :key="index" class="config-storage-entry-row">
              <el-input v-model="entry.key" :placeholder="kt('varKeyPlaceholder')" />
              <el-input v-model="entry.value" type="textarea" :autosize="{ minRows: 6 }" :placeholder="kt('varValuePlaceholder')" />
              <el-button link type="danger" :disabled="page.configStorageCreateForm.entries.length === 1" @click="page.removeConfigStorageEntry(index)">{{ kt('delete') }}</el-button>
            </div>
            <el-button link type="primary" @click="page.addConfigStorageEntry">{{ kt('addManually') }}</el-button>
          </div>
        </el-form-item>
      </template>
    </el-form>
    <template #footer>
      <el-button @click="page.configStorageCreateVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.configStorageCreateSaving" @click="page.submitConfigStorageCreate">
        {{ page.configStorageCreateTitle() }}
      </el-button>
    </template>
  </el-dialog>

  <el-dialog
    v-model="page.storageClassCreateVisible"
    :title="kt('newStorageClass')"
    width="840px"
    class="storage-class-create-dialog"
    destroy-on-close
  >
    <div class="storage-class-create-tip">
      {{ kt('storageClassCreateTip') }}
    </div>
    <el-form label-position="top" class="storage-class-create-form">
      <section class="storage-class-form-section">
        <div class="storage-class-section-heading">
          <strong>{{ kt('basicConfigTitle') }}</strong>
          <span>{{ kt('basicConfigHint') }}</span>
        </div>
        <el-form-item :label="kt('storageClassNameLabel')" required>
          <el-input v-model.trim="page.storageClassCreateForm.name" maxlength="63" :placeholder="kt('storageClassNamePlaceholder')" />
        </el-form-item>

        <div class="storage-scope-panel">
          <div class="storage-scope-panel-head">
            <el-checkbox v-model="page.storageClassCreateForm.scopeNamespaceEnabled">
              {{ kt('namespaceRestricted') }}
            </el-checkbox>
            <span>{{ page.storageClassCreateForm.scopeNamespaceEnabled ? kt('scopeRestrictedHint') : kt('scopeUnrestrictedHint') }}</span>
          </div>
          <el-select
            v-if="page.storageClassCreateForm.scopeNamespaceEnabled"
            v-model="page.storageClassCreateForm.scopeNamespace"
            class="storage-scope-namespace-select"
            filterable
            :placeholder="kt('selectNamespace')"
          >
            <el-option
              v-for="option in page.namespaceOptions.filter((item) => item.value !== '__all__')"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
        </div>

        <el-form-item :label="kt('sourceTypeLabel')" required>
          <el-radio-group v-model="page.storageClassCreateForm.sourceType" class="storage-source-type-group">
          <el-radio-button label="hostpath">{{ kt('hostPathSource') }}</el-radio-button>
          <el-radio-button label="nfs">{{ kt('nfsSource') }}</el-radio-button>
        </el-radio-group>
        <div class="config-storage-field-hint">
          {{ page.storageClassCreateForm.sourceType === 'hostpath'
            ? kt('hostPathHint')
            : kt('nfsHint') }}
        </div>
        </el-form-item>

        <el-row :gutter="16" class="storage-class-capacity-row">
        <el-col :span="12">
          <el-form-item :label="kt('capacityLabel')" required>
            <el-input v-model.trim="page.storageClassCreateForm.capacity" :placeholder="kt('capacity10GiPlaceholder')" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="Reclaim Policy" required>
            <el-radio-group v-model="page.storageClassCreateForm.reclaimPolicy" class="storage-reclaim-policy-group">
              <el-radio-button label="Delete">{{ kt('reclaimDelete') }}</el-radio-button>
              <el-radio-button label="Retain">{{ kt('reclaimRetain') }}</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </el-col>
        </el-row>
      </section>

      <section class="storage-class-form-section storage-source-config-section">
        <div class="storage-class-section-heading">
          <strong>{{ kt('mountSectionTitle') }}</strong>
          <span>{{ kt('mountSectionHint') }}</span>
        </div>
        <div class="storage-source-config-grid">
          <el-form-item v-if="page.storageClassCreateForm.sourceType === 'nfs'" :label="kt('nfsServerLabel')" required>
            <el-input v-model.trim="page.storageClassCreateForm.nfsServer" :placeholder="kt('nfsServerPlaceholder')" />
          </el-form-item>

          <el-form-item :label="kt('pathLabel')" required>
            <el-input
              v-model.trim="page.storageClassCreateForm.path"
              :placeholder="page.storageClassCreateForm.sourceType === 'hostpath' ? kt('hostPathExample') : kt('nfsPathExample')"
            />
          </el-form-item>

          <el-form-item :label="kt('accessPolicyLabel')" required>
            <el-select v-model="page.storageClassCreateForm.accessMode">
              <el-option
                v-for="option in page.storageAccessModeOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
          </el-form-item>
        </div>
      </section>
    </el-form>
    <template #footer>
      <el-button @click="page.storageClassCreateVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.storageClassCreateSaving" @click="page.submitStorageClassCreate">{{ kt('newStorageClass') }}</el-button>
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

  <el-dialog v-model="page.batchScaleDialogVisible" :title="kt('batchScaleTitle')" width="460px" destroy-on-close>
    <div class="batch-workload-dialog-tip">{{ kt('batchScaleTip', { count: page.workloadSelectionCount }) }}</div>
    <el-form label-position="top">
      <el-form-item :label="kt('targetReplicaLabel')" required>
        <el-input-number v-model="page.batchScaleForm.replicas" :min="0" :max="999" controls-position="right" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="page.batchScaleDialogVisible = false">{{ kt('cancel') }}</el-button>
      <el-button type="primary" :loading="page.batchScaleSaving" @click="page.submitBatchScale">{{ kt('continue') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.workloadResourceDialogVisible" :title="kt('podSettingsUpdateTitle')" width="920px" class="workload-resource-dialog-wrap" destroy-on-close>
    <div class="workload-resource-dialog">
      <div class="workload-resource-summary">
        <div><span>Namespace</span><strong>{{ page.workloadResourceForm.namespace }}</strong></div>
        <div><span>Workload</span><strong>{{ page.workloadResourceForm.workloadName }} · {{ page.workloadResourceForm.workloadType }}</strong></div>
      </div>
      <p class="dialog-tip">{{ kt('workloadResourceTip') }}</p>
      <section v-for="container in page.workloadResourceForm.containers" :key="container.name" class="container-resource-card">
        <div class="container-resource-head">
          <strong>{{ container.name }}</strong>
          <span>{{ kt('containerConfigLabel') }}</span>
        </div>
        <el-row :gutter="14" class="container-basic-row">
          <el-col :span="15"><el-form-item label="Image"><el-input :model-value="container.image || '-'" readonly /></el-form-item></el-col>
          <el-col :span="9"><el-form-item label="Image Pull Policy" class="image-pull-policy-field">
            <el-select v-model="container.imagePullPolicy" class="image-pull-policy-select">
              <el-option :label="kt('pullAlways')" value="Always" />
              <el-option :label="kt('pullIfNotPresent')" value="IfNotPresent" />
              <el-option :label="kt('pullNever')" value="Never" />
            </el-select>
          </el-form-item></el-col>
        </el-row>
        <div class="resource-setting-title">{{ kt('requestLimitTitle') }}</div>
        <el-row :gutter="14">
          <el-col :span="6"><el-form-item label="CPU Request"><el-input v-model="container.requestCPU" placeholder="100m" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="CPU Limit"><el-input v-model="container.limitCPU" placeholder="1" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="Memory Request"><el-input v-model="container.requestMemory" placeholder="256Mi" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="Memory Limit"><el-input v-model="container.limitMemory" placeholder="1Gi" /></el-form-item></el-col>
        </el-row>
        <div class="resource-setting-title env-setting-title">Environment Variable</div>
        <div v-if="container.env?.length" class="workload-env-head"><span>{{ kt('varName') }}</span><span>{{ kt('varValue') }}</span><span>Type</span><span>{{ kt('actions') }}</span></div>
        <div v-if="container.env?.length" class="workload-env-list">
          <div v-for="(env, envIndex) in container.env" :key="`${container.name}-${envIndex}`" class="workload-env-row">
            <el-input v-model="env.name" :placeholder="kt('envNamePlaceholder')" />
            <el-input :model-value="env.valueFrom ? (env.source || kt('k8sRefVar')) : env.value" :readonly="Boolean(env.valueFrom)" :placeholder="kt('envValuePlaceholder')" @update:model-value="env.value = $event" />
            <el-tag v-if="env.valueFrom" type="info" effect="plain">{{ kt('refVarTag') }}</el-tag>
            <span v-else class="workload-env-type">{{ kt('plainVarTag') }}</span>
            <el-button link type="danger" @click="page.removeWorkloadEnvironment(container, envIndex)">{{ kt('delete') }}</el-button>
          </div>
        </div>
        <el-button link type="primary" class="add-env-button" @click="page.addWorkloadEnvironment(container)">{{ kt('addEnvVar') }}</el-button>
      </section>
    </div>
    <template #footer>
      <el-button @click="page.workloadResourceDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.workloadResourceSaving" @click="page.submitWorkloadResourceSettings">{{ kt('saveSettings') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.imageVersionDialogVisible" :title="page.t('k8sBatchUpdateImageVersion')" width="520px">
    <el-form label-width="110px">
      <el-form-item :label="page.t('k8sSelectedWorkloads')">
        <span>{{ page.workloadSelectionCount }}</span>
      </el-form-item>
      <el-form-item :label="page.t('k8sTargetImageVersion')">
        <el-input v-model="page.imageVersionForm.version" :placeholder="page.t('k8sTargetImageVersionPlaceholder')" />
      </el-form-item>
      <el-form-item>
        <div class="dialog-tip">
          {{ page.t('k8sBatchImageVersionHint') }}
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="page.imageVersionDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.imageVersionSaving" @click="page.submitWorkloadImageVersionUpdate">
        {{ page.t('save') }}
      </el-button>
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
