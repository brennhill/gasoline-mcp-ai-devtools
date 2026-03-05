import { defineCollection, z } from 'astro:content'
import { docsLoader } from '@astrojs/starlight/loaders'
import { docsSchema } from '@astrojs/starlight/schema'
import { blogSchema } from 'starlight-blog/schema'

export const collections = {
  docs: defineCollection({
    loader: docsLoader(),
    schema: docsSchema({
      extend: (context) =>
        blogSchema(context).extend({
          last_verified_version: z.string().optional(),
          last_verified_date: z.union([z.string(), z.date()]).optional(),
          normalized_tags: z.array(z.string()).optional(),
          related_guides: z.array(z.string()).optional()
        })
    })
  })
}
