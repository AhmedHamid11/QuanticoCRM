import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		proxy: {
			'/api/v1': {
				target: 'http://localhost:8080',
				changeOrigin: true
			}
		}
	},
	build: {
		rollupOptions: {
			onwarn(warning, warn) {
				// Suppress Svelte a11y and state warnings
				if (warning.message?.includes('a11y') ||
					warning.message?.includes('state_referenced_locally') ||
					warning.message?.includes('non_reactive_update')) {
					return;
				}
				warn(warning);
			}
		}
	}
});
