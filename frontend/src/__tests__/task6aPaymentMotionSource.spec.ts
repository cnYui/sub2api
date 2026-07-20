import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()
const read = (path: string) => readFileSync(join(root, path), 'utf8')

const paymentResultSource = read('src/views/user/PaymentResultView.vue')
const paymentViewSource = read('src/views/user/PaymentView.vue')
const dataTableSource = read('src/components/common/DataTable.vue')

describe('Task 6A 支付页面 Material Relay 动效源码契约', () => {
  it('支付结果状态使用稳定 key 的 out-in crossfade，并在 reduced motion 下只保留 opacity', () => {
    expect(paymentResultSource).toContain('<Transition name="payment-result-status" mode="out-in">')
    expect(paymentResultSource).toContain('key="success"')
    expect(paymentResultSource).toContain('key="pending"')
    expect(paymentResultSource).toContain('key="failed"')
    expect(paymentResultSource).toMatch(
      /\.payment-result-status-enter-active,[\s\S]*?transition:\s*transform 220ms var\(--ease-out\),\s*opacity 220ms var\(--ease-out\)/,
    )
    expect(paymentResultSource).toMatch(
      /\.payment-result-status-enter-from,[\s\S]*?transform:\s*scale\(0\.95\)/,
    )
    expect(paymentResultSource).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.payment-result-status-enter-active,[\s\S]*?transition-property:\s*opacity[\s\S]*?\.payment-result-status-enter-from,[\s\S]*?transform:\s*none/,
    )
    expect(paymentResultSource).toContain('class="card p-5"')
    expect(paymentResultSource).not.toContain('rounded-xl bg-white p-5 shadow-sm')
  })

  it('支付阶段 transition 使用非阻塞的 180ms transform/opacity 过渡', () => {
    expect(paymentViewSource).toContain('<Transition name="payment-phase">')
    expect(paymentViewSource).not.toContain('<Transition name="payment-phase" mode="out-in">')
    expect(paymentViewSource).toContain(':key="paymentPhase"')
    expect(paymentViewSource).toMatch(
      /\.payment-phase-enter-active,[\s\S]*?transition:\s*transform 180ms var\(--ease-out\),\s*opacity 180ms var\(--ease-out\)/,
    )
    expect(paymentViewSource).toMatch(
      /\.payment-phase-enter-from,[\s\S]*?transform:\s*translate3d\(0, 4px, 0\)/,
    )
    expect(paymentViewSource).not.toMatch(/payment-phase[\s\S]{0,400}setTimeout/)
  })

  it('DataTable 排序箭头只使用 120ms transform 过渡', () => {
    expect(dataTableSource).toContain('data-table-sort-arrow')
    expect(dataTableSource).toMatch(
      /\.data-table-sort-arrow\s*\{[\s\S]*?transition:\s*transform 120ms var\(--ease-out\)/,
    )
    expect(dataTableSource).not.toMatch(/data-table-sort-arrow[\s\S]*?transition:\s*all\b/)
  })
})
