import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@fontsource-variable/dm-sans'
import './theme.css'
import './styles.css'
const embedded = /^\/embed\/([a-zA-Z0-9_-]+)\/?$/.exec(location.pathname)
if (embedded) {
  const { default: WidgetHost } = await import('./features/widget/WidgetHost.vue')
  createApp(WidgetHost, { widgetId: embedded[1] }).use(createPinia()).mount('#app')
} else {
  const { default: App } = await import('./App.vue')
  createApp(App).use(createPinia()).mount('#app')
}
