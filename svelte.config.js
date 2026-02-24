import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

const dev = process.argv.includes('dev');

function normalizeBasePath(value) {
	const raw = (value ?? '').trim();
	if (!raw || raw === '/') return '';

	const withLeadingSlash = raw.startsWith('/') ? raw : `/${raw}`;
	const withoutTrailingSlash = withLeadingSlash.replace(/\/+$/g, '');

	return withoutTrailingSlash === '/' ? '' : withoutTrailingSlash;
}

const productionBasePath = normalizeBasePath(process.env.PUBLIC_BASE_PATH ?? '/GTL-web');

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// Consult https://kit.svelte.dev/docs/integrations#preprocessors
	// for more information about preprocessors
	preprocess: vitePreprocess(),

	kit: {
		// /docs because of github pages
		adapter: adapter({
			pages: 'docs'
		}),

		paths: {
			base: dev ? '' : productionBasePath
		}
	}
};

export default config;
