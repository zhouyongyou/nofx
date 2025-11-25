import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { execSync } from 'child_process'

// Get current git branch name at build time
function getGitBranch(): string {
  // 優先使用環境變數（Docker 構建時使用）
  if (process.env.VITE_GIT_BRANCH) {
    return process.env.VITE_GIT_BRANCH
  }

  // 本地開發時從 git 讀取
  try {
    return execSync('git rev-parse --abbrev-ref HEAD').toString().trim()
  } catch {
    return 'unknown'
  }
}

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  define: {
    // Inject git branch name as global constant
    __GIT_BRANCH__: JSON.stringify(getGitBranch()),
  },
})
