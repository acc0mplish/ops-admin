<script setup>
import K8sNamespaceBoard from './K8sNamespaceBoard.vue'
import K8sWorkloadBoard from './K8sWorkloadBoard.vue'

defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <section v-if="page.hasCluster && page.currentTab === 'overview' && page.overview" class="section-body">
    <div class="stats-grid">
      <article class="stats-panel">
        <span>{{ page.t('k8sHealthScore') }}</span>
        <strong>{{ page.overview.healthScore }}</strong>
        <small>{{ page.t('k8sCurrentAlerts', { count: page.overview.alertCount }) }}</small>
      </article>
      <article class="stats-panel">
        <span>{{ page.t('k8sCpuUsage') }}</span>
        <strong>{{ page.overview.cpuUsage }}</strong>
        <small>{{ page.t('k8sWorkloads') }} {{ page.overview.requestRate }}</small>
      </article>
      <article class="stats-panel">
        <span>{{ page.t('k8sMemoryUsage') }}</span>
        <strong>{{ page.overview.memoryUsage }}</strong>
        <small>{{ page.t('k8sPodUsage', { value: page.overview.podUsage }) }}</small>
      </article>
    </div>

    <div class="summary-band">
      <div class="summary-list">
        <div v-for="item in page.overview.distribution" :key="item.label" class="summary-item">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
        </div>
      </div>
    </div>

    <div v-if="page.overview.certificates?.length" class="cert-band">
      <div class="cert-band-head">
        <div>
          <strong>{{ page.t('k8sCertificates') }}</strong>
          <span>{{ page.t('k8sCertificatesDesc') }}</span>
        </div>
      </div>
      <div class="cert-grid">
        <article v-for="item in page.overview.certificates" :key="item.type" class="cert-card">
          <div class="cert-card-head">
            <div class="cert-title">
              <strong>{{ item.name }}</strong>
              <span>{{ item.subject }}</span>
            </div>
            <el-tag size="small" :type="page.certificateStatusType(item.status)" effect="light">
              {{ page.certificateStatusText(item.status) }}
            </el-tag>
          </div>
          <div class="cert-meta-grid">
            <div class="cert-meta-item">
              <span>{{ page.t('k8sIssuer') }}</span>
              <strong>{{ item.issuer }}</strong>
            </div>
            <div class="cert-meta-item">
              <span>{{ page.t('k8sRemaining') }}</span>
              <strong>{{ page.certificateRemainText(item.daysRemaining) }}</strong>
            </div>
            <div class="cert-meta-item">
              <span>{{ page.t('k8sValidFrom') }}</span>
              <strong>{{ item.notBefore }}</strong>
            </div>
            <div class="cert-meta-item">
              <span>{{ page.t('k8sExpiresAt') }}</span>
              <strong>{{ item.notAfter }}</strong>
            </div>
          </div>
        </article>
      </div>
    </div>
  </section>

  <section v-if="page.hasCluster && page.currentTab === 'nodes'" class="section-body">
    <el-table v-if="page.hasItems(page.nodes)" :data="page.nodes" class="data-table">
      <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
      <el-table-column prop="role" :label="page.t('k8sRole')" width="140" />
      <el-table-column prop="status" :label="page.t('k8sStatus')" width="120" />
      <el-table-column prop="version" :label="page.t('k8sVersion')" width="120" />
      <el-table-column prop="internalIP" :label="page.t('k8sInternalIp')" min-width="150" />
      <el-table-column prop="cpu" :label="page.t('k8sCpu')" width="100" />
      <el-table-column prop="memory" :label="page.t('k8sMemory')" width="120" />
      <el-table-column prop="pods" :label="page.t('k8sPodAllocation')" width="120" />
      <el-table-column :label="page.t('k8sActions')" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="page.openNodeDetail(row)">{{ page.t('k8sDetail') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else :description="page.t('k8sNoRealtimeNodeData')" />
  </section>

  <K8sNamespaceBoard v-if="page.hasCluster && page.currentTab === 'namespaces'" :page="page" />

  <section v-if="page.hasCluster && page.currentTab === 'pods'" class="section-body">
    <el-table v-if="page.hasItems(page.filteredPods)" :data="page.filteredPods" class="data-table">
      <el-table-column prop="name" :label="page.t('k8sPodName')" min-width="240" />
      <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="140" />
      <el-table-column prop="status" :label="page.t('k8sStatus')" width="150" />
      <el-table-column prop="node" :label="page.t('k8sNode')" min-width="160" />
      <el-table-column prop="restarts" :label="page.t('k8sRestarts')" width="100" />
      <el-table-column prop="age" :label="page.t('k8sAge')" width="100" />
      <el-table-column prop="ip" :label="page.t('k8sPodIp')" min-width="120" />
      <el-table-column :label="page.t('k8sActions')" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="page.openPodDetail(row)">{{ page.t('k8sDetail') }}</el-button>
          <el-button link type="primary" @click="page.openPodYAML(row)">{{ page.t('k8sYaml') }}</el-button>
          <el-button link type="primary" @click="page.openPodTerminal(row)">{{ page.t('k8sTerminal') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else :description="page.t('k8sNoRealtimePodData')" />
  </section>

  <K8sWorkloadBoard v-if="page.hasCluster && page.currentTab === 'workloads'" :page="page" />

  <section v-if="page.hasCluster && page.currentTab === 'services'" class="section-body">
    <el-table v-if="page.hasItems(page.filteredServices)" :data="page.filteredServices" class="data-table">
      <el-table-column prop="name" :label="page.t('k8sName')" min-width="160" />
      <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
      <el-table-column prop="type" :label="page.t('k8sType')" width="120" />
      <el-table-column prop="clusterIP" :label="page.t('k8sClusterIp')" min-width="140" />
      <el-table-column prop="externalIP" :label="page.t('k8sExternalIp')" min-width="150" />
      <el-table-column prop="ports" :label="page.t('k8sPorts')" min-width="160" />
      <el-table-column prop="endpoints" :label="page.t('k8sEndpoints')" width="110" />
      <el-table-column :label="page.t('k8sActions')" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="page.openServiceDetail(row)">{{ page.t('k8sDetail') }}</el-button>
          <el-button link type="primary" @click="page.openServiceYAML(row)">{{ page.t('k8sYaml') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else :description="page.t('k8sNoRealtimeServiceData')" />
  </section>

  <section v-if="page.hasCluster && page.currentTab === 'ingresses'" class="section-body">
    <el-table v-if="page.hasItems(page.filteredIngresses)" :data="page.filteredIngresses" class="data-table">
      <el-table-column prop="name" :label="page.t('k8sName')" min-width="160" />
      <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
      <el-table-column prop="host" :label="page.t('k8sHost')" min-width="180" />
      <el-table-column prop="address" :label="page.t('k8sAddress')" min-width="140" />
      <el-table-column prop="tls" :label="page.t('k8sTls')" width="120" />
      <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
      <el-table-column :label="page.t('k8sActions')" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="page.openIngressDetail(row)">{{ page.t('k8sDetail') }}</el-button>
          <el-button link type="primary" @click="page.openIngressYAML(row)">{{ page.t('k8sYaml') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else :description="page.t('k8sNoRealtimeIngressData')" />
  </section>

  <section v-if="page.hasCluster && page.currentTab === 'advanced-network'" class="section-body config-grid">
    <div class="subsection">
      <div class="subsection-head">
        <strong>{{ page.t('k8sGatewayApiGateways') }}</strong>
        <el-button type="primary" plain @click="page.openIstioCreateDialog('gatewayapi')">{{ page.t('k8sCreate') }}</el-button>
      </div>
      <el-table v-if="page.hasItems(page.filteredGatewayApiGateways)" :data="page.filteredGatewayApiGateways" class="data-table">
        <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
        <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
        <el-table-column prop="hosts" :label="page.t('k8sHost')" min-width="220" />
        <el-table-column prop="address" :label="page.t('k8sAddress')" min-width="180" />
        <el-table-column prop="ports" :label="page.t('k8sPorts')" min-width="160" />
        <el-table-column prop="target" :label="page.t('k8sType')" min-width="160" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
        <el-table-column :label="page.t('k8sActions')" width="240" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button link type="primary" @click="page.openIstioResourceDetail(row, 'gatewayapi')">{{ page.t('k8sDetail') }}</el-button>
              <el-button link type="primary" @click="page.openIstioResourceYAML(row, 'gatewayapi')">{{ page.t('k8sYaml') }}</el-button>
              <el-button link type="danger" @click="page.handleDeleteIstioResource(row, 'gatewayapi')">{{ page.t('k8sDelete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeAdvancedNetworkData')" />
    </div>

    <div class="subsection">
      <div class="subsection-head">
        <strong>{{ page.t('k8sHttpRoutes') }}</strong>
        <el-button type="primary" plain @click="page.openIstioCreateDialog('httproute')">{{ page.t('k8sCreate') }}</el-button>
      </div>
      <el-table v-if="page.hasItems(page.filteredHTTPRoutes)" :data="page.filteredHTTPRoutes" class="data-table">
        <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
        <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
        <el-table-column prop="hosts" :label="page.t('k8sHost')" min-width="220" />
        <el-table-column prop="gateways" :label="page.t('k8sGateways')" min-width="180" />
        <el-table-column prop="target" :label="page.t('k8sTarget')" min-width="200" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
        <el-table-column :label="page.t('k8sActions')" min-width="300" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button link type="primary" @click="page.openIstioResourceDetail(row, 'httproute')">{{ page.t('k8sDetail') }}</el-button>
              <el-button link type="primary" @click="page.openIstioResourceYAML(row, 'httproute')">{{ page.t('k8sYaml') }}</el-button>
              <el-button link type="warning" @click="page.openTrafficDialog({ ...row, resourceType: 'httproute' })">{{ page.t('k8sTraffic') }}</el-button>
              <el-button link type="danger" @click="page.handleDeleteIstioResource(row, 'httproute')">{{ page.t('k8sDelete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeAdvancedNetworkData')" />
    </div>
  </section>

  <section v-if="page.hasCluster && page.currentTab === 'config-storage'" class="section-body config-grid">
    <div class="subsection">
      <div class="subsection-head">
        <strong>{{ page.t('k8sConfigMaps') }}</strong>
      </div>
      <el-table v-if="page.hasItems(page.filteredConfigMaps)" :data="page.filteredConfigMaps" class="data-table">
        <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
        <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
        <el-table-column prop="keys" :label="page.t('k8sKeys')" width="100" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
        <el-table-column :label="page.t('k8sActions')" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="page.openConfigMapDetail(row)">{{ page.t('k8sDetail') }}</el-button>
            <el-button link type="primary" @click="page.openConfigMapYAML(row)">{{ page.t('k8sYaml') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeConfigMapData')" />
    </div>

    <div class="subsection">
      <div class="subsection-head">
        <strong>{{ page.t('k8sSecrets') }}</strong>
      </div>
      <el-table v-if="page.hasItems(page.filteredSecrets)" :data="page.filteredSecrets" class="data-table">
        <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
        <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
        <el-table-column prop="type" :label="page.t('k8sType')" min-width="160" />
        <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
        <el-table-column :label="page.t('k8sActions')" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="page.openSecretDetail(row)">{{ page.t('k8sDetail') }}</el-button>
            <el-button link type="primary" @click="page.openSecretYAML(row)">{{ page.t('k8sYaml') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeSecretData')" />
    </div>

    <div class="subsection">
      <div class="subsection-head">
        <strong>{{ page.t('k8sStorage') }}</strong>
      </div>
      <el-table v-if="page.hasItems(page.filteredStorages)" :data="page.filteredStorages" class="data-table">
        <el-table-column prop="name" :label="page.t('k8sName')" min-width="200" />
        <el-table-column prop="kind" :label="page.t('k8sKind')" width="140" />
        <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
        <el-table-column prop="status" :label="page.t('k8sStatus')" width="120" />
        <el-table-column prop="capacity" :label="page.t('k8sCapacity')" width="120" />
        <el-table-column prop="storageClass" :label="page.t('k8sStorageClass')" min-width="140" />
        <el-table-column :label="page.t('k8sActions')" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="page.openStorageDetail(row)">{{ page.t('k8sDetail') }}</el-button>
            <el-button link type="primary" @click="page.openStorageYAML(row)">{{ page.t('k8sYaml') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="page.t('k8sNoRealtimeStorageData')" />
    </div>
  </section>

  <section v-if="!page.hasCluster" class="section-body">
    <el-empty :description="page.t('k8sNeedClusterFirst')" />
  </section>
</template>
