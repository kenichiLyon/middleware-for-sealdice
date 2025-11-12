import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import StatusBadge from './components/StatusBadge.vue'
import Steps from './components/Steps.vue'
import Tabs from './components/Tabs.vue'
import { enhanceAppWithTabs } from 'vitepress-plugin-tabs/client'

export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('StatusBadge', StatusBadge)
    app.component('Steps', Steps)
    app.component('Tabs', Tabs)
    enhanceAppWithTabs(app)
  }
} satisfies Theme

