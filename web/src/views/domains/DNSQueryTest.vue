<script setup>
import { reactive, ref } from 'vue'
import { testDNSResolution } from '../../api/domain'
import { dt } from '../../utils/domain-i18n'

const loading = ref(false)
const result = ref(null)
const form = reactive({ domain: 'grafana.ops.internal', type: 'A' })

async function run() {
  loading.value = true
  try {
    result.value = await testDNSResolution(form)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">{{ dt('queryTestTitle') }}</div>
        <h2>{{ dt('queryTestTitle') }}</h2>
        <p>{{ dt('queryTestDesc') }}</p>
      </div>
    </div>
    <div class="query-workbench">
      <el-form label-position="top">
        <el-form-item :label="dt('domain')" required>
          <el-input v-model="form.domain" size="large" placeholder="grafana.ops.internal" @keyup.enter="run" />
        </el-form-item>
        <el-form-item :label="dt('type')">
          <el-radio-group v-model="form.type"><el-radio-button value="A">A</el-radio-button><el-radio-button value="CNAME">CNAME</el-radio-button></el-radio-group>
        </el-form-item>
        <el-button type="primary" size="large" :loading="loading" @click="run">{{ dt('startResolution') }}</el-button>
      </el-form>
      <div class="query-result" :class="{ 'has-result': result }">
        <template v-if="result">
          <div class="result-head">
            <span class="dns-state" :class="{ 'is-running': result.status === 'success', 'is-error': result.status !== 'success' }">{{ result.status === 'success' ? dt('resolutionSuccess') : dt('resolutionFailure') }}</span>
            <strong>{{ result.responseTimeMs }} ms</strong>
          </div>
          <dl>
            <div><dt>{{ dt('upstreamDns') }}</dt><dd class="domain-mono">{{ result.dnsServer || '—' }}</dd></div>
            <div><dt>{{ dt('responseCode') }}</dt><dd>{{ result.rcode || 'NOERROR' }}</dd></div>
          </dl>
          <div v-for="answer in result.answers" :key="`${answer.type}-${answer.value}`" class="answer-row">
            <el-tag>{{ answer.type }}</el-tag><span class="domain-mono">{{ answer.value }}</span><small>TTL {{ answer.ttl }}</small>
          </div>
          <p v-if="!result.answers?.length">{{ dt('noAnswers') }}</p>
        </template>
        <div v-else class="result-placeholder"><strong>{{ dt('waitingResolution') }}</strong><p>{{ dt('waitingResolutionDesc') }}</p></div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.query-workbench{display:grid;grid-template-columns:minmax(0,460px) minmax(0,1fr);gap:24px}.query-result{min-height:250px;padding:20px;border:1px dashed #bfdbfe;border-radius:14px;background:#f8faff}.result-placeholder{display:grid;place-content:center;height:210px;text-align:center;color:#64748b}.result-placeholder strong{color:#334155}.result-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:20px}.result-head strong{font-variant-numeric:tabular-nums}.query-result dl{display:grid;gap:10px}.query-result dl div,.answer-row{display:flex;align-items:center;justify-content:space-between;gap:12px}.query-result dt{color:#64748b}.query-result dd{margin:0}.answer-row{margin-top:10px;padding:12px;border-radius:9px;background:#fff}.answer-row span{flex:1}@media(max-width:800px){.query-workbench{grid-template-columns:1fr}}
</style>
