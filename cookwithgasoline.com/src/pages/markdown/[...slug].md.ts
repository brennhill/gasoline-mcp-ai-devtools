import type { APIRoute, GetStaticPaths } from 'astro'
import { getCollection } from 'astro:content'
import { resolveDocSlug } from '../../utils/contentSlugs'

export const prerender = true

const contentSignal = 'ai-train=yes, search=yes, ai-input=yes'

function toYamlString(value: unknown) {
  const text = String(value ?? '')
    .replace(/\r?\n/g, ' ')
    .replace(/'/g, "''")
    .trim()
  return `'${text}'`
}

const slugToPath = (slug: string | undefined) => slug || 'index'

function renderFrontmatter(entry: any) {
  const title = entry.data?.title ?? 'Gasoline MCP'
  const description = entry.data?.description ?? entry.data?.summary ?? ''
  const resolvedSlug = resolveDocSlug(entry)

  return `---\ntitle: ${toYamlString(title)}\ndescription: ${toYamlString(description)}\ncanonical: https://cookwithgasoline.com${resolvedSlug === '' ? '/' : `/${resolvedSlug}/`}\n---`
}

export const GET: APIRoute = async ({ params }) => {
  const docs = await getCollection('docs')
  const slugPath = slugToPath(params.slug as string | undefined)
  const requestedSlug = slugPath === 'index' ? '' : slugPath

  const entry = docs.find((doc) => resolveDocSlug(doc) === requestedSlug)
  if (!entry) {
    return new Response('# Not found\n', {
      status: 404,
      headers: {
        'Content-Type': 'text/markdown; charset=utf-8',
        'Content-Signal': contentSignal
      }
    })
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

  return docs
    .map((doc) => resolveDocSlug(doc))
    .map((slug) => (slug === '' ? 'index' : slug))
    .map((slug) => ({ params: { slug } }))
}
