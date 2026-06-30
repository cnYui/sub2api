import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildRuntimeSql,
  buildSummary,
  parseArgs,
} from '../bind-codex-subscription-upstreams.mjs'

test('parseArgs defaults to dry-run candidate binding for 79 and 99 groups', () => {
  const opts = parseArgs([])

  assert.equal(opts.apply, false)
  assert.equal(opts.pgContainer, 'sub2api-candidate-postgres')
  assert.equal(opts.accountName, 'cliproxy-local-openai')
  assert.deepEqual(opts.groupNames, ['codex-pool-69-usd', 'codex-pool-89-usd'])
})

test('parseArgs allows overriding container, account, and groups', () => {
  const opts = parseArgs([
    '--apply',
    '--pg-container=sub2api-main-preview-postgres',
    '--account-name=staging-openai',
    '--group=codex-pool-69-usd',
    '--group=codex-pool-89-usd',
  ])

  assert.equal(opts.apply, true)
  assert.equal(opts.pgContainer, 'sub2api-main-preview-postgres')
  assert.equal(opts.accountName, 'staging-openai')
  assert.deepEqual(opts.groupNames, ['codex-pool-69-usd', 'codex-pool-89-usd'])
})

test('buildRuntimeSql binds groups to account by name and refreshes scheduler outbox', () => {
  const sql = buildRuntimeSql({
    accountName: 'cliproxy-local-openai',
    groupNames: ['codex-pool-69-usd', 'codex-pool-89-usd'],
  })

  assert.match(sql, /accounts/)
  assert.match(sql, /name = 'cliproxy-local-openai'/)
  assert.match(sql, /platform = 'openai'/)
  assert.match(sql, /status = 'active'/)
  assert.match(sql, /schedulable = true/)
  assert.match(sql, /codex-pool-69-usd/)
  assert.match(sql, /codex-pool-89-usd/)
  assert.match(sql, /INSERT INTO account_groups/)
  assert.match(sql, /priority, created_at/)
  assert.match(sql, /ON CONFLICT \(account_id, group_id\)/)
  assert.match(sql, /LEAST\(account_groups.priority, EXCLUDED.priority\)/)
  assert.match(sql, /INSERT INTO scheduler_outbox/)
  assert.match(sql, /'group_changed'/)
  assert.doesNotMatch(sql, /account_id\s*=\s*1/)
  assert.doesNotMatch(sql, /VALUES\s*\(\s*1\s*,/)
})

test('buildRuntimeSql quotes names safely', () => {
  const sql = buildRuntimeSql({
    accountName: "openai's account",
    groupNames: ["codex-pool-69-usd", "bad'group"],
  })

  assert.match(sql, /openai''s account/)
  assert.match(sql, /bad''group/)
})

test('buildSummary describes dry-run target without exposing credentials', () => {
  const summary = buildSummary({
    accountName: 'cliproxy-local-openai',
    groupNames: ['codex-pool-69-usd', 'codex-pool-89-usd'],
    pgContainer: 'sub2api-candidate-postgres',
    apply: false,
  })

  assert.match(summary, /mode: dry-run/)
  assert.match(summary, /postgres container: sub2api-candidate-postgres/)
  assert.match(summary, /account: cliproxy-local-openai/)
  assert.match(summary, /groups: codex-pool-69-usd, codex-pool-89-usd/)
})
