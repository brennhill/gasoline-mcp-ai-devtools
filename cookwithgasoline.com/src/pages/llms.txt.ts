import type { APIRoute } from 'astro'
import { getAllMarkdownPaths } from '../utils/markdownPaths'
import { siteReleaseChannel, siteVersionLabel } from '../utils/siteVersion'

export const prerender = true

export const GET: APIRoute = async () => {
  const urls = await getAllMarkdownPaths({ includeHtml: false })
  const lines = [`# docs_version: ${siteVersionLabel} (${siteReleaseChannel})`, ...urls]
  return new Response(lines.join('\n') + '\n', {
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Content-Signal': 'ai-train=yes, search=yes, ai-input=yes'
    }
  })
}
