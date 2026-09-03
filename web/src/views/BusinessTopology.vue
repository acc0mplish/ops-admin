<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Bell, Connection, DataAnalysis, Monitor, Operation, Refresh, SetUp } from '@element-plus/icons-vue'
import { appDefinitions } from '../utils/apps'
import { bt } from '../utils/topology-i18n'
import './BusinessTopology.css'

const router = useRouter()
const selectedKey = ref('')

const capabilityMap = computed(() => ({
  console: { icon: Monitor, tone: 'blue', role: bt('consoleRole'), description: bt('consoleDesc'), capabilities: bt('consoleCaps'), consumers: ['assets', 'containers', 'ops', 'applications', 'notify', 'integration', 'monitor', 'domains'] },
  assets: { icon: SetUp, tone: 'cyan', role: bt('assetsRole'), description: bt('assetsDesc'), capabilities: bt('assetsCaps'), consumers: ['containers', 'ops', 'applications', 'monitor', 'domains'] },
  containers: { icon: Connection, tone: 'teal', role: bt('containersRole'), description: bt('containersDesc'), capabilities: bt('containersCaps'), consumers: ['applications', 'monitor', 'notify'] },
  ops: { icon: Operation, tone: 'violet', role: bt('opsRole'), description: bt('opsDesc'), capabilities: bt('opsCaps'), consumers: ['applications', 'notify', 'monitor'] },
  applications: { icon: Connection, tone: 'green', role: bt('applicationsRole'), description: bt('applicationsDesc'), capabilities: bt('applicationsCaps'), consumers: ['containers', 'monitor', 'notify'] },
  notify: { icon: Bell, tone: 'pink', role: bt('notifyRole'), description: bt('notifyDesc'), capabilities: bt('notifyCaps'), consumers: ['console'] },
  integration: { icon: Connection, tone: 'indigo', role: bt('integrationRole'), description: bt('integrationDesc'), capabilities: bt('integrationCaps'), consumers: ['console'] },
  monitor: { icon: DataAnalysis, tone: 'orange', role: bt('monitorRole'), description: bt('monitorDesc'), capabilities: bt('monitorCaps'), consumers: ['notify', 'integration'] },
  domains: { icon: Connection, tone: 'purple', role: bt('domainsRole'), description: bt('domainsDesc'), capabilities: bt('domainsCaps'), consumers: ['assets', 'monitor', 'notify'] }
}))

const nodes = computed(() => appDefinitions.filter(item => capabilityMap.value[item.key]).map(item => ({ ...item, ...capabilityMap.value[item.key] })))
const selectedNode = computed(() => nodes.value.find(item => item.key === selectedKey.value) || null)
const capabilityCount = computed(() => nodes.value.reduce((total, item) => total + item.capabilities.length, 0))
const edges = computed(() => nodes.value.flatMap(item => item.consumers.filter(target => nodes.value.some(node => node.key === target)).map(target => ({ source: item.key, target }))))

function openApp(node) { router.push(node.defaultRoute) }
function selectNode(key) { selectedKey.value = selectedKey.value === key ? '' : key }
</script>

<template>
  <main class="business-topology-page">
    <section class="topology-hero">
      <div><p>OPS ADMIN · BUSINESS TOPOLOGY</p><h1>{{ bt('title') }}</h1><span>{{ bt('heroDesc') }}</span></div>
      <div class="hero-meta"><div><strong>{{ nodes.length }}</strong><span>{{ bt('businessApps') }}</span></div><div><strong>{{ capabilityCount }}</strong><span>{{ bt('implementedCapabilities') }}</span></div></div>
    </section>

    <section class="topology-legend"><div><span class="legend-resource"></span>{{ bt('resourceData') }}</div><div><span class="legend-execution"></span>{{ bt('opsDelivery') }}</div><div><span class="legend-observe"></span>{{ bt('monitoringAlerting') }}</div><div><span class="legend-output"></span>{{ bt('notificationAnalysis') }}</div><p>{{ bt('legendHint') }}</p></section>

    <section class="topology-canvas">
      <header><strong>{{ nodes.length }} {{ bt('applications') }}</strong><b>→</b><strong>{{ capabilityCount }} {{ bt('capabilities') }}</strong><span>{{ bt('columnHint') }}</span></header>
      <div class="capability-lanes">
        <article v-for="node in nodes" :key="node.key" class="capability-lane" :class="[`tone-${node.tone}`, { selected: selectedKey === node.key }]">
          <button class="application-parent" type="button" @click="selectNode(node.key)" @dblclick="openApp(node)">
            <i class="app-icon"><el-icon><component :is="node.icon" /></el-icon></i><span><small>{{ node.role }}</small><b>{{ node.name }}</b></span><em :title="bt('enterApplication')" @click.stop="openApp(node)">↗</em>
          </button>
          <div class="stem"></div>
          <button v-for="capability in node.capabilities" :key="capability" type="button" class="capability-node" @click="selectNode(node.key)">{{ capability }}</button>
        </article>
      </div>
    </section>

    <section class="capability-section">
      <div class="section-heading"><div><p>APPLICATION CAPABILITIES</p><h2>{{ selectedNode ? selectedNode.name : bt('featureDescription') }}</h2></div><el-button :icon="Refresh" @click="selectedKey = ''">{{ bt('clearSelection') }}</el-button></div>
      <div v-if="selectedNode" class="capability-detail" :class="`tone-${selectedNode.tone}`"><div><span>{{ bt('applicationRole') }}</span><strong>{{ selectedNode.role }}</strong><p>{{ selectedNode.description }}</p></div><div><span>{{ bt('implementedFeatures') }}</span><ul><li v-for="item in selectedNode.capabilities" :key="item">{{ item }}</li></ul></div><div><span>{{ bt('outputsTo') }}</span><ul><li v-for="key in selectedNode.consumers" :key="key">{{ nodes.find(item => item.key === key)?.name }}</li></ul></div></div>
      <div v-else class="capability-grid"><article v-for="node in nodes" :key="node.key" :class="`tone-${node.tone}`" @click="selectNode(node.key)"><strong>{{ node.name }}</strong><p>{{ node.description }}</p></article></div>
    </section>

    <section class="topology-note"><el-icon><Connection /></el-icon><div><strong>{{ bt('relationshipNote') }}</strong><p>{{ bt('relationshipDesc') }}</p></div><span>{{ edges.length }} {{ bt('businessRelationships') }}</span></section>
  </main>
</template>
