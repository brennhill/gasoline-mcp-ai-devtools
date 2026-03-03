import { getCollection } from 'astro:content'
import { resolveDocSlug } from './contentSlugs'

const siteBase = 'https://cookwithgasoline.com'

interface Options {
  includeHtml?: boolean
  includeLegacyMarkdown?: boolean
}

const toAgentMarkdownPath = (slug: string) => (slug === '' ? '/index.md' : `/${slug}.md`)
const toLegacyMarkdownPath = (slug: string) => `/markdown/${slug === '' ? 'index' : slug}.md`
const toHtmlPath = (slug: string) => (slug === '' ? '/' : `/${slug}/`)

/**
 * Build canonical markdown (and optionally HTML) URLs for every docs and blog page.
 */
export const getAllMarkdownPaths = async ({
  includeHtml = false,
  includeLegacyMarkdown = false
}: Options = {}): Promise<string[]> => {
  const docs = await getCollection('docs')
  const slugs = docs.map((entry) => resolveDocSlug(entry))

  const markdownUrls = slugs.map((slug) => new URL(toAgentMarkdownPath(slug), siteBase).toString())

  const urls = [...markdownUrls]

  if (includeLegacyMarkdown) {
    urls.push(...slugs.map((slug) => new URL(toLegacyMarkdownPath(slug), siteBase).toString()))
  }

  if (includeHtml) {
    urls.push(...slugs.map((slug) => new URL(toHtmlPath(slug), siteBase).toString()))
  }

  return Array.from(new Set(urls))
}
