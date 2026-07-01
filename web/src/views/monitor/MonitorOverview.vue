<script setup>
import { onMounted, ref } from 'vue'
import { queryMonitorOverview } from '../../api/monitor'

const loading = ref(false)
const overview = ref({
  datasourceCount: 0,
  ruleCount: 0,
  firingCount: 0,
  recoveredCount: 0,
  severity: [],
  recentEvents: []
})

function statusType(status) {
  if (status === 'firing') return 'danger'
  if (status === 'claimed') return 'warning'
  if (status === 'recovered') return 'success'
  return 'info'
}

async function loadData() {
  loading.value = true
  try {
    overview.value = await queryMonitorOverview()
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-page" v-loading="loading">
    <section class="monitor-hero">
      <div>
        <h1>监控概览</h1>
        <p>聚合数据源、告警规则与当前告警态势，先把值班视角立起来。</p>
      </div>
      <el-button type="primary" @click="loadData">刷新</el-button>
    </section>

    <section class="metric-grid">
      <div class="metric-card">
        <span>数据源</span>
        <strong>{{ overview.datasourceCount || 0 }}</strong>
        <small>Prometheus / VictoriaMetrics</small>
      </div>
      <div class="metric-card">
        <span>告警规则</span>
        <strong>{{ overview.ruleCount || 0 }}</strong>
        <small>周期评估规则数量</small>
      </div>
      <div class="metric-card danger">
        <span>当前告警</span>
        <strong>{{ overview.firingCount || 0 }}</strong>
        <small>触发中或已认领</small>
      </div>
      <div class="metric-card success">
        <span>已恢复</span>
        <strong>{{ overview.recoveredCount || 0 }}</strong>
        <small>自动恢复事件</small>
      </div>
    </section>

    <section class="content-grid">
      <div class="panel">
        <div class="panel-title">告警等级</div>
        <div class="severity-list">
          <div v-for="item in overview.severity || []" :key="item.severity" class="severity-row">
            <span>{{ item.severity }}</span>
            <el-progress :percentage="Math.min(100, Number(item.count || 0) * 20)" :show-text="false" />
            <strong>{{ item.count || 0 }}</strong>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-title">最近告警</div>
        <el-table :data="overview.recentEvents || []" border>
          <el-table-column prop="ruleName" label="规则" min-width="160" />
          <el-table-column prop="severity" label="等级" width="90" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusType(row.status)">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="summary" label="摘要" min-width="260" show-overflow-tooltip />
          <el-table-column prop="lastTriggerAt" label="最近触发" width="180" />
        </el-table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 18px; }
.monitor-hero { display: flex; justify-content: space-between; align-items: center; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.monitor-hero h1 { margin: 0 0 8px; font-size: 28px; color: #0f1f3d; }
.monitor-hero p { margin: 0; color: #6f7f9f; }
.metric-grid { display: grid; grid-template-columns: repeat(4, minmax(180px, 1fr)); gap: 16px; }
.metric-card { padding: 20px; background: #fff; border: 1px solid #e6edf7; border-radius: 12px; }
.metric-card span { color: #7584a3; }
.metric-card strong { display: block; margin: 12px 0 8px; font-size: 30px; color: #10213f; }
.metric-card small { color: #9aa8bd; }
.metric-card.danger strong { color: #ef4444; }
.metric-card.success strong { color: #16a34a; }
.content-grid { display: grid; grid-template-columns: 360px 1fr; gap: 16px; }
.panel { padding: 18px; background: #fff; border-radius: 12px; border: 1px solid #e6edf7; }
.panel-title { margin-bottom: 16px; font-size: 18px; font-weight: 700; color: #10213f; }
.severity-list { display: flex; flex-direction: column; gap: 18px; }
.severity-row { display: grid; grid-template-columns: 44px 1fr 42px; align-items: center; gap: 12px; color: #344566; }
@media (max-width: 1100px) {
  .metric-grid { grid-template-columns: repeat(2, 1fr); }
  .content-grid { grid-template-columns: 1fr; }
}
</style>
