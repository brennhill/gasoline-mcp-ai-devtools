import type { APIRoute } from 'astro'
import { SEARCH_SYNONYMS } from '../data/searchSynonyms'
import { siteReleaseChannel, siteVersionLabel } from '../utils/siteVersion'

export const prerender = true

export const GET: APIRoute = async () => {
  return new Response(
    JSON.stringify(
      {
        docs_version: siteVersionLabel,
        docs_channel: siteReleaseChannel,
        synonyms: SEARCH_SYNONYMS
      },
      null,
      2
    ),
    {
      headers: {
        'Content-Type': 'application/json; charset=utf-8',
        'Cache-Control': 'public, max-age=3600'
      }
    }
  )
}
