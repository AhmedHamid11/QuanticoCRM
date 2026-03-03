import adapter from '@sveltejs/adapter-vercel';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter({ runtime: 'nodejs22.x' })
	},
	warningFilter: (warning) => {
		// Suppress a11y warnings
		if (warning.code?.startsWith('a11y')) return false;
		// Suppress state warnings
		if (warning.code === 'state_referenced_locally') return false;
		if (warning.code === 'non_reactive_update') return false;
		return true;
	}
};

export default config;
