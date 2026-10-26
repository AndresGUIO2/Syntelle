import { readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import type { Plugin } from 'vite'

// Explicit public surface: adding a feature requires reviewing this boundary.
const publicSources = new Set([
  'main.ts',
  'App.vue',
  'styles.css',
  'components/AppHeader.vue',
  'components/AppIcon.vue',
  'components/ThemeToggle.vue',
  'components/LanguageSelector.vue',
  'stores/theme.ts',
  'stores/locale.ts',
  'locales/es.ts',
  'composables/usePageNavigation.ts',
  'features/pricing/PricingPage.vue',
  'features/characters/CreateCharacterPage.vue',
  ...['light-studio', 'dark-lounge', 'natural-space'].flatMap((name) => [
    `assets/backgrounds/${name}-thumb.webp`,
    `assets/backgrounds/${name}.webp`,
  ]),
  ...[
    'types.ts',
    'api.ts',
    'settingsCatalog.ts',
    'StreamWorkspace.vue',
    'composables/useRemoteStream.ts',
    'composables/useBackgroundVideo.ts',
    'backgroundCompositor.ts',
    'composables/useAccount.ts',
    'components/RemotePlayer.vue',
    'components/AccountDialog.vue',
    'components/ConversationHistory.vue',
    'components/PrivacyStorageNotice.vue',
    'components/StreamSettings.vue',
    'components/BehaviorSettings.vue',
    'components/AppearanceSettings.vue',
    'components/AdvancedSettings.vue',
    'components/customization.css',
    'components/ExpressionPreview.vue',
    'components/EmotionControls.vue',
  ].map((path) => `features/streaming/${path}`),
  ...[
    'types.ts',
    'components/ChatPanel.vue',
    'components/ConversationIcon.vue',
    'components/ConversationMessages.vue',
    'components/ConversationControls.vue',
    'components/MicrophoneControl.vue',
    'composables/useMicrophone.ts',
    'composables/useWorkspaceViewport.ts',
    'audio/capture.worklet.ts',
    'audio/resampler.ts',
    'audio/inputLevel.ts',
  ].map((path) => `features/conversation/${path}`),
])

export function isPrivatePlayerModule(id: string) {
  const path = id.replaceAll('\\', '/').split('?')[0]!
  if (path.includes('/node_modules/')) return false
  const index = path.indexOf('/src/')
  return index >= 0 && !publicSources.has(path.slice(index + 5))
}

export function playerBoundary(): Plugin {
  const root = fileURLToPath(new URL('../', import.meta.url))
  return {
    name: 'private-renderer-boundary',
    configureServer(server) {
      server.middlewares.use((request, response, next) => {
        let path: string
        try {
          path = decodeURIComponent((request.url ?? '').split('?')[0]!).replaceAll('\\', '/')
        } catch {
          response.statusCode = 400
          response.end('Invalid path')
          return
        }
        if (
          !isPrivatePlayerModule(path) &&
          !/(?:^|\/)(?:config|tests|scripts|docs|specs|\.tools|\.agents)(?:\/|$)/i.test(path) &&
          !/(?:^|\/)(?:vite\.config|package(?:-lock)?|tsconfig[^/]*|AGENTS|README)(?:\.|$)/i.test(
            path,
          )
        )
          return next()
        response.statusCode = 403
        response.end('Unavailable')
      })
    },
    generateBundle(_options, bundle) {
      for (const item of Object.values(bundle)) {
        if (item.type === 'chunk' && Object.keys(item.modules).some(isPrivatePlayerModule))
          throw Error('Unreviewed source cannot enter the public player build.')
        if (!isPublicOutput(item.fileName))
          throw Error('Unreviewed resource cannot enter the public player build.')
      }
    },
    writeBundle(options) {
      // Vite copies public/ without passing those files through generateBundle.
      const output = resolve(root, options.dir ?? 'dist')
      for (const file of readdirSync(output, { recursive: true, withFileTypes: true })) {
        if (file.isFile() && !isPublicOutput(file.name))
          throw Error(`Unreviewed public output: ${file.name}`)
      }
    },
  }
}

function isPublicOutput(name: string) {
  return (
    /\.(?:html|js|css|woff2|svg|txt)$/.test(name) ||
    /(?:^|\/)syntelle-poster\.jpg$/.test(name) ||
    /(?:^|\/)syntelle-(?:light-studio|dark-lounge|natural-space)-(?:portrait|mobile)\.webp$/.test(
      name,
    ) ||
    /(?:light-studio|dark-lounge|natural-space)(?:-thumb)?-[\w-]+\.webp$/.test(name)
  )
}
