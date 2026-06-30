#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const DEFAULTS = {
  pgContainer: 'sub2api-candidate-postgres',
  accountName: 'cliproxy-local-openai',
  groupNames: ['codex-pool-69-usd', 'codex-pool-89-usd'],
}

const sqlQuote = (value) => `'${String(value).replaceAll("'", "''")}'`

const assertNonBlank = (value, label) => {
  const text = String(value || '').trim()
  if (!text) throw new Error(`${label} 不能为空`)
  return text
}

const normalizeGroupNames = (groupNames) => {
  const names = groupNames.map((name) => assertNonBlank(name, 'group')).filter(Boolean)
  return [...new Set(names)]
}

export const parseArgs = (argv) => {
  const opts = {
    apply: false,
    pgContainer: DEFAULTS.pgContainer,
    accountName: DEFAULTS.accountName,
    groupNames: [],
  }

  for (const arg of argv) {
    if (arg === '--apply') opts.apply = true
    else if (arg === '--dry-run') opts.apply = false
    else if (arg === '--help' || arg === '-h') opts.help = true
    else if (arg.startsWith('--pg-container=')) opts.pgContainer = arg.slice('--pg-container='.length)
    else if (arg.startsWith('--account-name=')) opts.accountName = arg.slice('--account-name='.length)
    else if (arg.startsWith('--group=')) opts.groupNames.push(arg.slice('--group='.length))
    else throw new Error(`未知参数：${arg}`)
  }

  opts.pgContainer = assertNonBlank(opts.pgContainer, 'pg-container')
  opts.accountName = assertNonBlank(opts.accountName, 'account-name')
  opts.groupNames = normalizeGroupNames(opts.groupNames.length > 0 ? opts.groupNames : DEFAULTS.groupNames)
  return opts
}

export const buildRuntimeSql = (opts = {}) => {
  const accountName = assertNonBlank(opts.accountName || DEFAULTS.accountName, 'account-name')
  const groupNames = normalizeGroupNames(opts.groupNames || DEFAULTS.groupNames)
  const groupValues = groupNames.map((name) => `(${sqlQuote(name)})`).join(',\n        ')

  return `
BEGIN;

DO $$
DECLARE
    target_account_id BIGINT;
    target_group RECORD;
    target_group_id BIGINT;
BEGIN
    SELECT id INTO target_account_id
      FROM accounts
     WHERE name = ${sqlQuote(accountName)}
       AND deleted_at IS NULL
       AND platform = 'openai'
       AND status = 'active'
       AND schedulable = true
     ORDER BY id
     LIMIT 1;

    IF target_account_id IS NULL THEN
        RAISE EXCEPTION 'active schedulable OpenAI account not found: %', ${sqlQuote(accountName)};
    END IF;

    FOR target_group IN
        SELECT name
          FROM (VALUES
        ${groupValues}
          ) AS target_groups(name)
    LOOP
        SELECT id INTO target_group_id
          FROM groups
         WHERE name = target_group.name
           AND deleted_at IS NULL
           AND platform = 'openai'
           AND status = 'active'
         ORDER BY id
         LIMIT 1;

        IF target_group_id IS NULL THEN
            RAISE EXCEPTION 'active OpenAI group not found: %', target_group.name;
        END IF;

        INSERT INTO account_groups (account_id, group_id, priority, created_at)
        VALUES (target_account_id, target_group_id, 1, NOW())
        ON CONFLICT (account_id, group_id)
        DO UPDATE SET priority = LEAST(account_groups.priority, EXCLUDED.priority);

        INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload, dedup_key)
        VALUES (
            'group_changed',
            NULL,
            target_group_id,
            NULL,
            'scheduler_outbox:runtime-bind-codex-subscription-upstreams:' || target_group_id::TEXT
        )
        ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING;
    END LOOP;
END $$;

COMMIT;
`.trimStart()
}

export const buildSummary = (opts = {}) => {
  const groupNames = normalizeGroupNames(opts.groupNames || DEFAULTS.groupNames)
  const pgContainer = assertNonBlank(opts.pgContainer || DEFAULTS.pgContainer, 'pg-container')
  const accountName = assertNonBlank(opts.accountName || DEFAULTS.accountName, 'account-name')
  return [
    `mode: ${opts.apply ? 'apply' : 'dry-run'}`,
    `postgres container: ${pgContainer}`,
    `account: ${accountName}`,
    `groups: ${groupNames.join(', ')}`,
  ].join('\n')
}

const printHelp = () => {
  console.log(`用法：
  node scripts/bind-codex-subscription-upstreams.mjs
  node scripts/bind-codex-subscription-upstreams.mjs --apply

参数：
  --apply                  写入 PostgreSQL；默认只 dry-run
  --dry-run                只校验并输出摘要
  --pg-container=NAME      PostgreSQL 容器名，默认 sub2api-candidate-postgres
  --account-name=NAME      上游账号名，默认 cliproxy-local-openai
  --group=NAME             要绑定的 group，可重复；默认 codex-pool-69-usd 与 codex-pool-89-usd`)
}

const runPsql = (sql, opts) => {
  const result = spawnSync(
    'docker',
    ['exec', '-i', opts.pgContainer, 'psql', '-U', 'sub2api', '-d', 'sub2api', '-v', 'ON_ERROR_STOP=1'],
    {
      input: sql,
      encoding: 'utf8',
      env: {
        ...process.env,
        PATH: `/Applications/Docker.app/Contents/Resources/bin:${process.env.PATH || ''}`,
      },
      maxBuffer: 1024 * 1024 * 16,
    },
  )

  if (result.status !== 0) {
    throw new Error(`绑定 Codex 套餐上游失败：exit=${result.status}\nstdout:\n${result.stdout || ''}\nstderr:\n${result.stderr || ''}`)
  }

  return result.stdout || ''
}

export const runCli = (argv, env = process.env) => {
  void env
  const opts = parseArgs(argv)
  if (opts.help) {
    printHelp()
    return
  }

  const sql = buildRuntimeSql(opts)
  console.log(buildSummary(opts))

  if (!opts.apply) {
    console.log('dry-run: no database changes were applied')
    return
  }

  const output = runPsql(sql, opts)
  if (output.trim()) console.log(output.trim())
  console.log('applied: Codex subscription upstream bindings updated')
}

const isMain = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]

if (isMain) {
  try {
    runCli(process.argv.slice(2), process.env)
  } catch (error) {
    console.error(error.message)
    process.exit(1)
  }
}
