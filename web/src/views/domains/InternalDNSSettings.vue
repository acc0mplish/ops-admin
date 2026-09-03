<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getInternalDNSSettings, saveInternalDNSSettings } from '../../api/domain'
import { dt } from '../../utils/domain-i18n'

const loading = ref(false)
const saving = ref(false)
const status = ref({})
const form = reactive({ enabled: false, listenAddress: '0.0.0.0', listenPort: 53, upstreams: ['223.5.5.5', '119.29.29.29', '180.76.76.76', '114.114.114.114', '223.112.112.112'], timeoutSeconds: 2 })

async function load() {
  loading.value = true
  try {
    const data = await getInternalDNSSettings()
    Object.assign(form, data.settings || {})
    form.listenPort = 53
    status.value = data.status || {}
  } finally {
    loading.value = false
  }
}

async function save() {
  form.listenPort = 53
  if (form.enabled) {
    await ElMessageBox.confirm(dt('enableConfirm', { address: form.listenAddress }), dt('enableConfirmTitle'), { type: 'warning' })
  }
  saving.value = true
  try {
    await saveInternalDNSSettings(form)
    ElMessage.success(form.enabled ? dt('dnsEnabled') : dt('dnsSaved'))
    await load()
  } catch (error) {
    ElMessage.error(dt('dnsStartFailed', { message: error.message }))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">{{ uiT('dnsRuntimeSettings') }}</div>
        <h2>{{ dt('internalSettingsTitle') }}</h2>
        <p>{{ dt('internalSettingsDesc') }}</p>
      </div>
      <span class="dns-state" :class="{ 'is-running': status.running, 'is-error': status.lastError }">{{ status.running ? 'Running' : status.lastError ? dt('startFailed') : dt('stopped') }}</span>
    </div>
    <el-alert v-if="status.lastError" type="error" :closable="false" show-icon :title="status.lastError" />
    <div class="domain-settings-grid">
      <el-form label-position="top">
        <el-form-item :label="dt('enableInternalDns')"><el-switch v-model="form.enabled" :active-text="dt('enable')" :inactive-text="dt('disable')" /></el-form-item>
        <div class="settings-row">
          <el-form-item :label="dt('listenAddress')"><el-input v-model="form.listenAddress" placeholder="0.0.0.0" /></el-form-item>
          <el-form-item :label="dt('listenPort')"><el-input model-value="53" readonly><template #suffix><span class="fixed-port-label">{{ dt('fixedPort') }}</span></template></el-input></el-form-item>
        </div>
        <el-form-item :label="dt('upstreamDns')">
          <el-select v-model="form.upstreams" multiple filterable allow-create default-first-option style="width:100%" :placeholder="dt('upstreamPlaceholder')">
            <el-option :label="dt('aliyunPublicDns')" value="223.5.5.5" />
            <el-option label="DNSPod 119.29.29.29" value="119.29.29.29" />
            <el-option :label="dt('baiduCloudDns')" value="180.76.76.76" />
            <el-option :label="dt('telecomDns')" value="114.114.114.114" />
            <el-option :label="dt('mobileDns')" value="223.112.112.112" />
          </el-select>
          <div class="domain-form-tip">{{ dt('upstreamHint') }}</div>
        </el-form-item>
        <el-form-item :label="dt('upstreamTimeout')"><el-input-number v-model="form.timeoutSeconds" :min="1" :max="30" /><span style="margin-left:8px;color:#64748b">{{ dt('seconds') }}</span></el-form-item>
        <el-button v-permission="'domains:settings:save'" type="primary" :loading="saving" @click="save">{{ dt('saveDnsSettings') }}</el-button>
      </el-form>
      <aside class="runtime-card">
        <h3>{{ dt('runtimeStatus') }}</h3>
        <dl>
          <div><dt>UDP</dt><dd><span class="dns-state" :class="{ 'is-running': status.udpRunning }">{{ status.udpRunning ? 'Running' : 'Stopped' }}</span></dd></div>
          <div><dt>TCP</dt><dd><span class="dns-state" :class="{ 'is-running': status.tcpRunning }">{{ status.tcpRunning ? 'Running' : 'Stopped' }}</span></dd></div>
          <div><dt>{{ dt('listenAddress') }}</dt><dd class="domain-mono">{{ status.listenAddress || `${form.listenAddress}:${form.listenPort}` }}</dd></div>
          <div><dt>{{ dt('zoneRecords') }}</dt><dd>{{ status.zoneCount || 0 }} / {{ status.recordCount || 0 }}</dd></div>
          <div><dt>{{ dt('cacheRefresh') }}</dt><dd>{{ status.lastCacheRefresh || '—' }}</dd></div>
        </dl>
        <el-alert type="info" :closable="false" :title="dt('capabilityHint')" />
      </aside>
    </div>
  </section>
</template>

<style scoped>
.domain-settings-grid{display:grid;grid-template-columns:minmax(0,1fr) 360px;gap:24px}.settings-row{display:grid;grid-template-columns:1fr 180px;gap:12px}.fixed-port-label{color:#64748b;font-size:12px}.runtime-card{padding:18px;border:1px solid #e4ecfc;border-radius:12px;background:#f8faff}.runtime-card h3{margin:0 0 14px}.runtime-card dl{display:grid;gap:12px;margin:0 0 18px}.runtime-card dl div{display:flex;justify-content:space-between;gap:12px}.runtime-card dt{color:#64748b}.runtime-card dd{margin:0;color:#0f172a;font-weight:650;text-align:right}@media(max-width:900px){.domain-settings-grid{grid-template-columns:1fr}}@media(max-width:520px){.settings-row{grid-template-columns:1fr}}
</style>
