<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { queryDNSAccountOptions, queryPublicDomains, syncPublicDomains } from '../../api/domain'
import { dt } from '../../utils/domain-i18n'

const router = useRouter()
const loading = ref(false)
const syncing = ref(false)
const tableData = ref([])
const accounts = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', provider: '', accountId: '' })

async function load() {
  loading.value = true
  try {
    const data = await queryPublicDomains(query)
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadAccounts() {
  accounts.value = await queryDNSAccountOptions()
}

async function sync() {
  syncing.value = true
  try {
    const data = await syncPublicDomains(Number(query.accountId) || 0)
    ElMessage.success(dt('syncedCount', { count: data.synced }))
    await load()
  } finally {
    syncing.value = false
  }
}

function records(row) {
  router.push(`/domains/public/${row.accountId}/${encodeURIComponent(row.domain)}/records`)
}

onMounted(async () => {
  await loadAccounts()
  await load()
})
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">PUBLIC DNS INVENTORY</div>
        <h2>{{ dt('publicDomainsTitle') }}</h2>
        <p>{{ dt('publicDomainsDesc') }}</p>
      </div>
      <el-button v-permission="'domains:public:sync'" type="primary" :loading="syncing" @click="sync">{{ dt('refreshFromCloud') }}</el-button>
    </div>

    <div class="domain-stat-grid">
      <div class="domain-stat"><div class="domain-stat__label">{{ dt('syncedDomains') }}</div><div class="domain-stat__value">{{ total }}</div></div>
      <div class="domain-stat"><div class="domain-stat__label">{{ dt('enabledAccounts') }}</div><div class="domain-stat__value">{{ accounts.length }}</div></div>
      <div class="domain-stat"><div class="domain-stat__label">{{ dt('supportedProviders') }}</div><div class="domain-stat__value">2</div></div>
      <div class="domain-stat"><div class="domain-stat__label">{{ dt('dataSource') }}</div><div class="domain-stat__value" style="font-size:17px">{{ dt('officialApi') }}</div></div>
    </div>

    <div class="domain-toolbar" role="search">
      <div class="domain-toolbar__filters">
        <el-input v-model="query.keyword" clearable :placeholder="dt('searchDomain')" style="width:220px" @keyup.enter="load" />
        <el-select v-model="query.accountId" clearable :placeholder="dt('cloudAccount')" style="width:180px"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select>
        <el-select v-model="query.provider" clearable :placeholder="dt('provider')" style="width:160px"><el-option :label="dt('aliyunDns')" value="aliyun" /><el-option :label="dt('tencentDnspod')" value="tencent" /></el-select>
        <el-button @click="load">{{ dt('search') }}</el-button>
      </div>
      <div class="domain-toolbar__actions"><el-button v-permission="'domains:account:list'" @click="router.push('/domains/public/accounts')">{{ dt('manageDnsAccounts') }}</el-button></div>
    </div>

    <div class="domain-table-wrap">
      <el-table v-loading="loading" :data="tableData" border @row-dblclick="records">
        <el-table-column prop="domain" :label="dt('domain')" min-width="240"><template #default="{ row }"><el-button link type="primary" class="domain-mono" @click="records(row)">{{ row.domain }}</el-button></template></el-table-column>
        <el-table-column :label="dt('provider')" width="150"><template #default="{ row }">{{ row.provider === 'aliyun' ? dt('aliyunDns') : dt('tencentDnspod') }}</template></el-table-column>
        <el-table-column prop="accountName" :label="dt('account')" min-width="160" />
        <el-table-column prop="recordCount" :label="dt('recordCount')" width="110" />
        <el-table-column :label="dt('status')" width="110"><template #default="{ row }"><el-tag type="success">{{ row.status || dt('healthy') }}</el-tag></template></el-table-column>
        <el-table-column prop="syncedAt" :label="dt('lastSync')" width="180" />
        <el-table-column :label="dt('actions')" width="120" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="records(row)">{{ dt('dnsRecords') }}</el-button></template></el-table-column>
        <template #empty><div class="domain-empty"><strong>{{ dt('noSyncedDomains') }}</strong><p>{{ dt('noSyncedDomainsDesc') }}</p><el-button type="primary" @click="router.push('/domains/public/accounts')">{{ dt('configureDnsAccount') }}</el-button></div></template>
      </el-table>
    </div>
    <div class="domain-pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :total="total" @current-change="load" @size-change="load" /></div>
  </section>
</template>
