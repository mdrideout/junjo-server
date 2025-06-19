import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react-swc'
import viteJunjoPlugin from './vite-junjo-plugin'
import { analyzer } from 'vite-bundle-analyzer'

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), react(), viteJunjoPlugin(), analyzer()],
  server: {
    port: 5151,
    host: true,
    watch: {
      usePolling: true,
    },
    hmr: {
      clientPort: 5152, // must be mapped in docker compose to 5151
    },
    proxy: {
      '/api': {
        target: 'http://localhost:8080', // Assuming backend runs on 8080
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''), // Optional: if your backend doesn't expect /api prefix
      },
    },
  },
  build: {
    sourcemap: true,
  },
})
