import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const deployDir = path.dirname(fileURLToPath(import.meta.url))
const composeFiles = [
  'docker-compose.yml',
  'docker-compose.local.yml',
  'docker-compose.dev.yml',
  'docker-compose.candidate.yml',
]

const env = {
  ...process.env,
  POSTGRES_PASSWORD: 'compose-test-postgres-password',
  REDIS_PASSWORD: 'compose-test-redis-password',
  CANDIDATE_IMAGE: 'sub2api-candidate:test',
  JWT_SECRET: 'compose-test-jwt-secret',
  TOTP_ENCRYPTION_KEY: '00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff',
}

const fail = (message) => {
  throw new Error(message)
}

const renderCompose = (composeFile, renderEnv) => {
  const rendered = execFileSync(
    'docker',
    ['compose', '-f', composeFile, 'config', '--format', 'json'],
    { cwd: deployDir, env: renderEnv, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] },
  )
  return JSON.parse(rendered)
}

const getRedis = (composeFile, config) => {
  const redisEntries = Object.entries(config.services).filter(([name]) => name.includes('redis'))
  if (redisEntries.length !== 1) {
    fail(`${composeFile}: Redis 服务数量应为 1，实际为 ${redisEntries.length}`)
  }
  return redisEntries[0][1]
}

for (const composeFile of composeFiles) {
  const redis = getRedis(composeFile, renderCompose(composeFile, env))
  const command = redis.command
  if (!Array.isArray(command) || command.length !== 3 || command[0] !== 'sh' || command[1] !== '-c') {
    fail(`${composeFile}: Redis command 必须使用 [sh, -c, script]`)
  }

  const script = command[2].trim()
  if (!script.startsWith('exec redis-server ')) {
    fail(`${composeFile}: Redis 启动脚本必须以 exec redis-server 开头`)
  }
  if (!script.includes('$${REDIS_PASSWORD:+--requirepass "$$REDIS_PASSWORD"}')) {
    fail(`${composeFile}: Redis 启动脚本必须在容器内按需启用 requirepass`)
  }
  if (redis.environment?.REDIS_PASSWORD !== env.REDIS_PASSWORD) {
    fail(`${composeFile}: Redis 容器必须接收 REDIS_PASSWORD`)
  }
  if (redis.environment?.REDISCLI_AUTH !== env.REDIS_PASSWORD) {
    fail(`${composeFile}: redis-cli 必须使用相同密码`)
  }

  const redisWithoutPassword = getRedis(
    composeFile,
    renderCompose(composeFile, {...env, REDIS_PASSWORD: ''}),
  )
  const healthcheck = redisWithoutPassword.healthcheck?.test
  const healthcheckScript = Array.isArray(healthcheck) ? healthcheck.join(' ') : ''
  if (!healthcheckScript.includes('env -u REDISCLI_AUTH redis-cli ping')) {
    fail(`${composeFile}: 空密码健康检查必须取消 REDISCLI_AUTH`)
  }
}

console.log(`Redis Compose 验证通过：${composeFiles.length} 个文件`)
