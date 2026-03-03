import type { APIRoute } from 'astro'
import { getAllMarkdownPaths } from '../utils/markdownPaths'

export const prerender = true

export const GET: APIRoute = async () => {
  const urls = await getAllMarkdownPaths({ includeHtml: true })
  return new Response(urls.join('\n') + '\n', {
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Content-Signal': 'ai-train=yes, search=yes, ai-input=yes'
    }
  })
}
