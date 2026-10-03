import { defineConfig, mergeConfig } from 'vite';
import baseConfig from '../vite.config.js';

// https://vitejs.dev/config/
export default mergeConfig(baseConfig, defineConfig({
	server: {
		port: 8080
	}
}));
