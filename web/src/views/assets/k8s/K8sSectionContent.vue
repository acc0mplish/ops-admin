<script setup>
import K8sNamespaceBoard from './K8sNamespaceBoard.vue'
import K8sWorkloadBoard from './K8sWorkloadBoard.vue'
import { Connection, CopyDocument } from '@element-plus/icons-vue'

function serviceTypeTagType(type) {
  return {
    ClusterIP: 'success',
    Headless: 'info',
    NodePort: 'warning',
    LoadBalancer: 'primary',
    ExternalName: 'info'
  }[type] || 'info'
}

function serviceTypeTagClass(type) {
  return type === 'Headless' ? 'service-type-headless' : ''
}

function servicePorts(ports) {
  return String(ports || '').split(',').map((item) => item.trim()).filter(Boolean)
}

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

  <section v-if="page.hasCluster && page.currentTab === 'nodes'" class="section-body node-workspace">
    <div v-if="page.hasItems(page.nodes)" class="node-management-card">
      <div class="node-management-intro">
        <div><strong>节点运行概览</strong><span>查看节点角色、容量与 Pod 分配；标签管理会直接写入 Kubernetes Node。</span></div>
        <el-tag type="info" effect="plain">{{ page.nodes.length }} 个节点</el-tag>
      </div>
      <el-table :data="page.nodes" class="data-table node-table">
      <el-table-column :label="page.t('k8sName')" min-width="210">
        <template #default="{ row }"><div class="node-name-cell"><i></i><div><strong>{{ row.name }}</strong><small>{{ row.internalIP }}</small></div></div></template>
      </el-table-column>
      <el-table-column :label="page.t('k8sRole')" min-width="140"><template #default="{ row }"><el-tag effect="plain" type="info">{{ row.role || '-' }}</el-tag></template></el-table-column>
      <el-table-column :label="page.t('k8sStatus')" width="120"><template #default="{ row }"><el-tag :type="row.status === 'Ready' ? 'success' : 'danger'" effect="light">{{ row.status }}</el-tag></template></el-table-column>
      <el-table-column prop="version" :label="page.t('k8sVersion')" width="130" />
      <el-table-column prop="os" :label="page.t('k8sOs')" min-width="180" show-overflow-tooltip />
      <el-table-column :label="page.t('k8sCpu')" width="118"><template #default="{ row }"><div class="resource-cell"><b>{{ row.cpu }}</b><span>核</span></div></template></el-table-column>
      <el-table-column :label="page.t('k8sMemory')" min-width="145"><template #default="{ row }"><div class="resource-cell"><b>{{ row.memory }}</b></div></template></el-table-column>
      <el-table-column :label="page.t('k8sPodAllocation')" width="138"><template #default="{ row }"><div class="pod-allocation"><b>{{ row.pods }}</b><el-progress :percentage="page.nodePodPercent(row.pods)" :show-text="false" :stroke-width="5" /></div></template></el-table-column>
      <el-table-column :label="page.t('k8sActions')" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="page.openNodeLabels(row)">标签管理</el-button>
          <el-button link type="primary" @click="page.openNodeDetail(row)">{{ page.t('k8sDetail') }}</el-button>
        </template>
      </el-table-column>
      </el-table>
    </div>
    <el-empty v-else :description="page.t('k8sNoRealtimeNodeData')" />
  </section>

  <K8sNamespaceBoard v-if="page.hasCluster && page.currentTab === 'namespaces'" :page="page" />

  <section v-if="page.hasCluster && page.currentTab === 'pods'" class="section-body pod-workspace">
    <el-table v-if="page.hasItems(page.filteredPods)" :data="page.pagedPods" class="data-table pod-management-table">
      <el-table-column prop="name" :label="page.t('k8sPodName')" min-width="260" />
      <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="140" />
      <el-table-column label="工作负载" min-width="200">
        <template #default="{ row }">
          <div v-if="row.workloadName" class="pod-workload-cell">
            <el-tag size="small" effect="plain" class="pod-workload-type">{{ row.workloadName }}</el-tag>
          </div>
          <span v-else class="pod-workload-empty">独立 Pod</span>
        </template>
      </el-table-column>
      <el-table-column :label="page.t('k8sStatus')" width="110">
        <template #default="{ row }">
          <el-tag :type="page.podStatusTagType(row.status)" effect="light" round>{{ row.status || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="page.t('k8sNode')" min-width="180">
        <template #default="{ row }">
          <div class="pod-node-cell">
            <span>节点：{{ row.node || '-' }}</span>
            <small>节点 IP：{{ row.nodeIP || '-' }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="page.t('k8sPodIp')" width="135">
        <template #default="{ row }">{{ row.ip || '-' }}</template>
      </el-table-column>
      <el-table-column prop="restarts" :label="page.t('k8sRestarts')" width="90" />
      <el-table-column prop="age" :label="page.t('k8sAge')" width="90" />
      <el-table-column :label="page.t('k8sActions')" width="230">
        <template #default="{ row }">
          <div class="pod-row-actions">
            <el-button link type="primary" @click="page.openPodDetail(row)">{{ page.t('k8sDetail') }}</el-button>
            <el-button link type="primary" @click="page.openPodLogs(row)">日志</el-button>
            <el-button link type="primary" @click="page.openPodYAML(row)">{{ page.t('k8sYaml') }}</el-button>
            <el-button link type="primary" @click="page.openPodTerminal(row)">{{ page.t('k8sTerminal') }}</el-button>
            <el-button link type="danger" @click="page.handleDeletePod(row)">{{ page.t('k8sDelete') }}</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
    <div v-if="page.hasItems(page.filteredPods)" class="pod-pagination">
      <el-pagination
        background
        layout="total, sizes, prev, pager, next, jumper"
        :total="page.filteredPods.length"
        :current-page="page.podPage"
        :page-size="page.podPageSize"
        :page-sizes="[20, 30, 50, 100]"
        @size-change="page.handlePodPageSizeChange"
        @current-change="page.handlePodPageChange"
      />
    </div>
    <el-empty v-else :description="page.t('k8sNoRealtimePodData')" />
  </section>

  <K8sWorkloadBoard v-if="page.hasCluster && page.currentTab === 'workloads'" :page="page" />

  <section v-if="page.hasCluster && page.currentTab === 'services'" class="section-body service-workspace">
    <el-table v-if="page.hasItems(page.filteredServices)" :data="page.filteredServices" class="data-table service-resource-table">
      <el-table-column :label="page.t('k8sName')" min-width="220">
        <template #default="{ row }">
          <div class="service-name-cell">
            <span class="service-name-icon"><el-icon><Connection /></el-icon></span>
            <el-button link class="service-name-link" @click="page.openServiceDetail(row)">{{ row.name }}</el-button>
            <el-tooltip content="复制服务名称" placement="top">
              <el-button link class="service-copy-button" aria-label="复制服务名称" @click="page.copyServiceName(row)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="namespace" :label="page.t('k8sNamespace')" min-width="130">
        <template #default="{ row }"><el-tag class="service-namespace-tag" effect="plain">{{ row.namespace }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="page.t('k8sType')" width="128">
        <template #default="{ row }"><el-tag :type="serviceTypeTagType(row.type)" :class="serviceTypeTagClass(row.type)" effect="light" round>{{ row.type }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="clusterIP" :label="page.t('k8sClusterIp')" min-width="142">
        <template #default="{ row }"><span class="service-ip-text">{{ row.clusterIP }}</span></template>
      </el-table-column>
      <el-table-column prop="externalIP" :label="page.t('k8sExternalIp')" min-width="142" />
      <el-table-column :label="page.t('k8sPorts')" min-width="245">
        <template #default="{ row }">
          <div class="service-port-list">
            <el-tag v-for="port in servicePorts(row.ports)" :key="port" size="small" effect="plain">{{ port }}</el-tag>
            <span v-if="!servicePorts(row.ports).length" class="service-muted">—</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="page.t('k8sEndpoints')" width="122" align="center">
        <template #default="{ row }"><span class="service-endpoint-count" :class="{ 'is-empty': !row.endpoints }">{{ row.endpoints }}</span></template>
      </el-table-column>
      <el-table-column prop="age" :label="page.t('k8sAge')" width="116" />
      <el-table-column :label="page.t('k8sActions')" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="page.openServiceDetail(row)">{{ page.t('k8sDetail') }}</el-button>
          <el-button link type="primary" @click="page.openServiceEdit(row)">编辑</el-button>
          <el-button link @click="page.openServiceYAML(row)">{{ page.t('k8sYaml') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else :description="page.t('k8sNoRealtimeServiceData')" />
  </section>

  <section v-if="page.hasCluster && page.currentTab === 'ingresses'" class="section-body ingress-workspace">
    <el-table v-if="page.hasItems(page.filteredIngresses)" :data="page.filteredIngresses" class="data-table ingress-resource-table">
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

  <section v-if="page.hasCluster && page.currentTab === 'advanced-network'" class="section-body config-grid network-workspace">
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

  <section v-if="page.hasCluster && page.currentTab === 'config-storage'" class="section-body storage-workspace">
    <div class="config-storage-tabs">
      <div class="config-storage-create-action">
        <el-button v-if="page.configStorageTab === 'storage-classes'" type="primary" @click="page.openStorageClassCreate">
          新增存储类
        </el-button>
        <el-button v-else type="primary" @click="page.openConfigStorageCreate">
          {{ page.configStorageTab === 'configmaps' ? '新建 ConfigMap' : page.configStorageTab === 'secrets' ? '新建 Secret' : '新增存储卷' }}
        </el-button>
      </div>
      <el-tabs v-model="page.configStorageTab">
        <el-tab-pane :label="page.t('k8sConfigMaps')" name="configmaps">
          <el-table v-if="page.hasItems(page.filteredConfigMaps)" :data="page.filteredConfigMaps" class="data-table">
            <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
            <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
            <el-table-column prop="keys" :label="page.t('k8sKeys')" width="100" />
            <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
            <el-table-column :label="page.t('k8sActions')" width="260">
              <template #default="{ row }">
                <el-button link type="primary" @click="page.openConfigMapDetail(row)">{{ page.t('k8sDetail') }}</el-button>
                <el-button link type="primary" @click="page.openConfigMapEdit(row)">编辑</el-button>
                <el-button link type="primary" @click="page.openConfigMapYAML(row)">{{ page.t('k8sYaml') }}</el-button>
                <el-button link type="danger" @click="page.deleteConfigMap(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="page.t('k8sNoRealtimeConfigMapData')" />
        </el-tab-pane>

        <el-tab-pane :label="page.t('k8sSecrets')" name="secrets">
          <el-table v-if="page.hasItems(page.filteredSecrets)" :data="page.filteredSecrets" class="data-table">
            <el-table-column prop="name" :label="page.t('k8sName')" min-width="180" />
            <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="120" />
            <el-table-column prop="type" :label="page.t('k8sType')" min-width="160" />
            <el-table-column prop="age" :label="page.t('k8sAge')" width="110" />
            <el-table-column :label="page.t('k8sActions')" width="260">
              <template #default="{ row }">
                <el-button link type="primary" @click="page.openSecretDetail(row)">{{ page.t('k8sDetail') }}</el-button>
                <el-button link type="primary" @click="page.openSecretEdit(row)">编辑</el-button>
                <el-button link type="primary" @click="page.openSecretYAML(row)">{{ page.t('k8sYaml') }}</el-button>
                <el-button link type="danger" @click="page.deleteSecret(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="page.t('k8sNoRealtimeSecretData')" />
        </el-tab-pane>

        <el-tab-pane label="存储类" name="storage-classes">
          <el-table v-if="page.hasItems(page.filteredStorageClasses)" :data="page.filteredStorageClasses" class="data-table">
            <el-table-column prop="name" :label="page.t('k8sName')" min-width="200" />
            <el-table-column prop="namespaceScope" label="限定命名空间" min-width="140" />
            <el-table-column prop="status" :label="page.t('k8sStatus')" width="120" />
            <el-table-column prop="capacity" :label="page.t('k8sCapacity')" width="120" />
            <el-table-column prop="sourceType" label="存储源" width="110" />
            <el-table-column prop="path" label="路径" min-width="180" show-overflow-tooltip />
            <el-table-column prop="accessModes" label="读取策略" min-width="160" />
            <el-table-column prop="reclaimPolicy" label="回收策略" width="120" />
            <el-table-column :label="page.t('k8sActions')" width="240">
              <template #default="{ row }">
                <el-button link type="primary" @click="page.openStorageDetail(row)">{{ page.t('k8sDetail') }}</el-button>
                <el-button link type="primary" @click="page.openStorageYAML(row)">{{ page.t('k8sYaml') }}</el-button>
                <el-button link type="danger" @click="page.deleteStorageClass(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="暂无存储类，请新增 hostPath 或 NFS 存储类" />
        </el-tab-pane>

        <el-tab-pane label="存储卷" name="storage-volumes">
          <el-table v-if="page.hasItems(page.filteredStorageVolumes)" :data="page.filteredStorageVolumes" class="data-table">
            <el-table-column prop="name" :label="page.t('k8sName')" min-width="200" />
            <el-table-column prop="namespace" :label="page.t('k8sNamespace')" width="150" />
            <el-table-column prop="status" :label="page.t('k8sStatus')" width="120" />
            <el-table-column prop="capacity" :label="page.t('k8sCapacity')" width="120" />
            <el-table-column prop="storageClass" :label="page.t('k8sStorageClass')" min-width="160" />
            <el-table-column prop="accessModes" label="读取策略" min-width="160" />
            <el-table-column :label="page.t('k8sActions')" width="240">
              <template #default="{ row }">
                <el-button link type="primary" @click="page.openStorageDetail(row)">{{ page.t('k8sDetail') }}</el-button>
                <el-button link type="primary" @click="page.openStorageYAML(row)">{{ page.t('k8sYaml') }}</el-button>
                <el-button link type="danger" @click="page.deleteStorageVolume(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="暂无存储卷，请新增 PersistentVolumeClaim" />
        </el-tab-pane>
      </el-tabs>
    </div>
  </section>

  <section v-if="!page.hasCluster" class="section-body">
    <el-empty :description="page.t('k8sNeedClusterFirst')" />
  </section>
</template>
