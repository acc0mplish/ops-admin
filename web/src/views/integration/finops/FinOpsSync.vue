<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import FinOpsHeader from './FinOpsHeader.vue'
import { importFinOpsCosts, queryFinOpsAccounts, queryFinOpsSyncLogs, triggerFinOpsSync } from '../../../api/integration'
import './finops.css'

const accounts = ref([])
const logs = ref([])
const accountId = ref('')
const importing = ref(false)
const syncing = ref(false)
const rawRecords = ref('[]')

async function loadAccounts() {
  accounts.value = await queryFinOpsAccounts() || []
}

async function loadLogs() {
  logs.value = await queryFinOpsSyncLogs(accountId.value ? { accountId: accountId.value } : {}) || []
}

async function trigger() {
  if (!accountId.value) return ElMessage.warning('请先选择云账号')
  syncing.value = true
  try {
    await triggerFinOpsSync(accountId.value)
    ElMessage.success('已触发账单同步')
    await loadLogs()
  } finally {
    syncing.value = false
  }
}

async function importRecords() {
  if (!accountId.value) return ElMessage.warning('请先选择云账号')
  let records
  try {
    records = JSON.parse(rawRecords.value)
  } catch (_) {
    return ElMessage.error('账单 JSON 格式无效')
  }
  if (!Array.isArray(records)) return ElMessage.warning('账单内容必须是 JSON 数组')
  importing.value = true
  try {
    const result = await importFinOpsCosts({ accountId: accountId.value, records })
    ElMessage.success(`已导入 ${result?.recordCount || 0} 条账单`)
    await loadLogs()
  } finally {
    importing.value = false
  }
}

onMounted(async () => {
  await loadAccounts()
  await loadLogs()
})
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section class="finops-panel">
      <div class="finops-head">
        <div>
          <h2>账单同步</h2>
          <p>支持云厂商账单接口自动拉取，也可导入标准 JSON 账单。</p>
        </div>
        <div class="finops-actions">
          <el-select v-model="accountId" clearable placeholder="选择云账号" style="width: 220px" @change="loadLogs">
            <el-option v-for="item in accounts" :key="item.id" :label="`${item.name} · ${item.provider}`" :value="item.id" />
          </el-select>
          <el-button :loading="syncing" type="primary" @click="trigger">立即同步</el-button>
        </div>
      </div>
      <el-alert type="info" :closable="false" show-icon title="自动同步使用云账号中配置的账单接口与同步频率；未配置接口时可使用下方 JSON 导入。" />
    </section>

    <section class="finops-panel">
      <div class="finops-head">
        <div>
          <h2>手动导入账单</h2>
          <p>账单字段支持 billingDate、service、resourceId、resourceName、resourceType、region、amount、currency、tags。</p>
        </div>
        <el-button :loading="importing" type="primary" @click="importRecords">导入账单</el-button>
      </div>
      <textarea v-model="rawRecords" class="finops-import" spellcheck="false" />
    </section>

    <section class="finops-panel">
      <div class="finops-head">
        <div><h2>同步历史</h2><p>最近 200 条账单同步执行记录。</p></div>
        <el-button @click="loadLogs">刷新记录</el-button>
      </div>
      <el-table :data="logs" stripe>
        <el-table-column prop="accountName" label="云账号" min-width="160" />
        <el-table-column prop="trigger" label="触发方式" width="120" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : row.status === 'running' ? 'warning' : 'danger'">
              {{ row.status === 'success' ? '成功' : row.status === 'running' ? '执行中' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="recordCount" label="账单条目" width="110" />
        <el-table-column prop="message" label="执行信息" min-width="260" show-overflow-tooltip />
        <el-table-column prop="createdAt" label="执行时间" width="180" />
      </el-table>
      <el-empty v-if="!logs.length" description="暂无账单同步记录" />
    </section>
  </div>
</template>
