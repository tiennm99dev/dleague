import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// Shared SvelteKit setup. vite/config.dev.mjs and vite/config.prod.mjs merge
// their own options on top; svelte-check and vitest read this file directly.
export default defineConfig({
	plugins: [
		sveltekit({
			preprocess: vitePreprocess(),
			adapter: adapter({ precompress: false, fallback: 'index.html' })
		})
	]
});
