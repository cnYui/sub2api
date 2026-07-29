import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const deployDir = dirname(fileURLToPath(import.meta.url));
const readDeployFile = (name) => readFileSync(resolve(deployDir, name), "utf8");
const readRepositoryFile = (name) => readFileSync(resolve(deployDir, "..", name), "utf8");
const serviceNames = (compose) => {
  const names = [];
  let inServices = false;

  for (const line of compose.split(/\r?\n/)) {
    if (line === "services:") {
      inServices = true;
      continue;
    }
    if (inServices && /^\S/.test(line)) {
      inServices = false;
    }
    const match = inServices && line.match(/^  ([a-z][a-z0-9-]+):\s*$/);
    if (match) {
      names.push(match[1]);
    }
  }

  return names;
};

const compose = readDeployFile("docker-compose.openai-billing-candidate.yml");
const envExample = readDeployFile(".env.openai-billing-candidate.local.example");
const sanitizeSQL = readDeployFile("sql/openai-billing-candidate-sanitize.sql");
const outerRouteSQL = readDeployFile("sql/openai-billing-candidate-outer-route.sql");
const lifecycleScript = readDeployFile("openai-billing-candidate.ps1");
const rootGitIgnore = readRepositoryFile(".gitignore");

const requiredServices = [
  "billing-outer",
  "billing-outer-postgres",
  "billing-outer-redis",
  "billing-inner",
  "billing-inner-postgres",
  "billing-inner-redis",
];

assert.deepEqual(serviceNames(compose).sort(), requiredServices.sort());
assert.match(compose, /127\.0\.0\.1:18081:8080/);
assert.match(compose, /127\.0\.0\.1:18087:8080/);
assert.doesNotMatch(compose, /sub2api-dev|sub2api-upstream-latest|nginx/);
assert.match(envExample, /^BILLING_OUTER_IMAGE=/m);
assert.match(envExample, /^BILLING_INNER_IMAGE=/m);
assert.match(envExample, /^OUTER_POSTGRES_PASSWORD=/m);
assert.match(envExample, /^INNER_POSTGRES_PASSWORD=/m);
assert.match(sanitizeSQL, /payment_enabled', 'false'/);
assert.match(outerRouteSQL, /http:\/\/billing-inner:8080\/v1/);
assert.match(outerRouteSQL, /schedulable = false/);
assert.match(lifecycleScript, /ValidateSet\('Backup', 'Restore', 'Up', 'Down', 'Verify'\)/);
assert.match(lifecycleScript, /sub2api-postgres-dev/);
assert.match(lifecycleScript, /sub2api-upstream-postgres/);
assert.match(lifecycleScript, /pg_restore --list/);
assert.match(lifecycleScript, /18080/);
assert.match(lifecycleScript, /18086/);
assert.match(rootGitIgnore, /^deploy\/openai-billing-candidate\/$/m);

console.log("OpenAI 计费候选环境静态校验通过");
