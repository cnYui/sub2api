import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildRuntimeSql,
  buildSummary,
  scrubSecrets,
} from '../configure-zpay-alipay-runtime.mjs'

const env = {
  ZPAY_PID: 'test_pid_123',
  ZPAY_KEY: 'test_key_secret_456',
  ZPAY_API_BASE: 'https://zpayz.cn/',
  ZPAY_NOTIFY_URL: 'https://api.example.com/api/v1/payment/webhook/easypay',
  ZPAY_RETURN_URL: 'https://example.com/payment/result',
}

test('buildRuntimeSql writes an alipay-only EasyPay provider using ZPay hosted cashier', () => {
  const sql = buildRuntimeSql(env)

  assert.match(sql, /provider_key = 'easypay'/)
  assert.match(sql, /'ZPay Alipay'/)
  assert.match(sql, /supported_types = 'alipay'/)
  assert.match(sql, /payment_mode = 'popup'/)
  assert.match(sql, /10, '', false, 'popup', false/)
  assert.doesNotMatch(sql, /payment_mode = 'qrcode'/)
  assert.match(sql, /refund_enabled = false/)
  assert.match(sql, /allow_user_refund = false/)
  assert.match(sql, /'payment_visible_method_alipay_enabled', 'true'/)
  assert.match(sql, /'payment_visible_method_alipay_source', 'easypay_alipay'/)
  assert.match(sql, /'payment_visible_method_wxpay_enabled', 'false'/)
  assert.match(sql, /'payment_visible_method_wxpay_source', ''/)
  assert.doesNotMatch(sql, /supported_types = 'wxpay'/)
})

test('buildSummary does not expose merchant credentials', () => {
  const summary = buildSummary(env)

  assert.match(summary, /ZPay Alipay/)
  assert.match(summary, /payment method: alipay/)
  assert.match(summary, /launch mode: zpay hosted cashier/)
  assert.match(summary, /wechat: disabled/)
  assert.match(summary, /refund: disabled/)
  assert.doesNotMatch(summary, /test_pid_123/)
  assert.doesNotMatch(summary, /test_key_secret_456/)
})

test('scrubSecrets removes configured credential values from command output', () => {
  const text = 'failed for test_pid_123 with key test_key_secret_456'

  assert.equal(
    scrubSecrets(text, env),
    'failed for [REDACTED_ZPAY_PID] with key [REDACTED_ZPAY_KEY]',
  )
})
