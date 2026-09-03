<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { ot } from '../../../utils/ops-i18n'

const props = defineProps({ hostOptions: { type: Array, default: () => [] }, groupOptions: { type: Array, default: () => [] }, hostIds: { type: Array, default: () => [] }, groupIds: { type: Array, default: () => [] } })
const emit = defineEmits(['update:hostIds', 'update:groupIds'])
const dialogVisible = ref(false), tableRef = ref(), tempHostIds = ref([])
const activeGroupId = computed(() => Number(props.groupIds?.[0] || 0))
const flatGroups = computed(() => flattenGroups(props.groupOptions))
const selectedGroupHosts = computed(() => { const groupId = activeGroupId.value; if (!groupId) return []; return props.hostOptions.filter((host) => Number(host.groupId) === groupId || (host.hostGroups || []).some((group) => Number(group.id) === groupId)) })
const selectedHosts = computed(() => { if (activeGroupId.value) return selectedGroupHosts.value; const selectedSet = new Set((props.hostIds || []).map((id) => Number(id))); return props.hostOptions.filter((host) => selectedSet.has(Number(host.id))) })
watch(dialogVisible, async (visible) => { if (!visible) return; tempHostIds.value = [...(props.hostIds || [])]; await nextTick(); const table = tableRef.value; if (!table) return; table.clearSelection(); for (const row of props.hostOptions) if (tempHostIds.value.includes(row.id)) table.toggleRowSelection(row, true) })
function flattenGroups(nodes = [], prefix = '') { return nodes.flatMap((item) => { const label = prefix ? `${prefix} / ${item.name}` : item.name; return [{ label, value: item.id }, ...flattenGroups(item.children || [], label)] }) }
function openHostDialog() { dialogVisible.value = true }
function handleHostSelectionChange(rows) { tempHostIds.value = rows.map((item) => item.id) }
function confirmHosts() { emit('update:hostIds', tempHostIds.value); emit('update:groupIds', []); dialogVisible.value = false }
function clearHosts() { emit('update:hostIds', []) }
function handleGroupChange(value) { emit('update:groupIds', value ? [value] : []); emit('update:hostIds', []) }
function clearGroup() { emit('update:groupIds', []) }
</script>

<template>
  <div class="target-scope">
    <el-form-item :label="ot('targetHosts')"><div class="target-field"><el-button @click="openHostDialog" :disabled="Boolean(activeGroupId)">{{ ot('selectHosts') }}</el-button><el-button v-if="hostIds?.length" link type="danger" @click="clearHosts">{{ ot('clear') }}</el-button></div></el-form-item>
    <el-form-item :label="ot('hostGroup')"><div class="target-field"><el-select :model-value="activeGroupId || undefined" clearable filterable :placeholder="ot('selectHostGroup')" style="width:100%" :disabled="Boolean(hostIds?.length)" @change="handleGroupChange" @clear="clearGroup"><el-option v-for="item in flatGroups" :key="item.value" :label="item.label" :value="item.value" /></el-select></div></el-form-item>
    <el-form-item :label="ot('targetPreview')"><div class="target-tags"><el-empty v-if="!selectedHosts.length" :image-size="56" :description="ot('noTargetHosts')" /><el-tag v-for="item in selectedHosts" :key="item.id" class="target-tag" effect="light">{{ item.hostName }} ({{ item.sshIp || item.privateIp || '-' }})</el-tag></div></el-form-item>
    <el-dialog v-model="dialogVisible" :title="ot('selectTargetHosts')" width="860px"><el-table ref="tableRef" :data="hostOptions" border @selection-change="handleHostSelectionChange"><el-table-column type="selection" width="48" /><el-table-column prop="hostName" :label="ot('hostName')" min-width="180" /><el-table-column label="SSH IP" min-width="140"><template #default="{ row }">{{ row.sshIp || row.privateIp || '-' }}</template></el-table-column><el-table-column :label="ot('hostGroup')" min-width="160"><template #default="{ row }">{{ row.group?.name || (row.hostGroups || []).map((group) => group.name).join(', ') || '-' }}</template></el-table-column><el-table-column prop="sshUser" :label="ot('sshUser')" min-width="100" /></el-table><template #footer><el-button @click="dialogVisible = false">{{ ot('cancel') }}</el-button><el-button type="primary" @click="confirmHosts">{{ ot('confirm') }}</el-button></template></el-dialog>
  </div>
</template>

<style scoped>.target-field{display:flex;align-items:center;gap:12px;width:100%}.target-tags{display:flex;flex-wrap:wrap;gap:8px;min-height:44px;width:100%}.target-tag{max-width:100%}</style>
