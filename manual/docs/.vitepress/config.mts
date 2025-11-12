import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'zh-CN',
  title: 'middleware-for-sealdice',
  description: 'Sealdice 中间件手册',
  lastUpdated: true,
  cleanUrls: true,
  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: '指南', link: '/guide/overview' },
      { text: '参考', link: '/reference/onebot-v11' },
      { text: 'FAQ', link: '/faq/troubleshooting' }
    ],
    sidebar: [
      {
        text: '指南',
        items: [
          { text: '简介', link: '/guide/overview' },
          { text: '快速开始', link: '/guide/quick-start' },
          { text: '安装', link: '/guide/install&deploy' },
        ]
      },
      {
        text: '参考',
        items: [
          { text: 'OneBot V11', link: '/reference/onebot-v11' }
        ]
      },
      {
        text: 'FAQ',
        items: [
          { text: '故障排查', link: '/faq/troubleshooting' },
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
