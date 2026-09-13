<script setup>
import { at } from '../../../utils/asset-i18n'

// I5-H — template/CSS child extracted from views/assets/DatabaseWorkbench.vue
// (i5-plan §3.8 — CW-WB: database/ 자식). Identifiers are rewired to the
// page bundle passed by the parent (:page 주입 패턴 승계 — §5 #11). All state
// and handlers remain owned by the parent view and its composables.
const props = defineProps({ page: { type: Object, required: true } })
</script>

<template>
          <el-dialog v-model="page.redisKeyDialogVisible" :title="page.redisKeyDialogMode === 'create' ? at('redisKeyCreateTitle') : at('redisKeyEditTitle')" width="720px">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        :title="at('redisWriteAuditAlert')"
        class="redis-key-alert"
      />
      <el-form label-width="112px" class="redis-key-form">
        <el-form-item :label="at('keyName')" required>
          <el-input v-model="page.redisKeyForm.key" :disabled="page.redisKeyDialogMode === 'edit'" :placeholder="at('keyNamePlaceholder')" />
        </el-form-item>
        <el-form-item label="Data Type">
          <el-select v-model="page.redisKeyForm.type" :disabled="page.redisKeyDialogMode === 'edit'" style="width: 100%">
            <el-option v-for="type in page.redisKeyTypes" :key="type" :label="type" :value="type" />
          </el-select>
        </el-form-item>
        <el-form-item label="Value" required>
          <el-input v-model="page.redisKeyForm.value" type="textarea" :rows="8" :placeholder="page.redisKeyValuePlaceholder" />
        </el-form-item>
        <el-form-item :label="at('ttl')">
          <el-input-number v-model="page.redisKeyForm.ttl" :min="-1" :precision="0" controls-position="right" />
          <span class="redis-key-help">{{ at('ttlHelp') }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="page.redisKeyDialogVisible = false">{{ at('cancel') }}</el-button>
        <el-button type="primary" :loading="page.redisRunning" @click="page.submitRedisKey">{{ at('confirmSave') }}</el-button>
      </template>
    </el-dialog>
</template>

<style scoped>
.redis-key-alert {
  margin-bottom: 18px;
}

.redis-key-form .el-form-item:last-child {
  margin-bottom: 0;
}

.redis-key-help {
  margin-left: 12px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
