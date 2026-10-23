import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { legacyCharacterModuleAliases } from './config/legacy-module-paths.ts'
import { playerBoundary } from './config/player-boundary.ts'
import { fileURLToPath } from 'node:url'

export default defineConfig(({ mode }) => {
  if (mode === 'demo-baseline' || mode === 'demo-protected')
    throw Error(
      'Client-rendered demo builds are retired. Use npm run build for the backend-only player.',
    )
  return {
    plugins: [vue(), playerBoundary()],
    define: { __SYNTELLE_DEMO_BUILD__: 'false' },
    resolve: { alias: legacyCharacterModuleAliases },
    build: {
      sourcemap: false,
      ...(mode === 'stream-player' ? { outDir: 'dist-stream-player' } : {}),
    },
    preview: { host: '127.0.0.1', port: 4173, strictPort: true },
    server: {
      fs: { strict: true, allow: [fileURLToPath(new URL('.', import.meta.url))] },
      host: '127.0.0.1',
      port: 5173,
      strictPort: true,
      proxy: {
        '/api/v1/speech': {
          target:
            process.env.SPEECH_PROXY_TARGET ??
            process.env.API_PROXY_TARGET ??
            'http://127.0.0.1:8082',
        },
        '/api': { target: process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:8080', ws: true },
      },
    },
  }
})
