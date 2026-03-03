interface ContentEntryLike {
  id?: unknown;
  slug?: unknown;
  data?: Record<string, unknown>;
}

/**
 * Resolve a stable route slug from Astro content entries.
 * Falls back from `slug` to `data.slug` to derived value from `id`.
 */
export function resolveDocSlug(entry: ContentEntryLike): string {
  if (typeof entry.slug === 'string') {
    return entry.slug;
  }

  if (typeof entry.data?.slug === 'string') {
    return entry.data.slug;
  }

  if (typeof entry.id === 'string' && entry.id.length > 0) {
    const normalized = entry.id
      .replace(/\\/g, '/')
      .replace(/^docs\//, '')
      .replace(/\.(md|mdx)$/i, '')
      .replace(/\/index$/i, '');

    if (normalized === 'index') {
      return '';
    }

    return normalized;
  }

  return '';
}
