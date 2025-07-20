import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
import { resolve } from 'path';

export default defineConfig({
	plugins: [tailwindcss()],
	build: {
		rollupOptions: {
			input: {
				main: resolve(__dirname, 'index.html'),
				inspector: resolve(__dirname, 'inspector/index.html'),
			},
			output: {
				// JS files for entries
				entryFileNames: info => {
					if (info.name === 'inspector') {
						return 'inspector/[name].js';
					}
					return '[name].js';
				},
				// JS files for code-split chunks
				chunkFileNames: 'assets/[name]-[hash].js',
				// Other assets like CSS, images
				assetFileNames: 'assets/[name]-[hash][extname]',
			},
		},
	},
});
