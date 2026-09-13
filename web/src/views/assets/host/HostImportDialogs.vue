<script setup>
// I5-J (i5-plan §3.8·CW-4) — Host.vue 1,070행 분할 자식(가져오기·클라우드 동기화·
// 일괄 자격증명 다이얼로그 + More 메뉴 일괄 작업 — 드롭다운 3개 명령 전부가 본
// 자식 소속). 원본 좌표: 템플릿 :760-860·filteredCloudAccounts :88-90·
// resetImportForm :111-116·resetCloudSyncForm :118-133·
// resetBatchCredentialForm :135-137·openImportDialog :194-197·
// openCloudSyncDialog :199-202·openBatchCredentialDialog :204-211·
// selectedIds :315-317·submitBatchCredential :357-379·
// handleTemplateDownload :381-390·handleFileChange :392-394·
// clearImportFile :396-398·submitImport :400-418·submitCloudSync :420-461·
// handleBatchSync :319-334·handleBatchDelete :336-355·handleMoreCommand
// :506-510·CSS .credential-line/.dialog-tip :986-1008·:1010-1014.
// `page` 주입(§5 #11 승계) — reactive 래핑으로 ref/reactive가 언랩된다.
import { computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  batchDeleteAssetHosts,
  batchReplaceAssetHostCredential,
  batchSyncAssetHosts,
  downloadAssetHostTemplate,
  importAssetHosts,
  removeAssetHostsFromGroup,
  syncAssetHostsFromCloud
} from '../../../api/asset'
import { at } from '../../../utils/asset-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :88-90 — 클라우드 동기화 다이얼로그 전용 computed(자식 귀속)
const filteredCloudAccounts = computed(() =>
  props.page.cloudAccountOptions.filter((item) => (item.provider || '').toLowerCase() === props.page.cloudSyncForm.provider)
)

// 원본 :111-116
function resetImportForm() {
  Object.assign(props.page.importForm, {
    groupId: undefined,
    file: null
  })
}

// 원본 :118-133
function resetCloudSyncForm() {
  Object.assign(props.page.cloudSyncForm, {
    groupId: undefined,
    provider: 'tencent',
    useExistingAccount: true,
    cloudAccountId: undefined,
	credentialId: undefined,
	connectionMode: 'direct',
	gatewayId: undefined,
	environment: '',
    accessKey: '',
    secretKey: '',
    saveAccount: false,
    accountName: ''
  })
}

// 원본 :135-137
function resetBatchCredentialForm() {
  props.page.batchCredentialForm.credentialId = undefined
}

// 원본 :194-197
function openImportDialog() {
  resetImportForm()
  props.page.importDialogVisible = true
}

// 원본 :199-202
function openCloudSyncDialog() {
  resetCloudSyncForm()
  props.page.cloudSyncDialogVisible = true
}

// 원본 :204-211
function openBatchCredentialDialog() {
  if (!props.page.selectedRows.length) {
    ElMessage.warning(at('selectHostFirst'))
    return
  }
  resetBatchCredentialForm()
  props.page.batchCredentialDialogVisible = true
}

// 원본 :357-379
async function submitBatchCredential() {
  const ids = selectedIds()
  if (!ids.length) {
    ElMessage.warning(at('selectHostFirst'))
    return
  }
  if (!props.page.batchCredentialForm.credentialId) {
    ElMessage.warning(at('selectCredentialWarning'))
    return
  }
  props.page.batchCredentialSubmitting = true
  try {
    await batchReplaceAssetHostCredential({
      ids,
      credentialId: props.page.batchCredentialForm.credentialId
    })
    ElMessage.success(at('credentialReplaced'))
    props.page.batchCredentialDialogVisible = false
    await props.page.loadData()
  } finally {
    props.page.batchCredentialSubmitting = false
  }
}

// 원본 :381-390
async function handleTemplateDownload() {
  const response = await downloadAssetHostTemplate()
  const blob = new Blob([response.data], { type: response.headers['content-type'] })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'asset-host-template.xlsx'
  link.click()
  window.URL.revokeObjectURL(url)
}

// 원본 :392-394
function handleFileChange(uploadFile) {
  props.page.importForm.file = uploadFile.raw || null
}

// 원본 :396-398
function clearImportFile() {
  props.page.importForm.file = null
}

// 원본 :400-418
async function submitImport() {
  const importForm = props.page.importForm
  if (!importForm.groupId || !importForm.file) {
    ElMessage.warning(at('selectGroupAndExcel'))
    return
  }
  props.page.importSubmitting = true
  try {
    const formData = new FormData()
    formData.append('groupId', importForm.groupId)
    formData.append('file', importForm.file)
    const data = await importAssetHosts(formData)
    const failedPreview = (data.failedHosts || []).slice(0, 3).join('; ')
    ElMessage.success(at('importDone', { success: data.success, fail: data.fail, failedSuffix: failedPreview ? `(${failedPreview})` : '' }))
    props.page.importDialogVisible = false
    await props.page.loadData()
  } finally {
    props.page.importSubmitting = false
  }
}

// 원본 :420-461
async function submitCloudSync() {
  const cloudSyncForm = props.page.cloudSyncForm
  if (!cloudSyncForm.groupId || !cloudSyncForm.provider) {
    ElMessage.warning(at('selectGroupAndProvider'))
    return
  }
  if (cloudSyncForm.useExistingAccount && !cloudSyncForm.cloudAccountId) {
    ElMessage.warning(at('selectExistingCloudAccount'))
    return
  }
	if (!cloudSyncForm.credentialId) {
		ElMessage.warning(at('selectCredentialWarning'))
		return
	}
  if (!cloudSyncForm.environment) {
		ElMessage.warning(at('selectEnvironmentWarning'))
		return
	}
	if (cloudSyncForm.connectionMode === 'gateway' && !cloudSyncForm.gatewayId) {
		ElMessage.warning(at('selectGatewayWarning'))
		return
	}
  if (!cloudSyncForm.useExistingAccount && (!cloudSyncForm.accessKey || !cloudSyncForm.secretKey)) {
    ElMessage.warning(at('enterAccessKeys'))
    return
  }
  props.page.cloudSyncSubmitting = true
  try {
    const data = await syncAssetHostsFromCloud(cloudSyncForm)
    const addedNames = (data.addedHosts || []).slice(0, 3).join(', ')
    const updatedNames = (data.updatedHosts || []).slice(0, 3).join(', ')
    const details = [
      addedNames ? at('cloudAddedNames', { names: addedNames }) : '',
      updatedNames ? at('cloudUpdatedNames', { names: updatedNames }) : '',
      Object.keys(data.regionCounts || {}).length ? at('regionCountsSummary', { counts: Object.entries(data.regionCounts).map(([region, count]) => at('regionCountItem', { region, count })).join(', ') }) : ''
    ].filter(Boolean).join('; ')
    ElMessage.success(at('cloudSyncResult', { total: data.total || 0, added: data.added, updated: data.updated, skipped: data.skipped, detailsSuffix: details ? ` (${details})` : '' }))
    props.page.cloudSyncDialogVisible = false
    await props.page.loadData()
  } finally {
    props.page.cloudSyncSubmitting = false
  }
}

// 원본 :315-317 — 부모 handleBatchSync·handleBatchDelete와 본 자식
// submitBatchCredential이 공유했던 헬퍼(자식 귀속 후 page.selectedRows 사용)
function selectedIds() {
  return props.page.selectedRows.map((item) => item.id)
}

// 원본 :319-334
async function handleBatchSync() {
	if (props.page.batchSyncSubmitting) return
  const ids = selectedIds()
  if (!ids.length) {
    ElMessage.warning(at('selectHostFirst'))
    return
  }
	props.page.batchSyncSubmitting = true
	try {
		const data = await batchSyncAssetHosts(ids)
		ElMessage.success(at('batchSyncDone', { success: data.success, fail: data.fail }))
		await props.page.loadData()
	} finally {
		props.page.batchSyncSubmitting = false
	}
}

// 원본 :336-355
async function handleBatchDelete() {
  const ids = selectedIds()
  if (!ids.length) {
    ElMessage.warning(at('selectHostFirst'))
    return
  }
  if (props.page.isGroupView) {
    await ElMessageBox.confirm(at('batchRemoveFromGroupConfirm', { count: ids.length }), at('notice'), { type: 'warning' })
    await removeAssetHostsFromGroup({ groupId: props.page.query.groupId, hostIds: ids })
    ElMessage.success(at('batchRemovedFromGroup'))
    props.page.selectedRows = []
    await props.page.loadData()
    return
  }
  await ElMessageBox.confirm(at('batchDeleteConfirm', { count: ids.length }), at('notice'), { type: 'warning' })
  await batchDeleteAssetHosts(ids)
  ElMessage.success(at('batchDeleted'))
  props.page.selectedRows = []
  await props.page.loadData()
}

// 원본 :506-510 — More 드롭다운 3개 명령의 라우터(전부 본 자식 소속)
function handleMoreCommand(command) {
  if (command === 'batch-sync') handleBatchSync()
  if (command === 'batch-delete') handleBatchDelete()
  if (command === 'batch-credential') openBatchCredentialDialog()
}

defineExpose({
  openImportDialog,
  openCloudSyncDialog,
  openBatchCredentialDialog,
  handleMoreCommand
})
</script>

<template>
  <el-dialog v-model="page.importDialogVisible" title="Excel Import" width="520px">
    <el-form label-width="92px">
      <el-form-item label="Template Download">
        <el-button type="primary" @click="handleTemplateDownload">{{ at('downloadTemplate') }}</el-button>
      </el-form-item>
      <el-form-item :label="at('selectGroupLabel')">
        <el-select v-model="page.importForm.groupId" filterable :placeholder="at('selectGroupLabel')" style="width: 100%">
          <el-option v-for="item in page.groupOptions" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </el-form-item>
      <el-form-item label="Excel Upload">
        <el-upload
          :auto-upload="false"
          :show-file-list="true"
          :limit="1"
          accept=".xlsx,.xls"
          :on-change="handleFileChange"
          :on-remove="clearImportFile"
        >
          <el-button type="primary">{{ at('chooseFile') }}</el-button>
        </el-upload>
      </el-form-item>
      <div class="dialog-tip">{{ at('excelImportTip') }}</div>
    </el-form>
    <template #footer>
      <el-button @click="page.importDialogVisible = false">{{ at('cancel') }}</el-button>
      <el-button type="primary" :loading="page.importSubmitting" @click="submitImport">Host Import</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.cloudSyncDialogVisible" :title="at('syncCloudHosts')" width="620px">
    <el-form label-width="108px">
      <el-form-item label="Target Group">
        <el-select v-model="page.cloudSyncForm.groupId" filterable :placeholder="at('selectSyncGroup')" style="width: 100%">
          <el-option v-for="item in page.groupOptions" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </el-form-item>
      <el-form-item label="Cloud Provider">
        <el-select v-model="page.cloudSyncForm.provider" :placeholder="at('selectCloudProvider')" style="width: 100%">
          <el-option label="Tencent Cloud" value="tencent" />
          <el-option label="Alibaba Cloud" value="aliyun" />
        </el-select>
      </el-form-item>
      <el-form-item label="Cloud Account">
        <el-select v-model="page.cloudSyncForm.cloudAccountId" filterable clearable :placeholder="at('selectCloudAccount')" style="width: 100%">
          <el-option
            v-for="item in filteredCloudAccounts"
            :key="item.id"
            :value="item.id"
            :label="`${item.name} (${(item.regions?.length ? item.regions : (item.region ? item.region.split(/[,，;；\s]+/).filter(Boolean) : [])).join(', ')})`"
          />
        </el-select>
      </el-form-item>
		<el-form-item :label="at('authCredentialLabel')" required>
		  <div class="credential-line">
			<el-select v-model="page.cloudSyncForm.credentialId" clearable filterable :placeholder="at('selectCredential')">
			  <el-option v-for="item in page.credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
			</el-select>
			<el-button color="#f59e0b" @click="page.goCredential">{{ at('createCredential') }}</el-button>
		  </div>
		</el-form-item>
      <el-form-item :label="at('connectionModeLabel')">
		  <el-radio-group v-model="page.cloudSyncForm.connectionMode">
			<el-radio value="direct">{{ at('directConnection') }}</el-radio>
			<el-radio value="gateway">{{ at('viaGateway') }}</el-radio>
		  </el-radio-group>
		</el-form-item>
		<el-form-item v-if="page.cloudSyncForm.connectionMode === 'gateway'" :label="at('accessGatewayLabel')">
		  <el-select v-model="page.cloudSyncForm.gatewayId" filterable :placeholder="at('selectGateway')" style="width: 100%">
			<el-option v-for="item in page.gatewayOptions" :key="item.id" :value="item.id" :label="item.name" />
		  </el-select>
		</el-form-item>
		<el-form-item :label="at('environmentLabel')">
		  <el-select v-model="page.cloudSyncForm.environment" filterable :placeholder="at('selectEnvironment')" :loading="page.environmentLoading" style="width: 100%">
			<el-option v-for="item in page.environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
		  </el-select>
		</el-form-item>
      <div class="dialog-tip">{{ at('cloudSyncTip') }}</div>
    </el-form>
    <template #footer>
      <el-button @click="page.cloudSyncDialogVisible = false">{{ at('cancel') }}</el-button>
      <el-button type="primary" :loading="page.cloudSyncSubmitting" @click="submitCloudSync">{{ at('startSync') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.batchCredentialDialogVisible" :title="at('batchReplaceCredentialTitle')" width="480px">
    <el-form label-width="96px">
      <el-form-item :label="at('selectedHostsLabel')">
        <span>{{ at('hostCountUnit', { count: page.selectedRows.length }) }}</span>
      </el-form-item>
      <el-form-item :label="at('authCredentialLabel')">
        <el-select v-model="page.batchCredentialForm.credentialId" filterable :placeholder="at('selectCredential')" style="width: 100%">
          <el-option v-for="item in page.credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="page.batchCredentialDialogVisible = false">{{ at('cancel') }}</el-button>
      <el-button type="primary" :loading="page.batchCredentialSubmitting" @click="submitBatchCredential">{{ at('confirmReplace') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
/* 원본 :986-992·:1006-1008 — .credential-line은 HostFormDialog 자식과 공유(복사 1건) */
.credential-line {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.credential-line .el-select {
  flex: 1;
}

/* 원본 :1010-1014 */
.dialog-tip {
  color: #7c87a6;
  font-size: 13px;
  line-height: 1.7;
}
</style>
