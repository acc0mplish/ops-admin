<script setup>
import { useRoute, useRouter } from 'vue-router'
import { Coin, CreditCard, DataAnalysis, Opportunity, PieChart, Refresh } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const tabs = [
  { path: '/integration/finops/dashboard', label: '费用看板', icon: DataAnalysis },
  { path: '/integration/finops/accounts', label: '云账号', icon: CreditCard },
  { path: '/integration/finops/breakdown', label: '费用拆分', icon: PieChart },
  { path: '/integration/finops/recommendations', label: '优化建议', icon: Opportunity },
  { path: '/integration/finops/resources', label: '资源拆分', icon: Coin },
  { path: '/integration/finops/sync', label: '账单同步', icon: Refresh }
]
</script>

<template>
  <section class="finops-hero">
    <div>
      <div class="finops-kicker">CLOUD COST ANALYSIS</div>
      <h1>云费用分析</h1>
      <p>统一管理多云账单、成本归因与优化闭环。</p>
    </div>
    <nav class="finops-tabs">
      <button
        v-for="tab in tabs"
        :key="tab.path"
        :class="{ active: route.path === tab.path }"
        @click="router.push(tab.path)"
      >
        <el-icon><component :is="tab.icon" /></el-icon>
        {{ tab.label }}
      </button>
    </nav>
  </section>
</template>
