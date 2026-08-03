import { registerPlugins } from '@/plugins'
import App from './App.vue'
import { createApp } from 'vue'
import router from './router'
import { installRuntimeGuards } from '@/security/antiDebug'

import 'unfonts.css'
import '@fontsource-variable/noto-sans-sc'

installRuntimeGuards()

const app = createApp(App)

registerPlugins(app)
app.use(router)
app.mount('#app')
