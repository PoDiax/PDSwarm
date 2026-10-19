import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';
import { readFileSync } from 'fs';

const pkg = JSON.parse(readFileSync(path.resolve(import.meta.dirname, 'package.json'), 'utf-8'));

function optimizeLucideImports() {
  const lucideDir = path.resolve(import.meta.dirname, 'node_modules/@lucide/svelte/dist');
  let iconMap: Map<string, string> | null = null;

  function getMap() {
    if (iconMap) return iconMap;
    iconMap = new Map();
    try {
      const indexContent = readFileSync(path.join(lucideDir, 'icons/index.js'), 'utf-8');
      for (const match of indexContent.matchAll(/export\s*\{\s*default\s+as\s+([A-Za-z0-9_]+)\s*\}\s*from\s*'([^']+)';/g)) {
        const iconName = match[2].replace(/^\.\//, '').replace(/\.svelte$/, '');
        iconMap.set(match[1], `@lucide/svelte/icons/${iconName}`);
      }
      const aliasesContent = readFileSync(path.join(lucideDir, 'aliases/aliases.js'), 'utf-8');
      for (const match of aliasesContent.matchAll(/export\s*\{[\s\S]*?default\s+as\s+([A-Za-z0-9_]+)\s*\}\s*from\s*'([^']+)';/g)) {
        const iconName = match[2].replace(/^\.\.\/icons\//, '').replace(/\.js$/, '');
        iconMap.set(match[1], `@lucide/svelte/icons/${iconName}`);
      }
    } catch {
      // fallback to standard resolution if node_modules structure differs
    }
    return iconMap;
  }

  return {
    name: 'optimize-lucide-imports',
    enforce: 'pre' as const,
    transform(code: string, id: string) {
      if (!id.includes('/src/') || (!id.endsWith('.svelte') && !id.endsWith('.ts'))) {
        return null;
      }
      if (!code.includes('@lucide/svelte')) {
        return null;
      }
      const map = getMap();
      const newCode = code.replace(/import\s*\{([^}]+)\}\s*from\s*['"]@lucide\/svelte['"];?/g, (_, namesStr) => {
        const rawNames = namesStr.split(',').map((s: string) => s.trim()).filter(Boolean);
        const directImports: string[] = [];
        const remaining: string[] = [];
        for (const item of rawNames) {
          const parts = item.split(/\s+as\s+/);
          const name = parts[0].trim();
          const alias = parts[1] ? parts[1].trim() : name;
          const target = map.get(name);
          if (target) {
            directImports.push(`import ${alias} from '${target}';`);
          } else {
            remaining.push(item);
          }
        }
        if (remaining.length > 0) {
          directImports.push(`import { ${remaining.join(', ')} } from '@lucide/svelte';`);
        }
        return directImports.join('\n');
      });
      return { code: newCode, map: null };
    },
  };
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    optimizeLucideImports(),
    tailwindcss(),
    svelte(),
  ],
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  resolve: {
    alias: {
      $lib: path.resolve(import.meta.dirname, './src/lib'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('xterm')) {
            return 'xterm';
          }
          if (id.includes('/src/lib/components/Terminal.svelte')) {
            return 'terminal';
          }
          if (id.includes('node_modules')) {
            return 'vendor';
          }
          if (id.includes('/src/routes/')) {
            const match = id.match(/\/src\/routes\/([A-Za-z0-9_-]+)\.svelte/);
            if (match) {
              return `route-${match[1].toLowerCase()}`;
            }
          }
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
});
