import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		adapter: adapter({
			// Output directory for static files
			pages: 'build',
			assets: 'build',
			fallback: 'index.html', // SPA mode fallback
			precompress: false,
			strict: true
		})
	}
};

export default config;
