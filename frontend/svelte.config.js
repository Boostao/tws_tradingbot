import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
        compilerOptions: {
                runes: false
        },
        kit: {
                adapter: adapter({
                    fallback: 'index.html',
                    pages: 'dist',
                    assets: 'dist',
                    precompress: false,
                    strict: false
                })
        }
};

export default config;
