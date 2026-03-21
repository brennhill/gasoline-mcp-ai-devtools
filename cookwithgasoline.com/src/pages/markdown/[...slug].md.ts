import type { APIRoute, GetStaticPaths } from 'astro'
import { getCollection } from 'astro:content'
import { resolveDocSlug } from '../../utils/contentSlugs'
import { siteReleaseChannel, siteVersionLabel } from '../../utils/siteVersion'
import { getRelatedGuides } from '../../data/relatedGuides'

export const prerender = true

const contentSignal = 'ai-train=yes, search=yes, ai-input=yes'

function toYamlString(value: unknown) {
  const text = String(value ?? '')
    .replace(/\r?\n/g, ' ')
    .replace(/'/g, "''")
    .trim()
  return `'${text}'`
}

function toYamlArray(values: string[]) {
  if (values.length === 0) return '[]'
  return `[${values.map((value) => toYamlString(value)).join(', ')}]`
}

const slugToPath = (slug: string | undefined) => slug || 'index'

function renderFrontmatter(entry: any) {
  const title = entry.data?.title ?? 'STRUM MCP'
  const description = entry.data?.description ?? entry.data?.summary ?? ''
  const resolvedSlug = resolveDocSlug(entry)
  const verifiedVersion = entry.data?.last_verified_version ?? siteVersionLabel
  const verifiedDate = entry.data?.last_verified_date ?? new Date().toISOString().slice(0, 10)
  const normalizedTags = Array.isArray(entry.data?.normalized_tags) ? entry.data.normalized_tags : []
  const relatedGuides = getRelatedGuides(resolvedSlug).map((guide) => guide.href)

  return `---\ntitle: ${toYamlString(title)}\ndescription: ${toYamlString(description)}\ncanonical: https://cookwithgasoline.com${resolvedSlug === '' ? '/' : `/${resolvedSlug}/`}\ndocs_version: ${toYamlString(siteVersionLabel)}\ndocs_channel: ${toYamlString(siteReleaseChannel)}\nlast_verified_version: ${toYamlString(verifiedVersion)}\nlast_verified_date: ${toYamlString(verifiedDate)}\nnormalized_tags: ${toYamlArray(normalizedTags)}\nrelated_guides: ${toYamlArray(relatedGuides)}\n---`
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
