import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import StatusBadge from './components/StatusBadge.vue'
import Steps from './components/Steps.vue'

export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('StatusBadge', StatusBadge)
    app.component('Steps', Steps)
  }
} satisfies Theme

