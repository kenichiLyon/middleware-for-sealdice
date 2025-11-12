import { defineConfig } from 'vitepress'
import { tabsMarkdownPlugin } from 'vitepress-plugin-tabs'

export default defineConfig({
  lang: 'zh-CN',
  title: 'middleware-for-sealdice',
  description: 'Sealdice 中间件手册',
  lastUpdated: true,
  cleanUrls: true,
  base: '/middleware-for-sealdice/',
  markdown: {
    config: (md) => {
      md.core.ruler.before('normalize', 'tabs-compat', (state) => {
        state.src = state.src
          .replace(/:::\s+tabs/g, ':::tabs')
          .replace(/\n==\s+([^\n]+)/g, (m, t) => `\n== tab ${t}`)
          .replace(/keys:/g, 'key:')
      })
      md.use(tabsMarkdownPlugin)
    }
  },
  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: '指南', link: '/guide/overview' },
      { text: '参考', link: '/reference/about-project' },
      { text: 'FAQ', link: '/faq/faq' }
    ],
    sidebar: [
      {
        text: '指南',
        items: [
          { text: '简介', link: '/guide/overview' },
          { text: '快速开始', link: '/guide/quick-start' },
          { text: '安装部署', link: '/guide/install&deploy' }
        ]
      },
      {
        text: '参考',
        items: [
          { text: '关于项目', link: '/reference/about-project' }
        ]
      },
      {
        text: 'FAQ',
        items: [
          { text: '常见问题', link: '/faq/faq' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/vuejs/vitepress' }
    ],
    search: {
      provider: 'local'
    },
    footer: {
      message: '基于 VitePress 构建',
      copyright: '© 2025 middleware-for-sealdice'
    }
  }
})
