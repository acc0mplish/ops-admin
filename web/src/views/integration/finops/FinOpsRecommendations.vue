<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import FinOpsHeader from './FinOpsHeader.vue'
import { generateFinOpsRecommendations, queryFinOpsRecommendations, updateFinOpsRecommendation } from '../../../api/integration'
import './finops.css'

const loading = ref(false)
const generating = ref(false)
const status = ref('')
const rows = ref([])
const statuses = { open: '待处理', accepted: '已采纳', ignored: '已忽略', done: '已完成' }
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
async function load() { loading.value = true; try { rows.value = await queryFinOpsRecommendations(status.value ? { status: status.value } : {}) || [] } finally { loading.value = false } }
async function generate() { generating.value = true; try { const result = await generateFinOpsRecommendations(); ElMessage.success(`已生成 ${result?.count || 0} 条优化建议`); await load() } finally { generating.value = false } }
async function changeStatus(row, value) { await updateFinOpsRecommendation({ id: row.id, status: value }); ElMessage.success('建议状态已更新'); await load() }
onMounted(load)
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section class="finops-panel" v-loading="loading">
      <div class="finops-head">
        <div><h2>优化建议</h2><p>基于账单资源与用量结构识别闲置资源、规格偏高和成本异常。</p></div>
        <div class="finops-actions"><el-select v-model="status" clearable placeholder="全部状态" style="width:130px" @change="load"><el-option v-for="(label, value) in statuses" :key="value" :label="label" :value="value" /></el-select><el-button :loading="generating" type="primary" @click="generate">生成建议</el-button></div>
      </div>
      <el-table :data="rows" stripe>
        <el-table-column prop="title" label="优化项" min-width="220" />
        <el-table-column prop="provider" label="云厂商" width="110" />
        <el-table-column prop="accountName" label="云账号" min-width="150" />
        <el-table-column prop="resourceName" label="资源" min-width="180" />
        <el-table-column label="预计月节省" width="135"><template #default="{ row }"><b class="finops-money">¥ {{ money(row.saving) }}</b></template></el-table-column>
        <el-table-column prop="description" label="建议说明" min-width="280" show-overflow-tooltip />
        <el-table-column label="状态" width="120"><template #default="{ row }"><el-tag :type="row.status === 'open' ? 'warning' : row.status === 'done' ? 'success' : 'info'">{{ statuses[row.status] || row.status }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="155" fixed="right"><template #default="{ row }"><el-dropdown @command="value => changeStatus(row, value)"><el-button link type="primary">处理</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="accepted">采纳</el-dropdown-item><el-dropdown-item command="ignored">忽略</el-dropdown-item><el-dropdown-item command="done">标记完成</el-dropdown-item></el-dropdown-menu></template></el-dropdown></template></el-table-column>
      </el-table>
      <el-empty v-if="!rows.length" description="暂无优化建议，可先同步账单后生成建议" />
    </section>
  </div>
</template>
