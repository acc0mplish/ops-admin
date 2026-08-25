<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Bell, Connection, DataAnalysis, Monitor, Operation, Refresh, SetUp } from '@element-plus/icons-vue'
import { appDefinitions } from '../utils/apps'
import './BusinessTopology.css'

const router = useRouter()
const selectedKey = ref('')

const capabilityMap = {
  console: { icon: Monitor, tone: 'blue', role: '统一入口', description: '提供统一工作台、系统配置、用户权限与操作审计入口。', capabilities: ['仪表盘与全局视图', '业务拓扑图', '用户与权限管理', '组织与菜单配置', '登录与操作审计'], consumers: ['assets', 'containers', 'ops', 'applications', 'notify', 'integration', 'monitor', 'domains'] },
  assets: { icon: SetUp, tone: 'cyan', role: '资源底座', description: '统一管理环境、主机、云账号、数据库、网关等基础运行资源。', capabilities: ['资产概览', '环境模型', '主机与凭据管理', '云账号与云主机同步', '数据库管理', '网关管理', '终端登录'], consumers: ['containers', 'ops', 'applications', 'monitor', 'domains'] },
  containers: { icon: Connection, tone: 'teal', role: '容器运行', description: '统一管理 Kubernetes 集群、工作负载、服务网络和配置存储。', capabilities: ['服务管理', '服务健康诊断', '集群与节点管理', '工作负载', 'Pod 与服务', 'Ingress 与网络', '配置与存储'], consumers: ['applications', 'monitor', 'notify'] },
  ops: { icon: Operation, tone: 'violet', role: '运维执行', description: '基于资产与环境执行脚本、命令、文件分发、作业和定时任务。', capabilities: ['脚本库', '命令与脚本执行', '文件分发', '定时任务', '作业编排', '人工确认', '执行历史与模板'], consumers: ['applications', 'notify', 'monitor'] },
  applications: { icon: Connection, tone: 'green', role: '应用交付', description: '管理应用、构建任务、构建历史、镜像仓库与 CI/CD 流水线。', capabilities: ['应用管理', '构建任务', '构建历史', '镜像仓库', 'CI/CD 流水线'], consumers: ['containers', 'monitor', 'notify'] },
  notify: { icon: Bell, tone: 'pink', role: '消息触达', description: '通过模板、渠道和通知规则将告警或运维结果触达相关人员。', capabilities: ['通知规则', '消息模板', '通知媒介', '发送日志'], consumers: ['console'] },
  integration: { icon: Connection, tone: 'indigo', role: '外部集成与智能分析', description: '提供 AI 助手、模型工具集、知识库与云费用分析能力。', capabilities: ['导航管理', 'AI 助手与会话', '模型与工具集', '知识库管理', '云费用看板', '账单同步与费用拆分', '优化建议与资源分析'], consumers: ['console'] },
  monitor: { icon: DataAnalysis, tone: 'orange', role: '可观测性', description: '采集并查询指标、日志与告警，形成仪表盘、告警规则和事件闭环。', capabilities: ['监控概览与仪表盘', '监控数据源', '指标与日志查询', '链路追踪', '告警规则与事件', '监控大屏', '告警治理与巡检'], consumers: ['notify', 'integration'] },
  domains: { icon: Connection, tone: 'purple', role: '域名与解析', description: '统一管理公网域名、SSL 证书、内网 Zone、DNS 设置及解析审计。', capabilities: ['公网域名', 'DNS 账号', 'SSL 证书', 'Zone 管理', '内网 DNS 设置', '解析测试', '操作审计'], consumers: ['assets', 'monitor', 'notify'] }
}

const nodes = computed(() => appDefinitions.filter(item => capabilityMap[item.key]).map(item => ({ ...item, ...capabilityMap[item.key] })))
const selectedNode = computed(() => nodes.value.find(item => item.key === selectedKey.value) || null)
const capabilityCount = computed(() => nodes.value.reduce((total, item) => total + item.capabilities.length, 0))
const edges = computed(() => nodes.value.flatMap(item => item.consumers.filter(target => nodes.value.some(node => node.key === target)).map(target => ({ source: item.key, target }))))

function openApp(node) { router.push(node.defaultRoute) }
function selectNode(key) { selectedKey.value = selectedKey.value === key ? '' : key }
</script>

<template>
  <main class="business-topology-page">
    <section class="topology-hero">
      <div><p>OPS ADMIN · BUSINESS TOPOLOGY</p><h1>业务拓扑图</h1><span>以“应用 → 多项业务能力”的方式展示当前平台的业务能力版图。</span></div>
      <div class="hero-meta"><div><strong>{{ nodes.length }}</strong><span>个业务应用</span></div><div><strong>{{ capabilityCount }}</strong><span>项实现能力</span></div></div>
    </section>

    <section class="topology-legend"><div><span class="legend-resource"></span>资源与数据</div><div><span class="legend-execution"></span>运维与交付</div><div><span class="legend-observe"></span>监控与告警</div><div><span class="legend-output"></span>通知与智能分析</div><p>点击应用或能力卡片查看详细职责；双击应用进入对应工作台。</p></section>

    <section class="topology-canvas">
      <header><strong>{{ nodes.length }} 个应用</strong><b>→</b><strong>{{ capabilityCount }} 项业务能力</strong><span>每一列代表一个应用及其实现功能</span></header>
      <div class="capability-lanes">
        <article v-for="node in nodes" :key="node.key" class="capability-lane" :class="[`tone-${node.tone}`, { selected: selectedKey === node.key }]">
          <button class="application-parent" type="button" @click="selectNode(node.key)" @dblclick="openApp(node)">
            <i class="app-icon"><el-icon><component :is="node.icon" /></el-icon></i><span><small>{{ node.role }}</small><b>{{ node.name }}</b></span><em title="进入应用" @click.stop="openApp(node)">↗</em>
          </button>
          <div class="stem"></div>
          <button v-for="capability in node.capabilities" :key="capability" type="button" class="capability-node" @click="selectNode(node.key)">{{ capability }}</button>
        </article>
      </div>
    </section>

    <section class="capability-section">
      <div class="section-heading"><div><p>APPLICATION CAPABILITIES</p><h2>{{ selectedNode ? selectedNode.name : '应用功能说明' }}</h2></div><el-button :icon="Refresh" @click="selectedKey = ''">清除选择</el-button></div>
      <div v-if="selectedNode" class="capability-detail" :class="`tone-${selectedNode.tone}`"><div><span>应用定位</span><strong>{{ selectedNode.role }}</strong><p>{{ selectedNode.description }}</p></div><div><span>实现功能</span><ul><li v-for="item in selectedNode.capabilities" :key="item">{{ item }}</li></ul></div><div><span>业务输出给</span><ul><li v-for="key in selectedNode.consumers" :key="key">{{ nodes.find(item => item.key === key)?.name }}</li></ul></div></div>
      <div v-else class="capability-grid"><article v-for="node in nodes" :key="node.key" :class="`tone-${node.tone}`" @click="selectNode(node.key)"><strong>{{ node.name }}</strong><p>{{ node.description }}</p></article></div>
    </section>

    <section class="topology-note"><el-icon><Connection /></el-icon><div><strong>链路说明</strong><p>这里展示平台的业务依赖与信息输出方向，用于理解能力边界和联动路径，不替代实时调用链或资源依赖图。</p></div><span>{{ edges.length }} 条业务关系</span></section>
  </main>
</template>
