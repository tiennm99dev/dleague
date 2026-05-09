import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit()],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'node'
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
