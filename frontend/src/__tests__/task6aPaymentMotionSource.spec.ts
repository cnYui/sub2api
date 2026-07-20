import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()
const read = (path: string) => readFileSync(join(root, path), 'utf8')

const paymentResultSource = read('src/views/user/PaymentResultView.vue')
const paymentViewSource = read('src/views/user/PaymentView.vue')
const dataTableSource = read('src/components/common/DataTable.vue')
const dataTableTemplate = dataTableSource.slice(0, dataTableSource.indexOf('<script setup'))

describe('Task 6A 支付页面 Material Relay 动效源码契约', () => {
  it('支付结果状态在固定轨道并行 crossfade，并在 reduced motion 下保留静态处理中状态', () => {
    expect(paymentResultSource).toContain('<div class="relative mx-auto h-20 w-20">')
    expect(paymentResultSource).toContain('<Transition name="payment-result-status">')
    expect(paymentResultSource).not.toContain('<Transition name="payment-result-status" mode="out-in">')
    expect(paymentResultSource).toContain('key="success"')
    expect(paymentResultSource).toContain('key="pending"')
    expect(paymentResultSource).toContain('key="failed"')
    expect(paymentResultSource.match(/absolute inset-0/g) ?? []).toHaveLength(3)
    expect(paymentResultSource).toMatch(
      /\.payment-result-status-enter-active,[\s\S]*?transition:\s*transform 220ms var\(--ease-out\),\s*opacity 220ms var\(--ease-out\)/,
    )
    expect(paymentResultSource).toMatch(
      /\.payment-result-status-enter-from,[\s\S]*?transform:\s*scale\(0\.95\)/,
    )
    expect(paymentResultSource).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.payment-result-status-enter-active,[\s\S]*?transition-property:\s*opacity[\s\S]*?\.payment-result-status-enter-from,[\s\S]*?transform:\s*none/,
    )
    expect(paymentResultSource).toContain('payment-result-pending-spinner')
    expect(paymentResultSource).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.payment-result-pending-spinner\s*\{[\s\S]*?animation:\s*none/,
    )
    expect(paymentResultSource).toContain('class="card p-5"')
    expect(paymentResultSource).not.toContain('rounded-xl bg-white p-5 shadow-sm')
  })

  it('支付阶段 transition 使用非阻塞的 180ms transform/opacity 过渡', () => {
    expect(paymentViewSource).toContain('<Transition name="payment-phase">')
    expect(paymentViewSource).not.toContain('<Transition name="payment-phase" mode="out-in">')
    expect(paymentViewSource).toContain(':key="paymentPhase"')
    expect(paymentViewSource).toContain("'relative mx-auto space-y-6'")
    expect(paymentViewSource).toMatch(
      /\.payment-phase-enter-active,[\s\S]*?transition:\s*transform 180ms var\(--ease-out\),\s*opacity 180ms var\(--ease-out\)/,
    )
    expect(paymentViewSource).toMatch(
      /\.payment-phase-enter-from,[\s\S]*?transform:\s*translate3d\(0, 4px, 0\)/,
    )
    expect(paymentViewSource).toMatch(
      /\.payment-phase-leave-active\s*\{[\s\S]*?position:\s*absolute;[\s\S]*?left:\s*0;[\s\S]*?right:\s*0;[\s\S]*?pointer-events:\s*none/,
    )
    expect(paymentViewSource).not.toMatch(/payment-phase[\s\S]{0,400}setTimeout/)
  })

  it('DataTable 使用同一排序箭头完成 120ms transform 过渡', () => {
    expect(dataTableSource).toContain('data-table-sort-arrow')
    expect(dataTableTemplate.match(/data-table-sort-arrow/g) ?? []).toHaveLength(1)
    expect(dataTableTemplate).toContain("'rotate-180': sortKey === column.key && sortOrder === 'asc'")
    expect(dataTableTemplate).toContain("'opacity-50': sortKey !== column.key")
    expect(dataTableTemplate).not.toContain('<svg v-if="sortKey === column.key"')
    expect(dataTableTemplate).not.toContain('<svg v-else class="data-table-sort-arrow h-4 w-4"')
    expect(dataTableSource).toMatch(
      /\.data-table-sort-arrow\s*\{[\s\S]*?transition:\s*transform 120ms var\(--ease-out\)/,
    )
    expect(dataTableSource).not.toMatch(/data-table-sort-arrow[\s\S]*?transition:\s*all\b/)
  })
})
