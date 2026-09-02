<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { queryAssetServiceWorkloadLogs, queryAssetServiceWorkloadRuntime } from '../../api/asset'
import { at } from '../../utils/asset-i18n'

const props = defineProps({ serviceId: { type: Number, default: 0 }, workloadType: String, workloadName: String, podName: String, inline: Boolean })
const route = useRoute()
const loading = ref(false)
const logLoading = ref(false)
const runtime = ref({})
const logs = ref('')
const logLines = ref(200)
const selectedPod = ref(props.podName || route.query.podName || '')
const logCountLabel = (count) => at('recentLines', { count })
const params = computed(() => ({ serviceId: props.serviceId || Number(route.query.serviceId), workloadType: props.workloadType || route.query.workloadType, workloadName: props.workloadName || route.query.workloadName }))
const hasWorkloadContext = computed(() => Boolean(params.value.serviceId && params.value.workloadType && params.value.workloadName))

async function loadRuntime() {
  if (!hasWorkloadContext.value) {
    runtime.value = {}
    logs.value = ''
    return
  }
  loading.value = true
  try {
    runtime.value = await queryAssetServiceWorkloadRuntime(params.value)
    if (!selectedPod.value) selectedPod.value = runtime.value?.pods?.[0]?.name || ''
    if (selectedPod.value) await loadLogs()
  } finally {
    loading.value = false
  }
}

async function loadLogs() {
  if (!selectedPod.value) return
  logLoading.value = true
  try {
    const data = await queryAssetServiceWorkloadLogs({ ...params.value, podName: selectedPod.value, tailLines: logLines.value })
    logs.value = data.content || at('noLogContent')
  } finally {
    logLoading.value = false
  }
}

onMounted(loadRuntime)
</script>

<template>
  <div class="logs-page" v-loading="loading">
    <el-empty v-if="!hasWorkloadContext" :description="at('selectServiceWorkloadForLogs')" />
    <section v-else class="logs-card">
      <header>
        <div><h2>{{ at('podLogs') }}</h2><p>{{ at('podLogsHint') }}</p></div>
        <div class="controls">
          <el-select v-model="selectedPod" style="width:300px" placeholder="Pod" @change="loadLogs"><el-option v-for="pod in runtime.pods || []" :key="pod.name" :label="pod.name" :value="pod.name" /></el-select>
          <el-select v-model="logLines" style="width:142px" @change="loadLogs"><el-option v-for="item in [100, 200, 500, 800, 1000]" :key="item" :label="logCountLabel(item)" :value="item" /></el-select>
          <el-button :icon="Refresh" @click="loadLogs">{{ at('refreshLogs') }}</el-button>
        </div>
      </header>
      <pre v-loading="logLoading">{{ logs || at('selectPodForLogs') }}</pre>
    </section>
  </div>
</template>

<style scoped>
.logs-page { min-height: 100%; padding: 18px 22px 22px; background: #f4f7fc; }.logs-card { overflow: hidden; border: 1px solid #e0e8f5; border-radius: 16px; background: #fff; }.logs-card header { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 22px; background: #f8faff; }.logs-card h2 { margin: 0; color: #142e57; font-size: 19px; }.logs-card p { margin: 5px 0 0; color: #7485a0; font-size: 13px; }.controls { display: flex; align-items: center; gap: 10px; }.logs-card pre { min-height: 600px; max-height: calc(100vh - 220px); margin: 0; overflow: auto; padding: 20px; background: #101c33; color: #dbe8ff; white-space: pre-wrap; word-break: break-all; font: 13px/1.7 Consolas, Monaco, monospace; }@media(max-width:800px){.logs-card header{align-items:flex-start;flex-direction:column}.controls{width:100%;flex-wrap:wrap}}
</style>
