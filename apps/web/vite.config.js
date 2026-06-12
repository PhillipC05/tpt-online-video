import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), '');
    const apiTarget = env.VITE_API_PROXY || 'http://localhost:8080';
    const ipfsTarget = env.VITE_IPFS_API_PROXY || 'http://localhost:5001';
    return {
        plugins: [react()],
        server: {
            port: 5173,
            proxy: {
                '/api': {
                    target: apiTarget,
                    changeOrigin: true,
                },
                '/healthz': {
                    target: apiTarget,
                    changeOrigin: true,
                },
                '/readyz': {
                    target: apiTarget,
                    changeOrigin: true,
                },
                '/ipfs': {
                    target: ipfsTarget,
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/ipfs/, '/api/v0'),
                },
            },
        },
    };
});
