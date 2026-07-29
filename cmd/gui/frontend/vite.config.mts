// Plugins
import Components from 'unplugin-vue-components/vite'
import Vue from '@vitejs/plugin-vue'
import Vuetify, { transformAssetUrls } from 'vite-plugin-vuetify'
import Fonts from 'unplugin-fonts/vite'
import Wails from '@wailsio/runtime/plugins/vite'
import JavaScriptObfuscator from 'javascript-obfuscator'

// Utilities
import { defineConfig, type Plugin } from 'vite'
import { fileURLToPath, URL } from 'node:url'
import Icons from 'unplugin-icons/vite'

function productionObfuscation(): Plugin {
  return {
    name: 'btg-production-obfuscation',
    apply: 'build',
    enforce: 'post',
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        // The entry chunk contains the watermark and runtime guards. Limiting
        // deep obfuscation to entries keeps lazy-loaded UI chunks reliable and
        // avoids multiplying the application bundle size.
        if (output.type !== 'chunk' || !output.isEntry) continue

        output.code = JavaScriptObfuscator.obfuscate(output.code, {
          compact: true,
          controlFlowFlattening: true,
          controlFlowFlatteningThreshold: 0.28,
          deadCodeInjection: false,
          debugProtection: false,
          disableConsoleOutput: true,
          identifierNamesGenerator: 'hexadecimal',
          numbersToExpressions: true,
          renameGlobals: false,
          selfDefending: true,
          simplify: true,
          splitStrings: true,
          splitStringsChunkLength: 8,
          stringArray: true,
          stringArrayCallsTransform: true,
          stringArrayCallsTransformThreshold: 0.5,
          stringArrayEncoding: ['base64'],
          stringArrayIndexShift: true,
          stringArrayRotate: true,
          stringArrayShuffle: true,
          stringArrayThreshold: 0.72,
          transformObjectKeys: false,
          unicodeEscapeSequence: false,
        }).getObfuscatedCode()
      }
    },
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    Wails('./bindings'),
    Vue({
      template: { transformAssetUrls },
    }),
    // https://github.com/vuetifyjs/vuetify-loader/tree/master/packages/vite-plugin#readme
    Vuetify({
      autoImport: true,
      styles: {
        configFile: 'src/styles/settings.scss',
      },
    }),
    Icons({
      compiler: 'vue3',
    }),
    Components({
      dts: 'src/components.d.ts',
    }),
    Fonts({
      fontsource: {
        families: [
          {
            name: 'Roboto',
            weights: [100, 300, 400, 500, 700, 900],
            styles: ['normal', 'italic'],
          },
        ],
      },
    }),
    productionObfuscation(),
  ],
  optimizeDeps: {
    exclude: [
      'vuetify',
      'vue-router',
    ],
  },
  define: { 'process.env': {} },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('src', import.meta.url)),
    },
    extensions: [
      '.js',
      '.json',
      '.jsx',
      '.mjs',
      '.ts',
      '.tsx',
      '.vue',
    ],
  },
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 3000,
    strictPort: true,
  },
  build: {
    sourcemap: false,
    reportCompressedSize: false,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
  },
})
