import type { APIRoute } from 'astro'
import { getCollection } from 'astro:content'
import { resolveDocSlug } from '../utils/contentSlugs'
import { siteReleaseChannel, siteVersionLabel } from '../utils/siteVersion'

export const prerender = true

const contentSignal = 'ai-train=yes, search=yes, ai-input=yes'

function toYamlString(value: unknown) {
  const text = String(value ?? '')
    .replace(/\r?\n/g, ' ')
    .replace(/'/g, "''")
    .trim()
  return `'${text}'`
}

function renderFrontmatter(entry: any) {
  const title = entry.data?.title ?? 'Strum'
  const description = entry.data?.description ?? entry.data?.summary ?? ''
  return `---\ntitle: ${toYamlString(title)}\ndescription: ${toYamlString(description)}\ncanonical: https://usestrum.dev/\ndocs_version: ${toYamlString(siteVersionLabel)}\ndocs_channel: ${toYamlString(siteReleaseChannel)}\n---`
}

export const GET: APIRoute = async () => {
  const docs = await getCollection('docs')
  const entry = docs.find((doc) => resolveDocSlug(doc) === '')

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
