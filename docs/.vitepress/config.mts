import { withMermaid } from "vitepress-plugin-mermaid";
import { fileURLToPath, URL } from 'node:url'

// https://vitepress.dev/reference/site-config
export default withMermaid({
  title: "AnyApplication controller",
  description: "AnyApplication Controller for Decentralized Control Plane",
  base: "/anyapplication-controller/", 
  head: [['link', { rel: 'icon', href: 'https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller/refs/heads/main/docs/assets/dcp-log.png' }]],
  vite: {
      resolve: {
          alias: [
              {
                  find: /^.*\/VPFooter\.vue$/,
                  replacement: fileURLToPath(
                      new URL('./theme/components/VPFooter.vue', import.meta.url)
                  )
              },
          ]
      }
  },
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Concepts', link: '/concepts' },
      { text: 'Usage', link: '/usage' },
    ],

    editLink: {
      pattern: 'https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller/blob/main/docs/:path',
      text: 'Edit this page on GitHub'
    },

    logo: {
        src: 'https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller/refs/heads/main/docs/assets/dcp-log.png',
        width: 24,
        height: 24
    },

    search: {
      provider: 'local'
    },

    sidebar: [
        {
        items: [
          { text: 'Quick Start', link: '/usage/installation' },
          { text: 'Architecture', link: '/architecture' },
          { text: 'API Reference', link: '/api-reference/anyapplication' },
        ]
      },
      {
        text: 'Developer Guide',
        collapsed: false,
        items: [
          { text: 'Local Dev Setup', link: '/development/setup' },
        ]
      }
    ],

    footer: {
      message: 'Released under the Apache License 2.0.',
      copyright: 'Copyright © 2025-present HIRO-MicroDataCenters BV'
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/HIRO-MicroDataCenters-BV/anyapplication-controller' }
    ],
  }
})