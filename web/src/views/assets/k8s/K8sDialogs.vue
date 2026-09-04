<script setup>
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
    :title="`Service 수정 · ${page.serviceEditForm.name || '-'}`"
    width="980px"
    class="service-edit-dialog"
    destroy-on-close
  >
    <div v-loading="page.serviceEditLoading" class="service-edit-content">
      <div class="service-edit-summary">
        <div><span>Service 이름</span><strong>{{ page.serviceEditForm.name || '-' }}</strong></div>
        <div><span>Namespace</span><strong>{{ page.serviceEditForm.namespace || '-' }}</strong></div>
        <p>구조화된 필드로 Service 정의를 관리합니다. 저장 시 Service 라우팅 Rule만 업데이트하며 Workload는 변경하지 않습니다.</p>
      </div>

      <el-form label-position="top" class="service-edit-form">
        <section class="service-edit-section service-metadata-section">
          <div class="service-edit-section-head"><strong>Metadata</strong><span>Label은 필터링과 연관 지정에 사용되며 Annotation은 Controller 또는 플랫폼 확장 설정을 담습니다.</span></div>
          <div class="service-metadata-grid">
            <div class="service-metadata-block">
              <div class="service-metadata-block-head"><strong>Label</strong><el-button link type="primary" @click="page.addServiceMetadataEntry('labels')">+ 추가</el-button></div>
              <div v-if="!page.serviceEditForm.labels.length" class="service-edit-empty">Label이 없습니다.</div>
              <div v-else class="service-metadata-list">
                <div v-for="(item, index) in page.serviceEditForm.labels" :key="index" class="service-metadata-row">
                  <el-input v-model.trim="item.key" placeholder="예: app.kubernetes.io/name" aria-label="Label Key" />
                  <el-input v-model.trim="item.value" placeholder="Label 값" aria-label="Label 값" />
                  <el-button link type="danger" aria-label="Label 삭제" @click="page.removeServiceMetadataEntry('labels', index)">삭제</el-button>
                </div>
              </div>
            </div>
            <div class="service-metadata-block">
              <div class="service-metadata-block-head"><strong>Annotation</strong><el-button link type="primary" @click="page.addServiceMetadataEntry('annotations')">+ 추가</el-button></div>
              <div v-if="!page.serviceEditForm.annotations.length" class="service-edit-empty">Annotation이 없습니다.</div>
              <div v-else class="service-metadata-list">
                <div v-for="(item, index) in page.serviceEditForm.annotations" :key="index" class="service-metadata-row annotation-metadata-row">
                  <el-input v-model.trim="item.key" placeholder="예: service.beta.kubernetes.io/..." aria-label="Annotation Key" />
                  <el-input v-model="item.value" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="Annotation 값" aria-label="Annotation 값" />
                  <el-button link type="danger" aria-label="Annotation 삭제" @click="page.removeServiceMetadataEntry('annotations', index)">삭제</el-button>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="service-edit-section">
          <div class="service-edit-section-head"><strong>Service Type</strong><span>Cluster 내부, Cluster 외부 또는 외부 DNS에서의 접근 방식을 결정합니다.</span></div>
          <el-radio-group v-model="page.serviceEditForm.type" class="service-type-radio-group">
            <el-radio-button value="ClusterIP">ClusterIP</el-radio-button>
            <el-radio-button value="Headless">Headless</el-radio-button>
            <el-radio-button value="NodePort">NodePort</el-radio-button>
            <el-radio-button value="LoadBalancer">LoadBalancer</el-radio-button>
            <el-radio-button value="ExternalName">ExternalName</el-radio-button>
          </el-radio-group>
          <div v-if="page.serviceEditForm.type === 'Headless'" class="service-headless-option"><div><strong>Headless Service</strong><span>Pod DNS 레코드를 반환하며 Cluster IP를 할당하지 않습니다.</span></div></div>
          <el-form-item v-if="page.serviceEditForm.type === 'ExternalName'" label="외부 DNS 이름" required>
            <el-input v-model.trim="page.serviceEditForm.externalName" placeholder="예: mysql.example.com" />
            <div class="service-edit-hint">ExternalName은 Proxy Endpoint를 생성하지 않으며 DNS가 이 주소로 직접 Alias합니다.</div>
          </el-form-item>
        </section>

        <section v-if="page.serviceEditForm.type !== 'ExternalName'" class="service-edit-section">
          <div class="service-edit-section-head with-action"><div><strong>Selector</strong><span>Label이 일치하는 Pod가 해당 Service의 Backend Endpoint가 됩니다.</span></div><el-button link type="primary" @click="page.addServiceSelector">+ Selector 추가</el-button></div>
          <div v-if="!page.serviceEditForm.selectors.length" class="service-edit-empty">Selector가 구성되지 않았습니다. 저장 후 해당 Service는 Pod와 자동 연결되지 않습니다.</div>
          <div v-else class="service-selector-list">
            <div v-for="(item, index) in page.serviceEditForm.selectors" :key="index" class="service-selector-row">
              <el-input v-model.trim="item.key" placeholder="Label Key(예: app)" aria-label="Selector Label Key" />
              <el-input v-model.trim="item.value" placeholder="Label 값(예: api)" aria-label="Selector Label 값" />
              <el-button link type="danger" aria-label="Selector 삭제" @click="page.removeServiceSelector(index)">삭제</el-button>
            </div>
          </div>
        </section>

        <section v-if="page.serviceEditForm.type !== 'ExternalName'" class="service-edit-section">
          <div class="service-edit-section-head with-action"><div><strong>Port Mapping</strong><span>Service Port는 접근 EntryPoint를 노출하며 Target Port는 Container Port 또는 Named Port를 가리킵니다.</span></div><el-button link type="primary" @click="page.addServicePort">+ Port 추가</el-button></div>
          <div class="service-port-table-head" :class="{ 'has-node-port': page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer' }"><span>이름</span><span>Protocol</span><span>Service Port</span><span>Target Port</span><span v-if="page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer'">NodePort</span><span></span></div>
          <div v-for="(port, index) in page.serviceEditForm.ports" :key="index" class="service-port-row" :class="{ 'has-node-port': page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer' }">
            <el-input v-model.trim="port.name" placeholder="예: http" aria-label="Port 이름" />
            <el-select v-model="port.protocol" aria-label="Port Protocol"><el-option label="TCP" value="TCP" /><el-option label="UDP" value="UDP" /><el-option label="SCTP" value="SCTP" /></el-select>
            <el-input-number v-model="port.port" :min="1" :max="65535" controls-position="right" aria-label="Service Port" />
            <el-input v-model.trim="port.targetPort" placeholder="예: 8080 또는 http" aria-label="Target Port" />
            <el-input-number v-if="page.serviceEditForm.type === 'NodePort' || page.serviceEditForm.type === 'LoadBalancer'" v-model="port.nodePort" :min="1" :max="65535" controls-position="right" placeholder="자동 할당" aria-label="NodePort" />
            <el-button link type="danger" :disabled="page.serviceEditForm.ports.length === 1" aria-label="Port 삭제" @click="page.removeServicePort(index)">삭제</el-button>
          </div>
        </section>
      </el-form>
    </div>
    <template #footer>
      <el-button @click="page.serviceEditVisible = false">취소</el-button>
      <el-button type="primary" :loading="page.serviceEditSaving" @click="page.submitServiceEdit">Service 저장</el-button>
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
        <el-input v-model="page.namespaceCreateForm.name" maxlength="63" show-word-limit placeholder="예: game-prod" />
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
          <strong>Storage 구성</strong>
          <span>먼저 StorageClass를 선택하면 사용 가능한 Namespace와 Access Mode가 자동으로 제한됩니다.</span>
        </div>
        <el-form-item label="StorageClass" required>
          <el-select v-model="page.configStorageCreateForm.storageClass" filterable placeholder="StorageClass를 선택하십시오">
            <el-option
              v-for="item in page.pvcStorageClassOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
          <div class="config-storage-field-hint">플랫폼에서 생성한 StorageClass만 표시됩니다. Namespace가 제한된 StorageClass는 해당 Namespace로 자동 고정됩니다.</div>
        </el-form-item>
      </template>

      <el-form-item :label="page.t('k8sNamespace')" required>
        <el-select
          v-model="page.configStorageCreateForm.namespace"
          filterable
          placeholder="Namespace를 선택하십시오"
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
            ? '해당 StorageClass는 Namespace가 제한되어 있어 현재 Namespace는 StorageClass가 자동 결정합니다.'
            : 'ConfigMap, Secret 및 PVC는 Namespace 단위 Resource이며 이 Namespace에만 생성됩니다.' }}
        </div>
      </el-form-item>

      <el-form-item :label="page.t('k8sName')" required>
        <div class="config-storage-name-wrap">
          <el-input v-model="page.configStorageCreateForm.name" maxlength="63" :disabled="page.configStorageEditing" :placeholder="`${page.configStorageCreateForm.kind === 'secret' ? 'Secret' : page.configStorageCreateForm.kind === 'pvc' ? 'Storage' : 'ConfigMap'} 이름을 입력하십시오`" />
          <div class="config-storage-field-hint">최대 63자이며 소문자, 숫자 및 구분자(-)만 포함할 수 있고 소문자 또는 숫자로 시작하고 끝나야 합니다.</div>
        </div>
      </el-form-item>

      <template v-if="page.configStorageCreateForm.kind === 'pvc'">
        <div class="config-storage-section-head"><strong>요청 Spec</strong><span>용량은 Storage가 선언하며 Access Mode는 선택한 StorageClass를 그대로 상속하므로 수동으로 변경할 수 없습니다.</span></div>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="Storage 용량" required><el-input v-model="page.configStorageCreateForm.capacity" placeholder="예: 1Gi" /></el-form-item></el-col>
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
          <strong>Config 항목</strong>
          <span>{{ page.configStorageCreateForm.kind === 'secret' ? 'Secret은 안전한 stringData 방식으로 기록되며 목록에는 평문이 표시되지 않습니다.' : 'ConfigMap에 Application 구성을 추가합니다. 여러 줄 내용을 지원합니다.' }}</span>
        </div>
        <el-form-item label="내용" required>
          <div class="config-storage-entry-list">
            <div class="config-storage-entry-table-head"><span>변수 이름</span><span>변수 값</span><span></span></div>
            <div v-for="(entry, index) in page.configStorageCreateForm.entries" :key="index" class="config-storage-entry-row">
              <el-input v-model="entry.key" placeholder="예: APP_MODE" />
              <el-input v-model="entry.value" type="textarea" :autosize="{ minRows: 6 }" placeholder="변수 값을 입력하십시오. 여러 줄을 지원합니다" />
              <el-button link type="danger" :disabled="page.configStorageCreateForm.entries.length === 1" @click="page.removeConfigStorageEntry(index)">삭제</el-button>
            </div>
            <el-button link type="primary" @click="page.addConfigStorageEntry">+ 직접 추가</el-button>
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
    title="새 StorageClass"
    width="840px"
    class="storage-class-create-dialog"
    destroy-on-close
  >
    <div class="storage-class-create-tip">
      정적 Storage Resource를 생성합니다. 생성 후 "Storage"에서 동일한 이름의 StorageClass를 입력해 사용을 선언할 수 있습니다.
    </div>
    <el-form label-position="top" class="storage-class-create-form">
      <section class="storage-class-form-section">
        <div class="storage-class-section-heading">
          <strong>기본 구성</strong>
          <span>정적 Volume 이름, 용량 및 데이터 소스를 정의합니다.</span>
        </div>
        <el-form-item label="StorageClass 이름" required>
          <el-input v-model.trim="page.storageClassCreateForm.name" maxlength="63" placeholder="예: game-nfs" />
        </el-form-item>

        <div class="storage-scope-panel">
          <div class="storage-scope-panel-head">
            <el-checkbox v-model="page.storageClassCreateForm.scopeNamespaceEnabled">
              Namespace 제한
            </el-checkbox>
            <span>{{ page.storageClassCreateForm.scopeNamespaceEnabled ? '지정한 Namespace에서만 Storage를 생성할 수 있습니다' : '제한 없음, Scope는 Cluster 전체' }}</span>
          </div>
          <el-select
            v-if="page.storageClassCreateForm.scopeNamespaceEnabled"
            v-model="page.storageClassCreateForm.scopeNamespace"
            class="storage-scope-namespace-select"
            filterable
            placeholder="Namespace를 선택하십시오"
          >
            <el-option
              v-for="option in page.namespaceOptions.filter((item) => item.value !== '__all__')"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
        </div>

        <el-form-item label="Storage 소스 유형" required>
          <el-radio-group v-model="page.storageClassCreateForm.sourceType" class="storage-source-type-group">
          <el-radio-button label="hostpath">hostPath · Node 로컬 경로</el-radio-button>
          <el-radio-button label="nfs">NFS · 원격 공유 Storage</el-radio-button>
        </el-radio-group>
        <div class="config-storage-field-hint">
          {{ page.storageClassCreateForm.sourceType === 'hostpath'
            ? 'Node의 파일 시스템 경로를 직접 사용하며 단일 Node 테스트 환경에 적합합니다. 디렉터리는 Pod가 해당 Storage를 처음 마운트할 때 Node가 생성합니다.'
            : 'NFS Protocol로 원격 Storage를 마운트하며 다중 Node 공유 접근을 지원합니다.' }}
        </div>
        </el-form-item>

        <el-row :gutter="16" class="storage-class-capacity-row">
        <el-col :span="12">
          <el-form-item label="용량" required>
            <el-input v-model.trim="page.storageClassCreateForm.capacity" placeholder="예: 10Gi" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="Reclaim Policy" required>
            <el-radio-group v-model="page.storageClassCreateForm.reclaimPolicy" class="storage-reclaim-policy-group">
              <el-radio-button label="Delete">삭제(Delete)</el-radio-button>
              <el-radio-button label="Retain">유지(Retain)</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </el-col>
        </el-row>
      </section>

      <section class="storage-class-form-section storage-source-config-section">
        <div class="storage-class-section-heading">
          <strong>마운트 구성</strong>
          <span>Node 또는 NFS의 실제 저장 위치와 Access Mode를 설정합니다.</span>
        </div>
        <div class="storage-source-config-grid">
          <el-form-item v-if="page.storageClassCreateForm.sourceType === 'nfs'" label="NFS Server 주소" required>
            <el-input v-model.trim="page.storageClassCreateForm.nfsServer" placeholder="예: 10.0.0.10" />
          </el-form-item>

          <el-form-item label="경로" required>
            <el-input
              v-model.trim="page.storageClassCreateForm.path"
              :placeholder="page.storageClassCreateForm.sourceType === 'hostpath' ? '예: /data/k8s' : '예: /exports/k8s'"
            />
          </el-form-item>

          <el-form-item label="Node 접근 정책" required>
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
      <el-button type="primary" :loading="page.storageClassCreateSaving" @click="page.submitStorageClassCreate">새 StorageClass</el-button>
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

  <el-dialog v-model="page.batchScaleDialogVisible" title="Workload 일괄 Scale" width="460px" destroy-on-close>
    <div class="batch-workload-dialog-tip">현재 선택한 {{ page.workloadSelectionCount }}개 Workload에 Replica 수를 일괄 설정합니다. Scale을 지원하지 않는 Resource는 자동으로 건너뜁니다.</div>
    <el-form label-position="top">
      <el-form-item label="Target Replica 수" required>
        <el-input-number v-model="page.batchScaleForm.replicas" :min="0" :max="999" controls-position="right" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="page.batchScaleDialogVisible = false">취소</el-button>
      <el-button type="primary" :loading="page.batchScaleSaving" @click="page.submitBatchScale">계속</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.workloadResourceDialogVisible" title="Pod 설정 업데이트" width="920px" class="workload-resource-dialog-wrap" destroy-on-close>
    <div class="workload-resource-dialog">
      <div class="workload-resource-summary">
        <div><span>Namespace</span><strong>{{ page.workloadResourceForm.namespace }}</strong></div>
        <div><span>Workload</span><strong>{{ page.workloadResourceForm.workloadName }} · {{ page.workloadResourceForm.workloadType }}</strong></div>
      </div>
      <p class="dialog-tip">각 Container의 Resource Request / Limit, Image Pull Policy 및 Environment Variable을 관리합니다. 저장 시 Pod Template을 업데이트하고 Workload Rolling Update를 트리거합니다.</p>
      <section v-for="container in page.workloadResourceForm.containers" :key="container.name" class="container-resource-card">
        <div class="container-resource-head">
          <strong>{{ container.name }}</strong>
          <span>Container 구성</span>
        </div>
        <el-row :gutter="14" class="container-basic-row">
          <el-col :span="15"><el-form-item label="Image"><el-input :model-value="container.image || '-'" readonly /></el-form-item></el-col>
          <el-col :span="9"><el-form-item label="Image Pull Policy" class="image-pull-policy-field">
            <el-select v-model="container.imagePullPolicy" class="image-pull-policy-select">
              <el-option label="Always(항상 Pull)" value="Always" />
              <el-option label="IfNotPresent(로컬 우선)" value="IfNotPresent" />
              <el-option label="Never(로컬 Image만)" value="Never" />
            </el-select>
          </el-form-item></el-col>
        </el-row>
        <div class="resource-setting-title">CPU / Memory Request와 Limit</div>
        <el-row :gutter="14">
          <el-col :span="6"><el-form-item label="CPU Request"><el-input v-model="container.requestCPU" placeholder="100m" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="CPU Limit"><el-input v-model="container.limitCPU" placeholder="1" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="Memory Request"><el-input v-model="container.requestMemory" placeholder="256Mi" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="Memory Limit"><el-input v-model="container.limitMemory" placeholder="1Gi" /></el-form-item></el-col>
        </el-row>
        <div class="resource-setting-title env-setting-title">Environment Variable</div>
        <div v-if="container.env?.length" class="workload-env-head"><span>변수 이름</span><span>변수 값</span><span>Type</span><span>작업</span></div>
        <div v-if="container.env?.length" class="workload-env-list">
          <div v-for="(env, envIndex) in container.env" :key="`${container.name}-${envIndex}`" class="workload-env-row">
            <el-input v-model="env.name" placeholder="변수 이름(예: VECTOR_LOG)" />
            <el-input :model-value="env.valueFrom ? (env.source || 'Kubernetes 참조 변수') : env.value" :readonly="Boolean(env.valueFrom)" placeholder="변수 값" @update:model-value="env.value = $event" />
            <el-tag v-if="env.valueFrom" type="info" effect="plain">참조 변수</el-tag>
            <span v-else class="workload-env-type">일반 변수</span>
            <el-button link type="danger" @click="page.removeWorkloadEnvironment(container, envIndex)">삭제</el-button>
          </div>
        </div>
        <el-button link type="primary" class="add-env-button" @click="page.addWorkloadEnvironment(container)">+ Environment Variable 추가</el-button>
      </section>
    </div>
    <template #footer>
      <el-button @click="page.workloadResourceDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.workloadResourceSaving" @click="page.submitWorkloadResourceSettings">설정 저장</el-button>
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
