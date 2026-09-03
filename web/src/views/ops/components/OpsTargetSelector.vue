<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { ot } from '../../../utils/ops-i18n'

const props = defineProps({ hostOptions: { type: Array, default: () => [] }, groupOptions: { type: Array, default: () => [] }, hostIds: { type: Array, default: () => [] }, groupId: { type: [Number, String], default: undefined } })
const emit = defineEmits(['update:hostIds', 'update:groupId'])
const dialogVisible = ref(false), tableRef = ref(), tempHostIds = ref([]), environmentFilter = ref('')
const flatGroups = computed(() => flattenGroups(props.groupOptions))
const environmentOptions = computed(() => [...new Set(props.hostOptions.map((item) => item.environment).filter(Boolean))])
const filteredHostOptions = computed(() => props.hostOptions.filter((host) => !environmentFilter.value || host.environment === environmentFilter.value))
const selectedGroupHosts = computed(() => { const groupId = Number(props.groupId || 0); if (!groupId) return []; return props.hostOptions.filter((host) => Number(host.groupId) === groupId || (host.hostGroups || []).some((group) => Number(group.id) === groupId)) })
const selectedHosts = computed(() => { if (Number(props.groupId || 0)) return selectedGroupHosts.value; const selectedSet = new Set((props.hostIds || []).map((id) => Number(id))); return props.hostOptions.filter((host) => selectedSet.has(Number(host.id))) })
watch(dialogVisible, async (visible) => { if (!visible) return; tempHostIds.value = [...(props.hostIds || [])]; await nextTick(); const table = tableRef.value; if (!table) return; table.clearSelection(); for (const row of filteredHostOptions.value) if (tempHostIds.value.includes(row.id)) table.toggleRowSelection(row, true) })
watch(environmentFilter, async () => { if (!dialogVisible.value) return; await nextTick(); const table = tableRef.value; if (!table) return; table.clearSelection(); const selectedSet = new Set(tempHostIds.value.map(Number)); for (const row of filteredHostOptions.value) if (selectedSet.has(Number(row.id))) table.toggleRowSelection(row, true) })
function flattenGroups(nodes = [], prefix = '') { return nodes.flatMap((item) => { const label = prefix ? `${prefix} / ${item.name}` : item.name; return [{ label, value: item.id }, ...flattenGroups(item.children || [], label)] }) }
function openHostDialog() { dialogVisible.value = true }
function handleHostSelectionChange(rows) { const visibleIds = new Set(filteredHostOptions.value.map((item) => Number(item.id))), hiddenSelections = tempHostIds.value.filter((id) => !visibleIds.has(Number(id))); tempHostIds.value = [...hiddenSelections, ...rows.map((item) => item.id)] }
function confirmHosts() { emit('update:hostIds', tempHostIds.value); emit('update:groupId', undefined); dialogVisible.value = false }
function clearHosts() { emit('update:hostIds', []) }
function handleGroupChange(value) { emit('update:groupId', value || undefined); emit('update:hostIds', []) }
function clearGroup() { emit('update:groupId', undefined) }
</script>

<template>
  <div class="target-selector">
    <el-form-item :label="ot('targetHosts')"><div class="target-field"><el-button @click="openHostDialog" :disabled="Boolean(groupId)">{{ ot('selectHosts') }}</el-button><el-button v-if="hostIds?.length" link type="danger" @click="clearHosts">{{ ot('clear') }}</el-button></div></el-form-item>
    <el-form-item :label="ot('hostGroup')"><div class="target-field"><el-select :model-value="groupId" clearable filterable :placeholder="ot('selectHostGroup')" class="host-group-select" :disabled="Boolean(hostIds?.length)" @change="handleGroupChange" @clear="clearGroup"><el-option v-for="item in flatGroups" :key="item.value" :label="item.label" :value="item.value" /></el-select></div></el-form-item>
    <el-form-item :label="ot('targetPreview')"><div class="target-tags"><el-empty v-if="!selectedHosts.length" :image-size="60" :description="ot('noTargetHosts')" /><el-tag v-for="item in selectedHosts" :key="item.id" class="target-tag" effect="light">{{ item.hostName }} ({{ item.sshIp || item.privateIp || '-' }})</el-tag></div></el-form-item>
    <el-dialog v-model="dialogVisible" :title="ot('selectTargetHosts')" width="860px">
      <div class="target-filter-bar"><el-select v-model="environmentFilter" clearable :placeholder="ot('allEnvironments')" style="width:180px"><el-option v-for="env in environmentOptions" :key="env" :label="env" :value="env" /></el-select><span>{{ ot('matchedHosts', { count: filteredHostOptions.length }) }}</span></div>
      <el-table ref="tableRef" :data="filteredHostOptions" border @selection-change="handleHostSelectionChange"><el-table-column type="selection" width="48" /><el-table-column prop="hostName" :label="ot('hostName')" min-width="180" /><el-table-column prop="sshIp" label="SSH IP" min-width="140"><template #default="{ row }">{{ row.sshIp || row.privateIp || '-' }}</template></el-table-column><el-table-column prop="group.name" :label="ot('hostGroup')" min-width="140"><template #default="{ row }">{{ row.group?.name || '-' }}</template></el-table-column><el-table-column prop="environment" :label="ot('environment')" width="100" /><el-table-column prop="sshUser" :label="ot('sshUser')" min-width="100" /></el-table>
      <template #footer><el-button @click="dialogVisible = false">{{ ot('cancel') }}</el-button><el-button type="primary" @click="confirmHosts">{{ ot('confirm') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>.target-field{display:flex;align-items:center;gap:12px;width:100%}.target-tags{display:flex;flex-wrap:wrap;gap:8px;min-height:44px;width:100%}.target-tag{max-width:100%}.host-group-select{width:min(500px,100%)}.target-filter-bar{display:flex;align-items:center;gap:12px;margin-bottom:14px;color:#7282a0}</style>
