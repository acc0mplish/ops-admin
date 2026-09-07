// Slice C UI-flow E2E (plan N9 — PR 31c, Phase G).
//
// The stack under test is bootstrapped by e2e/stack.sh serve-c: a dedicated
// schema, the real backend child, and a MOCK proxmox account seeded straight
// into the V2 tables. The rows are NOT hand-written (R10): stack.sh drives
// TestSliceCSeedWriter (backend/internal/infra/adapter/proxmox/
// slice_c_seed_test.go), which renders one compute.vm, one
// compute.system_container and one compute.hypervisor_node through the real
// normalizer — the kinds, URNs and JSON payloads this spec asserts are the
// normalizer's own output.
//
// Scope limit (plan J10): this proves the UI read path only — the compute
// view's kindPrefix=compute. family filter, the detail drawer, and the
// opdef × kind intersection surfacing pve.guest.* (server-produced; the
// ResourceOperations panel needs no per-view wiring). Real-endpoint
// Validate/Health → register-pve → sync (claim 10) is NOT covered here —
// that gate runs through the pve-provisioning runbook with real credentials.
import { test, expect } from 'playwright/test'

const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'slicea-e2e-admin'
const VM_NAME = 'web-e2e-01'
const CT_NAME = 'ct-e2e-01'

test.describe.configure({ mode: 'serial' })

// Each test gets a fresh browser context, so every test authenticates through
// the real login view first.
test.beforeEach(async ({ page }) => {
  // Pin the display locale so the assertions below read the en-US strings.
  await page.addInitScript((locale) => {
    window.localStorage.setItem('ops-admin-display-locale', locale)
  }, 'en-US')
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await page.locator('.el-input input').first().fill('admin')
  await page.locator('input[type="password"]').fill(ADMIN_PASSWORD)
  await page.locator('button.submit-btn').click()
  await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 })
})

test('compute inventory lists the seeded PVE resources and offers the proxmox kinds', async ({ page }) => {
  // The view must actually issue the family filter — the M12 wiring under
  // proof, same request shape slice B asserted for the cloud mock.
  const listRequest = page.waitForRequest(
    (request) => request.url().includes('/api/v2/infra/resources') && request.url().includes('kindPrefix=compute.')
  )
  await page.goto('/infra/compute')
  await expect(page.locator('.page-title')).toHaveText('Compute Inventory')
  await listRequest

  // All three seeded PVE kinds render — guest kinds AND the hypervisor node.
  await expect(page.locator('.el-table__row', { hasText: VM_NAME })).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('.el-table__row', { hasText: CT_NAME })).toBeVisible()
  await expect(page.locator('.el-table__row', { hasText: 'hypervisor_node' })).toBeVisible()

  // M9: the kind filter offers the two Phase 5 kinds (labels are the kind
  // strings themselves — no i18n keys ride on them, A9).
  await page.locator('.el-select').click()
  await expect(page.locator('.el-select-dropdown__item', { hasText: 'compute.hypervisor_node' })).toBeVisible()
  await expect(page.locator('.el-select-dropdown__item', { hasText: 'compute.system_container' })).toBeVisible()

  // Selecting one narrows the list to that kind only.
  const filtered = page.waitForResponse(
    (response) => response.url().includes('/api/v2/infra/resources') && response.url().includes('kind=compute.hypervisor_node')
  )
  await page.locator('.el-select-dropdown__item', { hasText: 'compute.hypervisor_node' }).click()
  await filtered
  await expect(page.locator('.el-table__row', { hasText: 'hypervisor_node' })).toBeVisible()
  await expect(page.locator('.el-table__row', { hasText: VM_NAME })).toHaveCount(0)
  await expect(page.locator('.el-table__row', { hasText: CT_NAME })).toHaveCount(0)
})

test('compute resource detail drawer shows the normalized PVE observation', async ({ page }) => {
  await page.goto('/infra/compute')
  const row = page.locator('.el-table__row', { hasText: VM_NAME })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.click()

  const drawer = page.locator('.el-drawer')
  await expect(drawer).toBeVisible()
  // The URN is the normalizer's urnFor output (seedContextID=1) — asserted
  // verbatim so a generator drift fails loudly here, not in production.
  await expect(drawer).toContainText('urn:proxmox:1:vm:pve-e2e-1/100')
  await expect(drawer).toContainText('compute.vm')
  // Normalized first (bytes → GB / MB rounding, csv tags), then the raw
  // provider payload — the K8sInventory drawer pattern.
  await expect(drawer).toContainText('"cores": 2')
  await expect(drawer).toContainText('"memoryMB": 2048')
  await expect(drawer).toContainText('"tags": [\n    "e2e",\n    "web"\n  ]')
  await expect(drawer).toContainText('"guest": "qemu"')
  await drawer.locator('button.drawer-close').click()
  await expect(drawer).toBeHidden()
})

test('operations panel exposes the pve guest ops on the seeded VM through the opdef-kind intersection', async ({ page }) => {
  // The K8s inventory view reads every kind unfiltered, so the PVE rows ride
  // there too — and only that view's drawer carries the Operations tab
  // (ResourceOperations is a shared component; plan J10 wires nothing new).
  await page.goto('/infra/resources')
  await expect(page.locator('.page-title')).toHaveText('K8s Inventory')
  const row = page.locator('.el-table__row', { hasText: VM_NAME })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.click()

  const drawer = page.locator('.el-drawer')
  await expect(drawer).toBeVisible()
  await drawer.locator('.el-tabs__item', { hasText: 'Operations' }).click()

  // The three Phase D opdefs attach to compute.vm — server-produced list
  // (ListResourceOperations), no per-view wiring.
  await expect(drawer.locator('.el-table__row', { hasText: 'pve.guest.power' })).toBeVisible({ timeout: 20_000 })
  await expect(drawer.locator('.el-table__row', { hasText: 'pve.guest.snapshot' })).toBeVisible()
  await expect(drawer.locator('.el-table__row', { hasText: 'pve.guest.config' })).toBeVisible()

  // Intersection, not union: the k8s opdef must not attach to a PVE kind.
  await expect(drawer.locator('.el-table__row', { hasText: 'k8s.workload.restart' })).toHaveCount(0)
})

test('hypervisor node exposes no operations — guest kinds are the only proxmox opdef subjects', async ({ page }) => {
  await page.goto('/infra/resources')
  const row = page.locator('.el-table__row', { hasText: 'compute.hypervisor_node' })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.click()

  const drawer = page.locator('.el-drawer')
  await expect(drawer).toBeVisible()
  await drawer.locator('.el-tabs__item', { hasText: 'Operations' }).click()
  // proxmoxGuestKinds = {compute.vm, compute.system_container} — the node
  // kind sits outside every opdef, so the panel renders its empty state.
  await expect(drawer.locator('.resource-operations .el-alert')).toBeVisible({ timeout: 20_000 })
  await expect(drawer.locator('.resource-operations .el-table__row')).toHaveCount(0)
})
