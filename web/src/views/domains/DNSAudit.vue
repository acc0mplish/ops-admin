<script setup>
import { onMounted, reactive, ref } from 'vue'
import { queryDNSAudit } from '../../api/domain'
import { dt } from '../../utils/domain-i18n'

const loading = ref(false)
const list = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 20 })

async function load() {
  loading.value = true
  try {
    const data = await queryDNSAudit(query)
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">{{ dt('auditTitle') }}</div>
        <h2>{{ dt('auditTitle') }}</h2>
        <p>{{ dt('auditDesc') }}</p>
      </div>
      <el-button :loading="loading" @click="load">{{ dt('refresh') }}</el-button>
    </div>
    <div class="domain-table-wrap">
      <el-table v-loading="loading" :data="list" border>
        <el-table-column prop="createdAt" :label="dt('time')" width="180" />
        <el-table-column prop="username" :label="dt('operator')" width="120" />
        <el-table-column prop="ipAddress" :label="dt('sourceIp')" width="140" />
        <el-table-column prop="action" :label="dt('actionType')" min-width="170" />
        <el-table-column prop="provider" :label="dt('provider')" width="110" />
        <el-table-column prop="zone" :label="dt('zoneName')" min-width="150" />
        <el-table-column prop="domain" :label="dt('domain')" min-width="210">
          <template #default="{ row }"><span class="domain-mono">{{ row.domain }}</span></template>
        </el-table-column>
        <el-table-column prop="recordType" :label="dt('type')" width="80" />
        <el-table-column :label="dt('change')" min-width="260">
          <template #default="{ row }"><span class="domain-mono">{{ row.oldValue || '—' }} → {{ row.newValue || '—' }}</span></template>
        </el-table-column>
        <el-table-column :label="dt('result')" width="110">
          <template #default="{ row }">
            <el-tooltip :content="row.error || dt('operationSuccess')">
              <el-tag :type="row.success ? 'success' : 'danger'">{{ row.success ? dt('success') : dt('failure') }}</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <div class="domain-pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[10,20,50,100]" layout="total, sizes, prev, pager, next" :total="total" @current-change="load" @size-change="query.pageNum=1;load()" />
    </div>
  </section>
</template>
