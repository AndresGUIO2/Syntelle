import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@fontsource-variable/dm-sans'
import './styles.css'
import App from './App.vue'

createApp(App).use(createPinia()).mount('#app')
