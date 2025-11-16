#!/usr/bin/env node
const { spawnSync } = require('child_process')

if (process.env.HUSKY_INSTALL !== '1') {
  console.log('[husky] Skip install (set HUSKY_INSTALL=1 to enable)')
  process.exit(0)
}

const result = spawnSync('npx', ['husky'], {
  stdio: 'inherit',
  shell: true,
})
process.exit(result.status ?? 0)
