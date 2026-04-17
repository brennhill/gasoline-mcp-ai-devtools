export type RelatedGuide = {
  label: string
  href: string
}

const defaults: RelatedGuide[] = [
  { label: 'Quick Start', href: '/getting-started/' },
  { label: 'Debug Web Apps', href: '/guides/debug-webapps/' },
  { label: 'Reference Overview', href: '/reference/' }
]

const byPrefix: Array<{ prefix: string; guides: RelatedGuide[] }> = [
  {
    prefix: 'reference/',
    guides: [
      { label: 'Quick Start', href: '/getting-started/' },
      { label: 'API Validation Guide', href: '/guides/api-validation/' },
      { label: 'Debug Web Apps', href: '/guides/debug-webapps/' }
    ]
  },
  {
    prefix: 'guides/',
    guides: [
      { label: 'Tool Reference', href: '/reference/' },
      { label: 'Discover Workflows', href: '/discover-workflows/' },
      { label: 'Articles Library', href: '/articles/' }
    ]
  },
  {
    prefix: 'articles/',
    guides: [
      { label: 'Debug Web Apps', href: '/guides/debug-webapps/' },
      { label: 'API Validation Guide', href: '/guides/api-validation/' },
      { label: 'Observe Reference', href: '/reference/observe/' }
    ]
  },
  {
    prefix: 'mcp-integration/',
    guides: [
      { label: 'Downloads', href: '/downloads/' },
      { label: 'Quick Start', href: '/getting-started/' },
      { label: 'Troubleshooting', href: '/troubleshooting/' }
    ]
  }
]

const bySlug = new Map<string, RelatedGuide[]>([
  [
    'index',
    [
      { label: 'Quick Start', href: '/getting-started/' },
      { label: 'Discover Workflows', href: '/discover-workflows/' },
      { label: 'Reference Overview', href: '/reference/' }
    ]
  ],
  [
    'downloads',
    [
      { label: 'Quick Start', href: '/getting-started/' },
      { label: 'MCP Integration', href: '/mcp-integration/' },
      { label: 'Troubleshooting', href: '/troubleshooting/' }
    ]
  ],
  [
    'getting-started',
    [
      { label: 'Reference Overview', href: '/reference/' },
      { label: 'MCP Integration', href: '/mcp-integration/' },
      { label: 'Debug Web Apps', href: '/guides/debug-webapps/' }
    ]
  ],
  [
    'discover-workflows',
    [
      { label: 'Guides', href: '/guides/debug-webapps/' },
      { label: 'Articles Library', href: '/articles/' },
      { label: 'Reference Overview', href: '/reference/' }
    ]
  ]
])

function dedupe(guides: RelatedGuide[]): RelatedGuide[] {
  const out: RelatedGuide[] = []
  const seen = new Set<string>()
  for (const guide of guides) {
    if (seen.has(guide.href)) continue
    seen.add(guide.href)
    out.push(guide)
  }
  return out
}

export function getRelatedGuides(slug: string): RelatedGuide[] {
  if (!slug || slug === '404') return []

  if (bySlug.has(slug)) {
    return bySlug.get(slug) ?? []
  }

  const matched = byPrefix.find((entry) => slug.startsWith(entry.prefix))
  if (matched) {
    return dedupe(matched.guides)
  }

  return dedupe(defaults)
}
