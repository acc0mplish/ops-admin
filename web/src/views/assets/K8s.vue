<script setup>
import { onMounted, reactive, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import K8sConsoleLayout from './k8s/K8sConsoleLayout.vue'
import K8sSectionContent from './k8s/K8sSectionContent.vue'
import K8sDrawers from './k8s/K8sDrawers.vue'
import K8sDialogs from './k8s/K8sDialogs.vue'
import './k8s/k8s-page.css'
import { K8S_OPERATIONS, K8S_RESOURCE_TARGETS } from '../../api/k8s'
import { useK8sOperationProgress } from '../../composables/useK8sOperationProgress'
import { useK8sClusterState } from '../../composables/useK8sClusterState'
import { useK8sFilters } from '../../composables/useK8sFilters'
import { useK8sResourceActions } from '../../composables/useK8sResourceActions'
import { useK8sServiceEdit } from '../../composables/useK8sServiceEdit'
import { useK8sPodLogs } from '../../composables/useK8sPodLogs'
import { useK8sConfigStorage } from '../../composables/useK8sConfigStorage'
import { useK8sYamlEditor } from '../../composables/useK8sYamlEditor'
import { kt } from '../../utils/k8s-extra-i18n'
// `t` — main-catalog keys (k8sStatusWarning·k8sSectionOverviewDesc·k8sYamlEditor
// …) that stayed in i18n.js. b15577d (en-extract A5) dropped this import while
// converting the moved keys to `kt`, leaving these call sites with a
// ReferenceError that blanked the whole console view at mount.
import { t } from '../../utils/i18n'

// I5-H (i5-plan §3.8) — K8s.vue 2,803행 분할 잔류본: page 객체 조립 + 자식 배선만
// 남긴다(계획 §3.8 "K8s.vue 잔류 ≤650"). 스크립트 본체는 web/src/composables/
// useK8s*.js 7종으로 이동했고 각 커밋 메시지에 원본 좌표가 있다. 자식 8종
// (`k8s/`)과 `:page` 주입은 무변경 — page 키 248종 계약은 그대로다(행동 보존
// +재배선: composable 자유변수 주입·필터/워크로드 보드는 page.x 재배선).
const route = useRoute()
const router = useRouter()

// ---- V2 오퍼레이션 진행 상황 (plan §3.2 H1 — E2 restart 패턴의 기계적 확장) ----
// 9종 오퍼레이션 + restart가 하나의 4단 흐름 머신을 공유한다: 대상별 §16.2 태스크
// 분해 → plan → execute(Idempotency-Key) → 승인(권한 보유 시 자동) → 2초 폴링.
// 머신 본체는 composables/useK8sOperationProgress.js — 여기서는 대상 조립만 한다.
const {
  visible: opProgressVisible,
  title: opProgressTitle,
  rows: opProgressRows,
  percent: opProgressPercent,
  statusText: opStatusText,
  statusTagType: opStatusTagType,
  run: runOpTasks,
  closeOpProgress,
  pushFinishHook
} = useK8sOperationProgress({ onFinish: () => clusterState.refreshCurrentClusterData() })

// 클러스터 로드 오케스트레이션 훅 — useK8sClusterState의 loadClusters/
// loadClusterData가 도메인 간 후크를 이 순서대로 호출한다(원본 loadClusterData
// 1108-1133의 실행 순서 보존). 구성 순서 문제로 여기서 채운다(setup 직후,
// onMounted 전 — 원본과 동일한 타이밍).
const loadHooks = {}
const clusterState = useK8sClusterState({ route, router, runOpTasks, hooks: loadHooks })

// restart 전용 진입 — 기존 호출부 유지를 위한 래퍼. (원본 224-231)
async function runRestartTasks(targets) {
  await runOpTasks(clusterState.cluster.value?.id, kt('restartTaskProgressTitle'), targets.map((item) => ({
    op: K8S_OPERATIONS.restart,
    target: K8S_RESOURCE_TARGETS.workload(item.namespace, item.type, item.name),
    payload: {},
    display: `${item.namespace}/${item.name}`
  })))
}

const filters = useK8sFilters({
  cluster: clusterState.cluster,
  namespaces: clusterState.namespaces,
  pods: clusterState.pods,
  workloads: clusterState.workloads,
  services: clusterState.services,
  ingresses: clusterState.ingresses,
  gatewayApiGateways: clusterState.gatewayApiGateways,
  httpRoutes: clusterState.httpRoutes,
  configMaps: clusterState.configMaps,
  secrets: clusterState.secrets,
  storages: clusterState.storages,
  onTabChange: clusterState.handleTabChange
})

const resourceActions = useK8sResourceActions({
  cluster: clusterState.cluster,
  refreshCurrentClusterData: clusterState.refreshCurrentClusterData,
  runOpTasks,
  runRestartTasks,
  pushFinishHook,
  namespaceFilter: filters.namespaceFilter
})

const serviceEdit = useK8sServiceEdit({
  cluster: clusterState.cluster,
  refreshCurrentClusterData: clusterState.refreshCurrentClusterData,
  runOpTasks
})

const podLogs = useK8sPodLogs({ cluster: clusterState.cluster, router })

const configStorage = useK8sConfigStorage({
  cluster: clusterState.cluster,
  namespaces: clusterState.namespaces,
  namespaceFilter: filters.namespaceFilter,
  namespaceOptions: filters.namespaceOptions,
  storages: clusterState.storages,
  runOpTasks
})

const yamlEditor = useK8sYamlEditor({
  cluster: clusterState.cluster,
  runOpTasks,
  details: {
    namespaceDetail: clusterState.namespaceDetail,
    workloadDetail: resourceActions.workloadDetail,
    podDetail: podLogs.podDetail,
    serviceDetail: serviceEdit.serviceDetail,
    ingressDetail: serviceEdit.ingressDetail,
    istioDetail: resourceActions.istioDetail,
    configMapDetail: configStorage.configMapDetail,
    secretDetail: configStorage.secretDetail,
    storageDetail: configStorage.storageDetail
  }
})

Object.assign(loadHooks, {
  resetNamespaceFilter: filters.resetNamespaceFilter,
  restoreNamespaceFilter: filters.restoreNamespaceFilter,
  clearWorkloadImageCache: filters.clearWorkloadImageCache,
  resetWorkloadSelection: resourceActions.resetWorkloadSelection,
  hydrateWorkloadImages: filters.hydrateWorkloadImages
})

// 워크로드 타입 필터 변경 — 원본 654-658. selectedWorkloads(액션 도메인)와
// 이미지 하이드레이트(필터 도메인)를 잇는 페이지 수준 오케스트레이션이라 잔류.
function handleWorkloadTypeChange(value) {
  filters.workloadTypeFilter.value = value || 'all'
  resourceActions.selectedWorkloads.value = []
  void filters.hydrateWorkloadImages()
}

const page = reactive({
  t,
  ...clusterState,
  ...filters,
  ...resourceActions,
  ...serviceEdit,
  ...podLogs,
  ...configStorage,
  ...yamlEditor,
  handleWorkloadTypeChange
})

onMounted(async () => {
  await clusterState.loadClusters()
})

watch(
  () => [clusterState.currentTab.value, filters.namespaceFilter.value, filters.resourceKeyword.value, filters.workloadTypeFilter.value, clusterState.workloads.value.length],
  () => {
    resourceActions.selectedWorkloads.value = []
    if (clusterState.currentTab.value === 'workloads') {
      void filters.hydrateWorkloadImages()
    }
  }
)
</script>

<template>
  <div class="k8s-page" :class="`k8s-page--${page.currentTab}`" v-loading="page.loading">
    <K8sConsoleLayout :page="page">
      <K8sSectionContent :page="page" />
    </K8sConsoleLayout>
    <K8sDrawers :page="page" />
    <K8sDialogs :page="page" />
    <el-dialog v-model="opProgressVisible" :title="opProgressTitle" width="720px" destroy-on-close @close="closeOpProgress">
      <el-progress :percentage="opProgressPercent" :status="opProgressPercent === 100 ? 'success' : undefined" />
      <el-table :data="opProgressRows" size="small">
        <el-table-column min-width="200">
          <template #default="{ row }">{{ row.display }}</template>
        </el-table-column>
        <el-table-column :label="kt('k8sStatus')" min-width="130">
          <template #default="{ row }">
            <el-tag :type="opStatusTagType(row.status)" size="small">{{ opStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="kt('restartTaskNoticeCol')" min-width="220">
          <template #default="{ row }">
            <span v-if="row.status === 'timeout' || row.status === 'timed_out'">{{ kt('restartTaskTimeout') }}
              <router-link to="/infra/tasks">{{ kt('restartTaskDetailLink') }}</router-link>
            </span>
            <span v-else-if="row.notice">{{ kt(row.notice) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>
