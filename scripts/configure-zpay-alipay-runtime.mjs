#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const DEFAULTS = {
  apiBase: 'https://zpayz.cn',
  pgContainer: 'sub2api-postgres',
  providerName: 'ZPay Alipay',
  paymentMode: 'popup',
  refundEnabled: false,
  allowUserRefund: false,
}

const requiredEnvKeys = ['ZPAY_PID', 'ZPAY_KEY', 'ZPAY_NOTIFY_URL', 'ZPAY_RETURN_URL']

const trimRightSlash = (value) => String(value || '').trim().replace(/\/+$/, '')

const sqlQuote = (value) => `'${String(value).replaceAll("'", "''")}'`

const normalizeURL = (value, name) => {
  const normalized = String(value || '').trim()
  try {
    const parsed = new URL(normalized)
    if (parsed.protocol !== 'https:') throw new Error('protocol')
  } catch {
    throw new Error(`${name} 必须是 HTTPS URL`)
  }
  return normalized
}

const readConfig = (env, opts = {}) => {
  const missing = requiredEnvKeys.filter((key) => !String(env[key] || '').trim())
  if (missing.length > 0) throw new Error(`缺少环境变量：${missing.join(', ')}`)

  const refundEnabled = opts.refundEnabled ?? DEFAULTS.refundEnabled
  const allowUserRefund = opts.allowUserRefund ?? DEFAULTS.allowUserRefund
  if (allowUserRefund && !refundEnabled) {
    throw new Error('开启用户退款前必须开启退款功能')
  }

  return {
    pid: String(env.ZPAY_PID).trim(),
    pkey: String(env.ZPAY_KEY).trim(),
    apiBase: normalizeURL(trimRightSlash(opts.apiBase || env.ZPAY_API_BASE || DEFAULTS.apiBase), 'ZPay 网关'),
    notifyUrl: normalizeURL(opts.notifyUrl || env.ZPAY_NOTIFY_URL, '异步回调地址'),
    returnUrl: normalizeURL(opts.returnUrl || env.ZPAY_RETURN_URL, '同步跳转地址'),
    providerName: String(opts.providerName || DEFAULTS.providerName).trim(),
    paymentMode: String(opts.paymentMode || env.ZPAY_PAYMENT_MODE || DEFAULTS.paymentMode).trim(),
    refundEnabled,
    allowUserRefund,
  }
}

export const buildRuntimeSql = (env, opts = {}) => {
  const cfg = readConfig(env, opts)
  const configJson = JSON.stringify({
    pid: cfg.pid,
    pkey: cfg.pkey,
    apiBase: cfg.apiBase,
    notifyUrl: cfg.notifyUrl,
    returnUrl: cfg.returnUrl,
  })

  return `
BEGIN;

WITH updated AS (
    UPDATE payment_provider_instances
       SET config = ${sqlQuote(configJson)},
           supported_types = 'alipay',
           enabled = true,
           payment_mode = ${sqlQuote(cfg.paymentMode)},
           sort_order = 10,
           limits = '',
           refund_enabled = ${cfg.refundEnabled},
           allow_user_refund = ${cfg.allowUserRefund},
           updated_at = now()
     WHERE provider_key = 'easypay'
       AND name = ${sqlQuote(cfg.providerName)}
 RETURNING id
)
INSERT INTO payment_provider_instances (
    provider_key, name, config, supported_types, enabled,
    sort_order, limits, refund_enabled, payment_mode, allow_user_refund,
    created_at, updated_at
)
SELECT
    'easypay', ${sqlQuote(cfg.providerName)}, ${sqlQuote(configJson)}, 'alipay', true,
    10, '', ${cfg.refundEnabled}, ${sqlQuote(cfg.paymentMode)}, ${cfg.allowUserRefund},
    now(), now()
WHERE NOT EXISTS (SELECT 1 FROM updated);

INSERT INTO settings (key, value, updated_at)
VALUES
  ('payment_enabled', 'true', now()),
  ('payment_visible_method_alipay_enabled', 'true', now()),
  ('payment_visible_method_alipay_source', 'easypay_alipay', now()),
  ('payment_visible_method_wxpay_enabled', 'false', now()),
  ('payment_visible_method_wxpay_source', '', now())
ON CONFLICT (key)
DO UPDATE SET value = EXCLUDED.value, updated_at = now();

COMMIT;
`.trimStart()
}

export const buildSummary = (env, opts = {}) => {
  const cfg = readConfig(env, opts)
  return [
    `provider: ${cfg.providerName}`,
    'provider key: easypay',
    'payment method: alipay',
    'launch mode: zpay hosted cashier',
    `refund: ${cfg.refundEnabled ? 'enabled' : 'disabled'}`,
    `user refund: ${cfg.allowUserRefund ? 'enabled' : 'disabled'}`,
    `api base: ${cfg.apiBase}`,
    `notify url: ${cfg.notifyUrl}`,
    `return url: ${cfg.returnUrl}`,
  ].join('\n')
}

export const scrubSecrets = (text, env) => {
  const replacements = [
    ['ZPAY_PID', '[REDACTED_ZPAY_PID]'],
    ['ZPAY_KEY', '[REDACTED_ZPAY_KEY]'],
  ]
  return replacements.reduce((result, [key, label]) => {
    const value = String(env[key] || '').trim()
    return value ? result.split(value).join(label) : result
  }, String(text || ''))
}

const parseArgs = (argv) => {
  const opts = {
    apply: false,
    pgContainer: DEFAULTS.pgContainer,
    refundEnabled: DEFAULTS.refundEnabled,
    allowUserRefund: DEFAULTS.allowUserRefund,
  }
  for (const arg of argv) {
    if (arg === '--apply') opts.apply = true
    else if (arg === '--dry-run') opts.apply = false
    else if (arg === '--enable-refunds') opts.refundEnabled = true
    else if (arg === '--allow-user-refund') opts.allowUserRefund = true
    else if (arg === '--help' || arg === '-h') opts.help = true
    else if (arg.startsWith('--pg-container=')) opts.pgContainer = arg.slice('--pg-container='.length)
    else if (arg.startsWith('--api-base=')) opts.apiBase = arg.slice('--api-base='.length)
    else if (arg.startsWith('--notify-url=')) opts.notifyUrl = arg.slice('--notify-url='.length)
    else if (arg.startsWith('--return-url=')) opts.returnUrl = arg.slice('--return-url='.length)
    else throw new Error(`未知参数：${arg}`)
  }
  return opts
}

const printHelp = () => {
  console.log(`用法：
  ZPAY_PID=... ZPAY_KEY=... ZPAY_NOTIFY_URL=... ZPAY_RETURN_URL=... node scripts/configure-zpay-alipay-runtime.mjs
  ZPAY_PID=... ZPAY_KEY=... ZPAY_NOTIFY_URL=... ZPAY_RETURN_URL=... node scripts/configure-zpay-alipay-runtime.mjs --apply

参数：
  --apply                    写入 PostgreSQL；默认只 dry-run
  --dry-run                  只校验并输出脱敏摘要
  --pg-container=NAME        PostgreSQL 容器名，默认 sub2api-postgres
  --api-base=URL             ZPay 网关，默认 https://zpayz.cn
  --notify-url=URL           ZPay 异步通知地址
  --return-url=URL           支付完成浏览器跳转地址
  --enable-refunds           开启服务端退款
  --allow-user-refund        允许用户发起退款，需同时传 --enable-refunds`)
}

const runPsql = (sql, opts, env) => {
  const result = spawnSync(
    'docker',
    ['exec', '-i', opts.pgContainer, 'psql', '-U', 'sub2api', '-d', 'sub2api', '-v', 'ON_ERROR_STOP=1'],
    {
      input: sql,
      encoding: 'utf8',
      maxBuffer: 1024 * 1024 * 16,
    },
  )

  if (result.status !== 0) {
    const stdout = scrubSecrets(result.stdout, env)
    const stderr = scrubSecrets(result.stderr, env)
    throw new Error(`写入 ZPay 运行态配置失败：exit=${result.status}\nstdout:\n${stdout}\nstderr:\n${stderr}`)
  }

  return scrubSecrets(result.stdout, env)
}

const runCli = (argv, env) => {
  const opts = parseArgs(argv)
  if (opts.help) {
    printHelp()
    return
  }

  const sql = buildRuntimeSql(env, opts)
  console.log(buildSummary(env, opts))

  if (!opts.apply) {
    console.log('dry-run: no database changes were applied')
    return
  }

  const output = runPsql(sql, opts, env)
  if (output.trim()) console.log(output.trim())
  console.log('applied: ZPay Alipay runtime payment config updated')
}

const isMain = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]

if (isMain) {
  try {
    runCli(process.argv.slice(2), process.env)
  } catch (error) {
    console.error(scrubSecrets(error.message, process.env))
    process.exit(1)
  }
}
