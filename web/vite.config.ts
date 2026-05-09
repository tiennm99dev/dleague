import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
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
