import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightBlog from 'starlight-blog';

export default defineConfig({
  site: 'https://cookwithgasoline.com',
  integrations: [
    starlight({
      title: 'Gasoline MCP',
      description:
        'Browser automation, debugging, demo recording, and interactive web development for MCP-compatible AI agents.',
      logo: {
        src: './src/assets/logo.png',
        alt: 'Gasoline MCP',
      },
      favicon: '/images/logo.png',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/brennhill/gasoline',
        },
      ],
      customCss: ['./src/styles/custom.css'],
      components: {
        Footer: './src/components/Footer.astro',
      },
      plugins: [
        starlightBlog({
          title: 'Blog',
          prefix: 'blog',
          postsPerPage: 50,
          authors: {
            brenn: {
              name: 'Brenn Hill',
            },
            brennhill: {
              name: 'Brenn Hill',
            },
          },
        }),
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Downloads', slug: 'downloads' },
            { label: 'Quick Start', slug: 'getting-started' },
            { label: 'Features', slug: 'features' },
            { label: 'Discover Workflows', slug: 'discover-workflows' },
            { label: 'Articles', slug: 'articles' },
          ],
        },
        {
          label: 'MCP Integration',
          items: [
            { label: 'Overview', slug: 'mcp-integration' },
            { label: 'Claude Code', slug: 'mcp-integration/claude-code' },
            { label: 'Cursor', slug: 'mcp-integration/cursor' },
            { label: 'Windsurf', slug: 'mcp-integration/windsurf' },
            { label: 'Claude Desktop', slug: 'mcp-integration/claude-desktop' },
            { label: 'Zed', slug: 'mcp-integration/zed' },
            { label: 'Gemini CLI', slug: 'mcp-integration/gemini' },
            { label: 'OpenCode', slug: 'mcp-integration/opencode' },
            { label: 'Antigravity', slug: 'mcp-integration/antigravity' },
          ],
        },
        {
          label: 'Tool Reference',
          items: [
            { label: 'Reference Overview', slug: 'reference' },
            { label: 'observe()', slug: 'reference/observe' },
            { label: 'analyze()', slug: 'reference/analyze' },
            { label: 'generate()', slug: 'reference/generate' },
            { label: 'configure()', slug: 'reference/configure' },
            { label: 'interact()', slug: 'reference/interact' },
            { label: 'Script Execution', slug: 'execute-scripts' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Product Demos', slug: 'guides/product-demos' },
            { label: 'Demo Scripts', slug: 'guides/demo-scripts' },
            { label: 'Debug Web Apps', slug: 'guides/debug-webapps' },
            { label: 'Security Auditing', slug: 'guides/security-auditing' },
            { label: 'Performance', slug: 'guides/performance' },
            { label: 'Accessibility', slug: 'guides/accessibility' },
            { label: 'WebSocket Debugging', slug: 'guides/websocket-debugging' },
            { label: 'API Validation', slug: 'guides/api-validation' },
            { label: 'Noise Filtering', slug: 'guides/noise-filtering' },
            { label: 'Resilient UAT Scripts', slug: 'guides/resilient-uat' },
            { label: 'Replace Selenium', slug: 'guides/replace-selenium' },
            { label: 'Automate & Notify', slug: 'guides/automate-and-notify' },
          ],
        },
        {
          label: 'More',
          items: [
            { label: 'Architecture', slug: 'architecture' },
            { label: 'Security', slug: 'security' },
            { label: 'Troubleshooting', slug: 'troubleshooting' },
            { label: 'Agent Install Guide', slug: 'agent-install-guide' },
            { label: 'Alternatives', slug: 'alternatives' },
          ],
        },
      ],
    }),
  ],
});
