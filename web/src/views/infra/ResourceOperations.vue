<template>
  <div class="resource-operations">
    <el-alert v-if="!loading && !operations.length" :title="inft('noOperations')" type="info" :closable="false" />
    <el-table v-else v-loading="loading" :data="operations" size="small">
      <el-table-column prop="name" :label="inft('operationCol')" min-width="170" />
      <el-table-column prop="version" :label="inft('versionCol')" width="90" />
      <el-table-column :label="inft('mutatingCol')" width="90">
        <template #default="{ row }">{{ row.mutating ? inft('builtinYes') : inft('builtinNo') }}</template>
      </el-table-column>
      <el-table-column prop="riskLevel" :label="inft('riskCol')" width="110" />
      <el-table-column :label="inft('requiresApprovalCol')" width="110">
        <template #default="{ row }">{{ row.requiresApproval ? inft('builtinYes') : inft('builtinNo') }}</template>
      </el-table-column>
      <el-table-column prop="permission" :label="inft('permissionCol')" min-width="190" show-overflow-tooltip />
      <el-table-column width="90" align="right">
        <template #default="{ row }">
          <el-button v-permission="row.permission" link type="primary" @click="openPlan(row)">{{ inft('runOp') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="planVisible" :title="inft('planDialogTitle')" width="520px" @closed="resetPlan">
      <el-descriptions v-if="plan" :column="1" border size="small">
        <el-descriptions-item :label="inft('operationCol')">{{ plan.operation }}</el-descriptions-item>
        <el-descriptions-item :label="inft('versionCol')">{{ plan.version }}</el-descriptions-item>
        <el-descriptions-item :label="inft('riskCol')">{{ plan.riskLevel }}</el-descriptions-item>
        <el-descriptions-item :label="inft('requiresApprovalCol')">{{ plan.requiresApproval ? inft('builtinYes') : inft('builtinNo') }}</el-descriptions-item>
        <el-descriptions-item :label="inft('permissionCol')">{{ plan.permission }}</el-descriptions-item>
        <el-descriptions-item :label="inft('planPolicyVersion')">{{ plan.policyVersion }}</el-descriptions-item>
        <el-descriptions-item :label="inft('planResourceRevision')">{{ plan.resourceRevision ?? '-' }}</el-descriptions-item>
        <el-descriptions-item :label="inft('planRestartedAt')">{{ plan.restartedAt }}</el-descriptions-item>
      </el-descriptions>
      <p class="plan-note">{{ inft('planExecuteNote') }}</p>
      <template #footer>
        <el-button @click="planVisible = false">{{ inft('close') }}</el-button>
        <el-button v-permission="plan?.permission" type="primary" :loading="executing" @click="confirmExecute">{{ inft('runOp') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { executeInfraOperation, listInfraResourceOperations, planInfraOperation } from '../../api/infra'
import { inft } from '../../utils/infra-i18n'

const props = defineProps({
  resource: { type: Object, required: true }
})

const router = useRouter()
const loading = ref(false)
const operations = ref([])
const planVisible = ref(false)
const plan = ref(null)
const executing = ref(false)

async function loadOperations() {
  loading.value = true
  operations.value = []
  try {
    const response = await listInfraResourceOperations(props.resource.uid)
    operations.value = response?.data?.items || []
  } catch (error) {
    operations.value = []
  } finally {
    loading.value = false
  }
}

async function openPlan(row) {
  try {
    const response = await planInfraOperation(props.resource.uid, row.name)
    plan.value = response?.data || null
    planVisible.value = true
  } catch (error) {
    // the interceptor already surfaced the failure reason
  }
}

function resetPlan() {
  plan.value = null
  executing.value = false
}

async function confirmExecute() {
  if (!plan.value) return
  executing.value = true
  try {
    const payload = { restartedAt: plan.value.restartedAt }
    if (plan.value.resourceRevision !== null && plan.value.resourceRevision !== undefined) {
      payload.resourceRevision = plan.value.resourceRevision
    }
    const response = await executeInfraOperation(props.resource.uid, plan.value.operation, payload, crypto.randomUUID())
    const taskUid = response?.data?.task?.uid
    planVisible.value = false
    ElMessage.success(inft('executeSuccess', { uid: taskUid }))
    router.push({ path: '/infra/tasks', query: taskUid ? { uid: taskUid } : {} })
  } catch (error) {
    // the interceptor already surfaced the failure reason
  } finally {
    executing.value = false
  }
}

watch(() => props.resource?.uid, (uid) => { if (uid) loadOperations() }, { immediate: true })
</script>

<style scoped>
.plan-note { margin: 12px 0 0; color: var(--el-text-color-secondary); font-size: 13px; }
</style>
