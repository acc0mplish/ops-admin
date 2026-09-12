import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { deleteDBMSTableRow, insertDBMSTableRow, updateDBMSTableRow } from '../api/dbms'
import { at } from '../utils/asset-i18n'
import { confirmRiskOperation } from './useRiskConfirm'
import { isProductionEnvironment } from './useDbmsSqlExecution'

// I5-H — extracted from views/assets/DatabaseWorkbench.vue (behavior
// preservation + rewiring contract, i5-plan §3.8). Original coordinates per
// block: 44·54 row dialog state · 61-62 selected keys?? (selection refs stay
// in view) · 130-136 cell edit state · 439-485 row key·cell edit · 1103-1167
// row dialog open·submit·delete. isProductionEnvironment is the module export
// of useDbmsSqlExecution. Free variables injected: databaseId·selection refs·
// capability computeds (view)·load fns.
export function useDbmsRowOps({
  databaseId,
  selectedSchema,
  selectedTable,
  selectedColumns,
  selectedPrimaryKeys,
  canEditRows,
  isReadOnly,
  connection,
  loadTableData,
  loadHistory
}) {
  const rowDialogVisible = ref(false)
  const rowDialogMode = ref('insert')

  const editingCell = reactive({
    rowKey: '',
    column: ''
  })

  const pendingCellValue = ref('')
  const editingOriginalRow = ref(null)

  const rowForm = reactive({})
  const rowOriginal = ref({})

  function rowKeyFor(row, index = 0) {
    const keys = selectedPrimaryKeys.value || []
    if (keys.length) {
      return keys.map((key) => `${key}:${row[key] ?? ''}`).join('|')
    }
    return `${index}:${JSON.stringify(row)}`
  }

  function isEditingCell(row, column, index) {
    return editingCell.rowKey === rowKeyFor(row, index) && editingCell.column === column
  }

  function startCellEdit(row, column, index) {
    if (!canEditRows.value) {
      ElMessage.warning(isReadOnly.value ? at('readOnlyDatabaseWarning') : at('noPrimaryKeyWarning'))
      return
    }
    editingCell.rowKey = rowKeyFor(row, index)
    editingCell.column = column
    pendingCellValue.value = row[column] == null ? '' : String(row[column])
    editingOriginalRow.value = { ...row }
  }

  function cancelCellEdit() {
    editingCell.rowKey = ''
    editingCell.column = ''
    pendingCellValue.value = ''
    editingOriginalRow.value = null
  }

  async function commitCellEdit(row, column) {
    if (!editingOriginalRow.value) {
      cancelCellEdit()
      return
    }
    const current = { ...row, [column]: pendingCellValue.value }
    await updateDBMSTableRow({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      original: editingOriginalRow.value,
      current
    })
    ElMessage.success(at('cellUpdated'))
    cancelCellEdit()
    await Promise.all([loadTableData(), loadHistory()])
  }

  function openInsertRow() {
    rowDialogMode.value = 'insert'
    rowOriginal.value = {}
    Object.keys(rowForm).forEach((key) => delete rowForm[key])
    for (const col of selectedColumns.value) {
      rowForm[col.name] = null
    }
    rowDialogVisible.value = true
  }

  function openEditRow(row) {
    rowDialogMode.value = 'update'
    rowOriginal.value = { ...row }
    Object.keys(rowForm).forEach((key) => delete rowForm[key])
    for (const col of selectedColumns.value) {
      rowForm[col.name] = row[col.name] ?? null
    }
    rowDialogVisible.value = true
  }

  async function submitRow() {
    await confirmRiskOperation({
      operation: rowDialogMode.value === 'insert' ? at('rowInsertOperation') : at('rowUpdateOperation'),
      targetSummary: `${selectedSchema.value}.${selectedTable.value}`,
      production: isProductionEnvironment(connection.value?.environment),
      destructive: rowDialogMode.value === 'update'
    })
    if (rowDialogMode.value === 'insert') {
      await insertDBMSTableRow({
        databaseId: databaseId.value,
        schema: selectedSchema.value,
        table: selectedTable.value,
        row: { ...rowForm }
      })
      ElMessage.success(at('rowInserted'))
    } else {
      await updateDBMSTableRow({
        databaseId: databaseId.value,
        schema: selectedSchema.value,
        table: selectedTable.value,
        original: rowOriginal.value,
        current: { ...rowForm }
      })
      ElMessage.success(at('rowUpdated'))
    }
    rowDialogVisible.value = false
    await Promise.all([loadTableData(), loadHistory()])
  }

  async function handleDeleteRow(row) {
    await confirmRiskOperation({
      operation: at('rowDeleteOperation'),
      targetSummary: `${selectedSchema.value}.${selectedTable.value}`,
      production: isProductionEnvironment(connection.value?.environment),
      destructive: true
    })
    await deleteDBMSTableRow({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      row
    })
    ElMessage.success(at('rowDeleted'))
    await Promise.all([loadTableData(), loadHistory()])
  }

  return {
    rowDialogVisible,
    rowDialogMode,
    rowForm,
    rowOriginal,
    editingCell,
    pendingCellValue,
    editingOriginalRow,
    rowKeyFor,
    isEditingCell,
    startCellEdit,
    cancelCellEdit,
    commitCellEdit,
    openInsertRow,
    openEditRow,
    submitRow,
    handleDeleteRow
  }
}
