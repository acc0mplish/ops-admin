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

  <el-dialog v-model="page.namespaceCreateVisible" :title="page.t('k8sCreateNamespace')" width="480px" destroy-on-close>
    <el-form label-position="top">
      <el-form-item :label="page.t('k8sNamespaceName')" required>
        <el-input v-model="page.namespaceCreateForm.name" maxlength="63" show-word-limit placeholder="例如 game-prod" />
      </el-form-item>
      <div class="dialog-tip">{{ page.t('k8sCreateNamespaceHint') }}</div>
    </el-form>
    <template #footer>
      <el-button @click="page.namespaceCreateVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.namespaceCreateSaving" @click="page.submitNamespaceCreate">{{ page.t('k8sCreate') }}</el-button>
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

  <el-dialog v-model="page.workloadResourceDialogVisible" title="更新 Pod 设置" width="920px" class="workload-resource-dialog-wrap" destroy-on-close>
    <div class="workload-resource-dialog">
      <div class="workload-resource-summary">
        <div><span>命名空间</span><strong>{{ page.workloadResourceForm.namespace }}</strong></div>
        <div><span>工作负载</span><strong>{{ page.workloadResourceForm.workloadName }} · {{ page.workloadResourceForm.workloadType }}</strong></div>
      </div>
      <p class="dialog-tip">可维护每个容器的资源 Request / Limit、镜像拉取策略与环境变量；保存后将更新 Pod 模板并触发工作负载滚动更新。</p>
      <section v-for="container in page.workloadResourceForm.containers" :key="container.name" class="container-resource-card">
        <div class="container-resource-head">
          <strong>{{ container.name }}</strong>
          <span>容器配置</span>
        </div>
        <el-row :gutter="14" class="container-basic-row">
          <el-col :span="15"><el-form-item label="镜像"><el-input :model-value="container.image || '-'" readonly /></el-form-item></el-col>
          <el-col :span="9"><el-form-item label="镜像拉取策略" class="image-pull-policy-field">
            <el-select v-model="container.imagePullPolicy" class="image-pull-policy-select">
              <el-option label="Always（始终拉取）" value="Always" />
              <el-option label="IfNotPresent（本地优先）" value="IfNotPresent" />
              <el-option label="Never（仅本地镜像）" value="Never" />
            </el-select>
          </el-form-item></el-col>
        </el-row>
        <div class="resource-setting-title">CPU / 内存 Request 与 Limit</div>
        <el-row :gutter="14">
          <el-col :span="6"><el-form-item label="CPU Request"><el-input v-model="container.requestCPU" placeholder="100m" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="CPU Limit"><el-input v-model="container.limitCPU" placeholder="1" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="内存 Request"><el-input v-model="container.requestMemory" placeholder="256Mi" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="内存 Limit"><el-input v-model="container.limitMemory" placeholder="1Gi" /></el-form-item></el-col>
        </el-row>
        <div class="resource-setting-title env-setting-title">环境变量</div>
        <div v-if="container.env?.length" class="workload-env-head"><span>变量名</span><span>变量值</span><span>类型</span><span>操作</span></div>
        <div v-if="container.env?.length" class="workload-env-list">
          <div v-for="(env, envIndex) in container.env" :key="`${container.name}-${envIndex}`" class="workload-env-row">
            <el-input v-model="env.name" placeholder="变量名，例如 VECTOR_LOG" />
            <el-input :model-value="env.valueFrom ? (env.source || 'Kubernetes 引用变量') : env.value" :readonly="Boolean(env.valueFrom)" placeholder="变量值" @update:model-value="env.value = $event" />
            <el-tag v-if="env.valueFrom" type="info" effect="plain">引用变量</el-tag>
            <span v-else class="workload-env-type">普通变量</span>
            <el-button link type="danger" @click="page.removeWorkloadEnvironment(container, envIndex)">删除</el-button>
          </div>
        </div>
        <el-button link type="primary" class="add-env-button" @click="page.addWorkloadEnvironment(container)">+ 新增环境变量</el-button>
      </section>
    </div>
    <template #footer>
      <el-button @click="page.workloadResourceDialogVisible = false">{{ page.t('cancel') }}</el-button>
      <el-button type="primary" :loading="page.workloadResourceSaving" @click="page.submitWorkloadResourceSettings">保存设置</el-button>
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
