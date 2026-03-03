import { getCollection } from 'astro:content'

const siteBase = 'https://cookwithgasoline.com'

interface Options {
  includeHtml?: boolean
}

/**
 * Build canonical markdown (and optionally HTML) URLs for every doc and blog page.
 */
export const getAllMarkdownPaths = async ({ includeHtml = false }: Options = {}): Promise<string[]> => {
  const docs = await getCollection('docs')
  const markdownUrls = docs.map((entry) => {
    const slugPath = entry.slug === '' ? 'index' : entry.slug
    return new URL(`/markdown/${slugPath}.md`, siteBase).toString()
  })

  if (!includeHtml) return markdownUrls

  const htmlUrls = docs.map((entry) => {
    const slugPath = entry.slug === '' ? '/' : `/${entry.slug}/`
    return new URL(slugPath, siteBase).toString()
  })

  return [...markdownUrls, ...htmlUrls]
}
