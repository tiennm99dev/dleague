import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter({
			// Output into web/dist so Go's FileServer can serve it directly.
			pages: 'dist',
			assets: 'dist',
			// SPA fallback: any unmatched path returns index.html so client-side
			// routing (/match/<token>, /play, etc.) works without a server rewrite.
			fallback: 'index.html'
		})
	}
};

export default config;
