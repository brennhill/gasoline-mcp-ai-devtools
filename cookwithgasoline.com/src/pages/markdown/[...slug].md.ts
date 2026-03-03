import type { APIRoute, GetStaticPaths } from 'astro'
import { getCollection } from 'astro:content'

export const prerender = true

const contentSignal = 'ai-train=yes, search=yes, ai-input=yes'

const slugToPath = (slug: string[] | undefined) => {
  if (!slug || slug.length === 0) return 'index'
  return slug.join('/')
}

function renderFrontmatter(entry: any) {
  const title = entry.data?.title ?? 'Gasoline MCP'
  const description = entry.data?.description ?? entry.data?.summary ?? ''
  return `---\ntitle: ${title}\ndescription: ${description}\ncanonical: https://cookwithgasoline.com/${entry.slug}\n---`
}

export const GET: APIRoute = async ({ params }) => {
  const docs = await getCollection('docs')
  const slugPath = slugToPath(params.slug as string[] | undefined)

  const entry = docs.find((doc) => (doc.slug === '' ? 'index' : doc.slug) === slugPath)
  if (!entry) {
    return new Response('# Not found\n', { status: 404, headers: { 'Content-Type': 'text/markdown; charset=utf-8' } })
  }

  const fm = renderFrontmatter(entry)
  const body = typeof entry.body === 'string' ? entry.body : await entry.body()

  return new Response(`${fm}\n\n${body}\n`, {
    headers: {
      'Content-Type': 'text/markdown; charset=utf-8',
      'Content-Signal': contentSignal
    }
  })
}

export const getStaticPaths: GetStaticPaths = async () => {
  const docs = await getCollection('docs')
  return docs.map((doc) => {
    const parts = doc.slug ? doc.slug.split('/') : undefined
    return { params: { slug: parts } }
  })
}
