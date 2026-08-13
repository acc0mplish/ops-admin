import { ElMessageBox } from 'element-plus'

/**
 * A consistent confirmation gate for production and destructive operations.
 * The backend still owns authorization; this supplies an explicit, auditable
 * client-side acknowledgement before a request is sent.
 */
export async function confirmRiskOperation({
  operation,
  targetSummary,
  targetCount = 0,
  production = false,
  destructive = false
}) {
  const acknowledgement = production ? '生产环境' : destructive ? '我确认' : '确认执行'
  const riskText = [
    production ? '目标包含生产环境。' : '',
    destructive ? '该操作可能修改或删除现有数据。' : '',
    targetCount ? `本次将作用于 ${targetCount} 个目标。` : ''
  ].filter(Boolean).join(' ')

  await ElMessageBox.prompt(
    `${riskText}\n\n操作：${operation}\n目标：${targetSummary}\n\n请输入“${acknowledgement}”后继续。`,
    production ? '生产操作确认' : '高风险操作确认',
    {
      type: 'warning',
      inputPlaceholder: acknowledgement,
      inputValidator: (value) => value === acknowledgement || `请输入“${acknowledgement}”以确认`,
      confirmButtonText: '确认并继续',
      cancelButtonText: '取消',
      closeOnClickModal: false,
      closeOnPressEscape: false
    }
  )
}
