<script setup>
import { computed, reactive, watch } from 'vue'
import { ot } from '../../../utils/ops-i18n'

const props = defineProps({ modelValue: { type: String, default: '0 */5 * * * *' } })
const emit = defineEmits(['update:modelValue'])
const fields = reactive({ second: '0', minute: '*/5', hour: '*', day: '*', month: '*', week: '*' })
const fieldDefinitions = computed(() => [
  { key: 'second', label: ot('second'), hint: '0-59' }, { key: 'minute', label: ot('minute'), hint: '0-59' }, { key: 'hour', label: ot('hour'), hint: '0-23' }, { key: 'day', label: ot('day'), hint: '1-31' }, { key: 'month', label: ot('month'), hint: '1-12' }, { key: 'week', label: ot('week'), hint: '0-6' }
])
const presets = computed(() => [
  { label: ot('everyMinute'), value: '0 * * * * *' }, { label: ot('every5Minutes'), value: '0 */5 * * * *' }, { label: ot('every10Minutes'), value: '0 */10 * * * *' }, { label: ot('hourly'), value: '0 0 * * * *' }, { label: ot('daily2am'), value: '0 0 2 * * *' }, { label: ot('weeklyMonday2am'), value: '0 0 2 * * 1' }
])
let syncing = false
const expression = computed(() => fieldDefinitions.value.map((item) => fields[item.key] || '*').join(' '))
const description = computed(() => { const matched = presets.value.find((item) => item.value === expression.value); if (matched) return ot('executes', { label: matched.label }); if (fields.second.startsWith('*/') && fields.minute === '*' && fields.hour === '*') return ot('everySeconds', { count: fields.second.slice(2) }); return ot('cronDescription') })
watch(() => props.modelValue, (value) => { const parts = String(value || '').trim().split(/\s+/).filter(Boolean), normalized = parts.length === 5 ? ['0', ...parts] : parts; if (normalized.length !== 6) return; syncing = true; fieldDefinitions.value.forEach((item, index) => { fields[item.key] = normalized[index] }); syncing = false }, { immediate: true })
watch(expression, (value) => { if (!syncing && value !== props.modelValue) emit('update:modelValue', value) }, { flush: 'sync' })
function applyPreset(value) { emit('update:modelValue', value) }
</script>

<template>
  <div class="cron-editor">
    <div class="cron-toolbar"><span class="cron-toolbar-label">{{ ot('commonSchedules') }}</span><el-button v-for="item in presets" :key="item.value" size="small" :type="expression === item.value ? 'primary' : 'default'" plain @click="applyPreset(item.value)">{{ item.label }}</el-button></div>
    <div class="cron-fields"><label v-for="item in fieldDefinitions" :key="item.key" class="cron-field"><span>{{ item.label }}</span><el-input v-model="fields[item.key]" /><small>{{ item.hint }}</small></label></div>
    <div class="cron-preview"><div><span>{{ ot('generatedExpression') }}</span><code>{{ expression }}</code></div><el-tag type="info" effect="plain">{{ description }}</el-tag></div>
    <p class="cron-help"><code>*</code> {{ ot('cronHelp') }}</p>
  </div>
</template>

<style scoped>
.cron-editor{width:100%;padding:16px;border:1px solid #dce5f4;border-radius:8px;background:#f8faff}.cron-toolbar{display:flex;align-items:center;flex-wrap:wrap;gap:8px;margin-bottom:16px}.cron-toolbar-label{margin-right:4px;color:#52627d;font-size:13px;font-weight:600}.cron-fields{display:grid;grid-template-columns:repeat(6,minmax(88px,1fr));gap:10px}.cron-field{min-width:0}.cron-field>span{display:block;margin-bottom:6px;color:#1d2c48;font-weight:600}.cron-field small{display:block;margin-top:5px;color:#8a98af;text-align:center}.cron-preview{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:14px;padding:10px 12px;border:1px solid #e2e8f3;border-radius:6px;background:#fff}.cron-preview>div{display:flex;align-items:center;gap:12px;min-width:0;color:#71809a}.cron-preview code{color:#245cc5;font-family:Consolas,"Courier New",monospace;font-size:14px;font-weight:700}.cron-help{margin:10px 0 0;color:#7a899f;font-size:12px}.cron-help code{color:#245cc5;font-family:Consolas,"Courier New",monospace}@media(max-width:900px){.cron-fields{grid-template-columns:repeat(3,minmax(88px,1fr))}.cron-preview{align-items:flex-start;flex-direction:column}}
</style>
