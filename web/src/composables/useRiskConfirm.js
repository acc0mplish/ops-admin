import { ElMessageBox } from 'element-plus'
import { ct } from '../utils/common-i18n'

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
  const acknowledgement = production ? ct('productionAck') : destructive ? ct('destructiveAck') : ct('executeAck')
  const riskText = [
    production ? ct('productionRisk') : '',
    destructive ? ct('destructiveRisk') : '',
    targetCount ? ct('targetCountRisk', { count: targetCount }) : ''
  ].filter(Boolean).join(' ')

  await ElMessageBox.prompt(
    ct('riskPrompt', { risk: riskText, operation, target: targetSummary, ack: acknowledgement }),
    production ? ct('productionConfirmTitle') : ct('highRiskConfirmTitle'),
    {
      type: 'warning',
      inputPlaceholder: acknowledgement,
      inputValidator: (value) => value === acknowledgement || ct('acknowledgementRequired', { ack: acknowledgement }),
      confirmButtonText: ct('confirmContinue'),
      cancelButtonText: ct('cancel'),
      closeOnClickModal: false,
      closeOnPressEscape: false
    }
  )
}
