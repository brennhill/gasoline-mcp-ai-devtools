import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';

export const prerender = true;

const contentSignal = 'ai-train=yes, search=yes, ai-input=yes';

export const GET: APIRoute = async () => {
  const docs = await getCollection('docs');
  const landing = docs.find((doc) => doc.slug === '' || doc.slug === 'index');

  const body = landing?.body ? (typeof landing.body === 'string' ? landing.body : await landing.body()) : '# Gasoline MCP\n\nAI-native debugging and replay for web apps.';

  return new Response(body, {
    headers: {
      'Content-Type': 'text/markdown; charset=utf-8',
      'Content-Signal': contentSignal,
    },
  });
};
