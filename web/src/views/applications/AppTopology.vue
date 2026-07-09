<template>
  <div class="topology-page">
    <section class="hero">
      <div>
        <p class="eyebrow">APPLICATION TOPOLOGY</p>
        <h1>应用拓扑</h1>
        <p>把应用、主机、K8s、数据库、监控告警和发布流水线串成一张可追踪关系图。</p>
      </div>
      <div class="filters">
        <el-select v-model="query.appId" clearable filterable placeholder="选择应用" @change="loadData">
          <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="query.env" clearable placeholder="选择环境" @change="loadData">
          <el-option v-for="item in environments" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
        </el-select>
        <el-button type="primary" @click="loadData">刷新拓扑</el-button>
      </div>
    </section>

    <section class="summary-grid">
      <div v-for="item in summaryCards" :key="item.label" class="summary-card">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
      </div>
    </section>

    <section class="topology-canvas">
      <div class="node app-node">
        <span>应用</span>
        <strong>{{ data.app?.name || '未选择应用' }}</strong>
        <em>{{ data.app?.code || '-' }}</em>
      </div>
      <div class="relation-grid">
        <div class="relation-block">
          <h3>主机资源</h3>
          <div v-for="item in data.hosts || []" :key="item.id" class="mini-node">
            <strong>{{ item.hostName }}</strong>
            <span>{{ item.sshIp || item.privateIp || item.publicIp || '-' }}</span>
          </div>
          <el-empty v-if="!data.hosts?.length" description="暂无主机关联" :image-size="70" />
        </div>
        <div class="relation-block">
          <h3>K8s 集群</h3>
          <div v-for="item in data.k8sClusters || []" :key="item.id" class="mini-node">
            <strong>{{ item.name }}</strong>
            <span>{{ item.version }} / {{ item.nodeCount }} 节点</span>
          </div>
          <el-empty v-if="!data.k8sClusters?.length" description="暂无 K8s 关联" :image-size="70" />
        </div>
        <div class="relation-block">
          <h3>数据库</h3>
          <div v-for="item in data.databases || []" :key="item.id" class="mini-node">
            <strong>{{ item.name }}</strong>
            <span>{{ item.host }}:{{ item.port }} / {{ item.dbName || '-' }}</span>
          </div>
          <el-empty v-if="!data.databases?.length" description="暂无数据库关联" :image-size="70" />
        </div>
        <div class="relation-block">
          <h3>监控告警</h3>
          <div v-for="item in data.alerts || []" :key="item.id" class="mini-node warning">
            <strong>{{ item.ruleName }}</strong>
            <span>{{ item.severity }} / {{ item.status }}</span>
          </div>
          <el-empty v-if="!data.alerts?.length" description="暂无告警关联" :image-size="70" />
        </div>
      </div>
    </section>

    <section class="two-columns">
      <div class="panel">
        <h2>发布记录</h2>
        <el-table :data="data.releases || []" height="280">
          <el-table-column prop="version" label="版本" min-width="140" />
          <el-table-column prop="env" label="环境" width="100" />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column prop="stage" label="阶段" width="120" />
          <el-table-column prop="createTime" label="时间" min-width="180" />
        </el-table>
      </div>
      <div class="panel">
        <h2>流水线执行</h2>
        <el-table :data="data.pipelineRuns || []" height="280">
          <el-table-column prop="pipelineName" label="流水线" min-width="160" />
          <el-table-column prop="env" label="环境" width="100" />
          <el-table-column prop="imageTag" label="镜像 Tag" min-width="140" />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column prop="createTime" label="时间" min-width="180" />
        </el-table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { queryOpsApplicationOptions, queryOpsApplicationTopology, queryOpsEnvironmentList } from '../../api/ops'

const query = reactive({ appId: undefined, env: '' })
const appOptions = ref([])
const environments = ref([])
const data = ref({})

const summaryCards = computed(() => {
  const summary = data.value.summary || {}
  return [
    { label: '主机', value: summary.hosts || 0 },
    { label: 'K8s 集群', value: summary.k8sClusters || 0 },
    { label: '数据库', value: summary.databases || 0 },
    { label: '告警', value: summary.alerts || 0 },
    { label: '发布', value: summary.releases || 0 },
    { label: '流水线', value: summary.pipelineRuns || 0 }
  ]
})

async function loadOptions() {
  const [apps, envs] = await Promise.all([queryOpsApplicationOptions(), queryOpsEnvironmentList({ status: 1 })])
  appOptions.value = apps || []
  environments.value = envs || []
}

async function loadData() {
  data.value = await queryOpsApplicationTopology(query)
}

onMounted(async () => {
  await loadOptions()
  await loadData()
})
</script>

<style scoped>
.topology-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.hero,
.panel,
.topology-canvas,
.summary-card {
  background: #fff;
  border: 1px solid #dfe8f6;
  border-radius: 8px;
}
.hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 24px;
}
.eyebrow {
  margin: 0 0 8px;
  color: #2f6eea;
  font-size: 12px;
  font-weight: 800;
}
h1,
h2,
h3 {
  margin: 0;
  color: #071a3d;
}
.hero p:last-child {
  margin: 10px 0 0;
  color: #6d7f9f;
}
.filters {
  display: flex;
  gap: 12px;
}
.filters .el-select {
  width: 220px;
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(120px, 1fr));
  gap: 14px;
}
.summary-card {
  padding: 18px;
}
.summary-card span {
  color: #7889a8;
}
.summary-card strong {
  display: block;
  margin-top: 8px;
  font-size: 28px;
  color: #071a3d;
}
.topology-canvas {
  padding: 24px;
}
.node {
  width: 260px;
  margin: 0 auto 26px;
  padding: 20px;
  text-align: center;
  background: linear-gradient(135deg, #3267d6, #20b6a6);
  color: #fff;
  border-radius: 8px;
}
.node span,
.node em {
  display: block;
  opacity: .85;
  font-style: normal;
}
.node strong {
  display: block;
  margin: 6px 0;
  font-size: 22px;
}
.relation-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}
.relation-block {
  min-height: 240px;
  padding: 16px;
  background: #f7faff;
  border: 1px solid #e1eaf7;
  border-radius: 8px;
}
.relation-block h3 {
  margin-bottom: 12px;
  font-size: 16px;
}
.mini-node {
  padding: 12px;
  margin-bottom: 10px;
  background: #fff;
  border-left: 4px solid #2f6eea;
  border-radius: 6px;
}
.mini-node.warning {
  border-left-color: #f59e0b;
}
.mini-node strong,
.mini-node span {
  display: block;
}
.mini-node span {
  margin-top: 4px;
  color: #6d7f9f;
  font-size: 12px;
}
.two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}
.panel {
  padding: 18px;
}
.panel h2 {
  margin-bottom: 14px;
  font-size: 18px;
}
</style>
