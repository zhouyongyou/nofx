#!/usr/bin/env node
const { spawnSync } = require('child_process')
const fs = require('fs')
const path = require('path')

// 🔧 修復：在 Docker 或 CI 環境中跳過
if (process.env.CI || process.env.DOCKER_BUILD || process.env.HUSKY_INSTALL !== '1') {
  console.log('[husky] Skip install (CI/Docker environment or HUSKY_INSTALL not set)')
  process.exit(0)
}

// 🔧 修復：檢查 .git 目錄是否存在（Docker 中通常沒有）
const gitDir = path.join(__dirname, '../../.git')
if (!fs.existsSync(gitDir)) {
  console.log('[husky] Skip install (.git directory not found)')
  process.exit(0)
}

const result = spawnSync('npx', ['husky'], {
  stdio: 'inherit',
  shell: true,
})
process.exit(result.status ?? 0)
