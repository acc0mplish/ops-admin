import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { executeRedisCommand } from '../api/dbms'
import { at } from '../utils/asset-i18n'
import { confirmRiskOperation } from './useRiskConfirm'
import { isProductionEnvironment } from './useDbmsSqlExecution'

// I5-H — extracted from views/assets/DatabaseWorkbench.vue (behavior
// preservation + rewiring contract, i5-plan §3.8). Original coordinates per
// block: 40·45·55·90 state · 140-145 key form · 178-191 command snippets ·
// 217-222 value placeholder · 827-871 command execute·run · 873-1020 key
// builders·CRUD. isProductionEnvironment is the module export of
// useDbmsSqlExecution (single source). Free variables injected: result/exec
// refs (useDbmsSqlExecution)·selection refs·tree/history loaders.
export function useDbmsRedis({
  databaseId,
  connection,
  isRedis,
  selectedSchema,
  selectedTable,
  selectedRows,
  selectedColumns,
  schemaTree,
  execMeta,
  resultColumns,
  resultRows,
  activeTab,
  resetExecMeta,
  loadTree,
  loadHistory,
  loadResourceData
}) {
  const redisRunning = ref(false)
  const redisKeyDialogVisible = ref(false)
  const redisKeyDialogMode = ref('create')
  const redisCommandText = ref('GET key')

  const redisKeyForm = reactive({
    key: '',
    type: 'string',
    value: '',
    ttl: undefined
  })

  const redisCommandSnippets = [
    { label: 'GET', text: 'GET key' },
    { label: 'MGET', text: 'MGET key1 key2' },
    { label: 'TTL', text: 'TTL key' },
    { label: 'TYPE', text: 'TYPE key' },
    { label: 'SCAN', text: 'SCAN 0 MATCH * COUNT 100' },
    { label: 'HGETALL', text: 'HGETALL hash_key' },
    { label: 'LRANGE', text: 'LRANGE list_key 0 -1' },
    { label: 'SMEMBERS', text: 'SMEMBERS set_key' },
    { label: 'ZRANGE', text: 'ZRANGE zset_key 0 -1 WITHSCORES' },
    { label: 'SET', text: 'SET key value' },
    { label: 'DEL', text: 'DEL key' },
    { label: 'EXPIRE', text: 'EXPIRE key 3600' }
  ]

  const redisKeyTypes = ['string', 'hash', 'list', 'set', 'zset']

  const redisKeyValuePlaceholder = computed(() => {
    if (redisKeyForm.type === 'hash') return '{"field":"value"}'
    if (redisKeyForm.type === 'zset') return '[{"member":"user:1","score":100}]'
    if (['list', 'set'].includes(redisKeyForm.type)) return '["value-1", "value-2"]'
    return at('enterStringValue')
  })

  async function executeRedisCommandText(confirmed = false) {
    const data = await executeRedisCommand({
      databaseId: databaseId.value,
      commandText: redisCommandText.value.trim(),
      confirmed
    })
    resultColumns.value = data.columns || []
    resultRows.value = data.rows || []
    execMeta.sqlType = `REDIS ${data.command || ''}`.trim()
    execMeta.rowsAffected = Number(data.rowsAffected || 0)
    execMeta.durationMs = Number(data.durationMs || 0)
    activeTab.value = 'result'
    ElMessage.success(at('redisCommandDone'))
    await loadHistory()
    if (selectedTable.value) await loadResourceData()
  }

  async function runRedisCommand() {
    if (!isRedis.value) return
    if (!redisCommandText.value.trim()) {
      ElMessage.warning(at('enterRedisCommand'))
      return
    }
    redisRunning.value = true
    resetExecMeta()
    try {
      const analysis = await analyzeRedisCommand({
        databaseId: databaseId.value,
        commandText: redisCommandText.value.trim()
      })
      if (analysis.writeOperation) {
        await confirmRiskOperation({
          operation: `Redis Write Command: ${analysis.command}`,
          targetSummary: `${connection.value?.databaseName || connection.value?.name || databaseId.value}`,
          production: isProductionEnvironment(connection.value?.environment),
          destructive: true
        })
      }
      await executeRedisCommandText(analysis.writeOperation)
    } catch (error) {
      if (error !== 'cancel' && error !== 'close') throw error
    } finally {
      redisRunning.value = false
    }
  }

  function quoteRedisArgument(value) {
    return JSON.stringify(String(value ?? ''))
  }

  function resetRedisKeyForm() {
    redisKeyForm.key = ''
    redisKeyForm.type = 'string'
    redisKeyForm.value = ''
    redisKeyForm.ttl = undefined
  }

  function openRedisKeyCreate() {
    resetRedisKeyForm()
    redisKeyDialogMode.value = 'create'
    redisKeyDialogVisible.value = true
  }

  function openRedisKeyEdit(row) {
    if (row.type === 'stream') {
      ElMessage.warning(at('streamNotEditableWarning'))
      return
    }
    redisKeyDialogMode.value = 'edit'
    redisKeyForm.key = row.key || ''
    redisKeyForm.type = row.type || 'string'
    redisKeyForm.value = row.value || ''
    redisKeyForm.ttl = Number(row.ttlSeconds) >= 0 ? Number(row.ttlSeconds) : undefined
    redisKeyDialogVisible.value = true
  }

  function parseRedisValueArguments() {
    const raw = redisKeyForm.value ?? ''
    if (redisKeyForm.type === 'string') return [quoteRedisArgument(raw)]
    let parsed
    try {
      parsed = JSON.parse(raw)
    } catch {
      throw new Error(at('invalidComplexValue'))
    }
    if (redisKeyForm.type === 'hash') {
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error(at('hashMustBeObject'))
      const entries = Object.entries(parsed)
      if (!entries.length) throw new Error(at('hashRequiresField'))
      return entries.flatMap(([field, value]) => [quoteRedisArgument(field), quoteRedisArgument(value)])
    }
    if (redisKeyForm.type === 'zset') {
      if (!Array.isArray(parsed) || !parsed.length) throw new Error(at('zsetMustBeArray'))
      return parsed.flatMap((item) => {
        if (!item || item.member === undefined || Number.isNaN(Number(item.score))) throw new Error(at('zsetElementRequired'))
        return [quoteRedisArgument(item.score), quoteRedisArgument(item.member)]
      })
    }
    if (!Array.isArray(parsed) || !parsed.length) throw new Error(at('listSetMustBeArray'))
    return parsed.map((item) => quoteRedisArgument(item))
  }

  function redisTTLCommand(key) {
    if (redisKeyForm.ttl === undefined || redisKeyForm.ttl === null || redisKeyForm.ttl === '') return ''
    const ttl = Number(redisKeyForm.ttl)
    if (!Number.isInteger(ttl) || ttl < -1) throw new Error(at('invalidTTL'))
    return ttl === -1 ? `PERSIST ${quoteRedisArgument(key)}` : `EXPIRE ${quoteRedisArgument(key)} ${ttl}`
  }

  function buildRedisKeyCommands() {
    const key = redisKeyForm.key.trim()
    if (!key) throw new Error(at('enterKeyName'))
    const valueArgs = parseRedisValueArguments()
    const quotedKey = quoteRedisArgument(key)
    const editing = redisKeyDialogMode.value === 'edit'
    let command = ''
    switch (redisKeyForm.type) {
      case 'string':
        command = `SET ${quotedKey} ${valueArgs[0]}${editing && redisKeyForm.ttl === undefined ? ' KEEPTTL' : ''}`
        break
      case 'hash': command = `HSET ${quotedKey} ${valueArgs.join(' ')}`; break
      case 'list': command = `RPUSH ${quotedKey} ${valueArgs.join(' ')}`; break
      case 'set': command = `SADD ${quotedKey} ${valueArgs.join(' ')}`; break
      case 'zset': command = `ZADD ${quotedKey} ${valueArgs.join(' ')}`; break
      default: throw new Error(at('unsupportedRedisKeyType'))
    }
    const commands = []
    if (editing && redisKeyForm.type !== 'string') commands.push(`DEL ${quotedKey}`)
    commands.push(command)
    const ttlCommand = redisTTLCommand(key)
    if (ttlCommand) commands.push(ttlCommand)
    return commands
  }

  async function submitRedisKey() {
    let commands
    try {
      commands = buildRedisKeyCommands()
    } catch (error) {
      ElMessage.warning(error.message || at('invalidKeyContent'))
      return
    }
    const action = redisKeyDialogMode.value === 'create' ? at('addAction') : at('updateAction')
    try {
      await confirmRiskOperation({
        operation: at('redisKeyOperation', { action }),
        targetSummary: redisKeyForm.key,
        production: isProductionEnvironment(connection.value?.environment),
        destructive: redisKeyDialogMode.value === 'edit'
      })
    } catch {
      return
    }
    redisRunning.value = true
    try {
      for (const commandText of commands) {
        await executeRedisCommand({ databaseId: databaseId.value, commandText, confirmed: true })
      }
      redisKeyDialogVisible.value = false
      await Promise.all([loadTree(), loadHistory()])
      selectedSchema.value = schemaTree.value?.[0]?.name || selectedSchema.value
      selectedTable.value = redisKeyForm.key
      await loadResourceData()
      ElMessage.success(at('redisKeyActionDone', { action }))
    } finally {
      redisRunning.value = false
    }
  }

  async function deleteRedisKey(row) {
    try {
      await confirmRiskOperation({
        operation: at('redisKeyDeleteOperation'),
        targetSummary: row.key,
        production: isProductionEnvironment(connection.value?.environment),
        destructive: true
      })
    } catch {
      return
    }
    redisRunning.value = true
    try {
      await executeRedisCommand({ databaseId: databaseId.value, commandText: `DEL ${quoteRedisArgument(row.key)}`, confirmed: true })
      if (selectedTable.value === row.key) {
        selectedTable.value = ''
        selectedRows.value = []
        selectedColumns.value = []
      }
      await Promise.all([loadTree(), loadHistory()])
      ElMessage.success(at('redisKeyDeleted'))
    } finally {
      redisRunning.value = false
    }
  }

  return {
    redisRunning,
    redisKeyDialogVisible,
    redisKeyDialogMode,
    redisCommandText,
    redisKeyForm,
    redisCommandSnippets,
    redisKeyTypes,
    redisKeyValuePlaceholder,
    executeRedisCommandText,
    runRedisCommand,
    openRedisKeyCreate,
    openRedisKeyEdit,
    submitRedisKey,
    deleteRedisKey
  }
}
