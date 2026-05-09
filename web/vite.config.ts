import { sveltekit } from '@sveltejs/kit/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit(), svelteTesting()],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'happy-dom',
		alias: {
			$lib: '/config/workspace/tiennm99/dleague/web/src/lib'
		}
	},
	server: {
		proxy: {
			// Forward /ws upgrades to the Go server running on :8080.
			// ws:true is required so Vite rewrites the upgrade handshake.
			'/ws': {
				target: 'ws://localhost:8080',
				ws: true
			},
			// Forward /health GETs to Go server for dev smoke tests.
			'/health': 'http://localhost:8080'
		}
	}
});
