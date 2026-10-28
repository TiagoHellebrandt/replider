import { defineConfig } from 'vite';

export default defineConfig({
  // Vite 8 transforms with oxc, not esbuild. Use the automatic JSX runtime
  // (matching tsconfig's jsx: react-jsx + jsxImportSource: preact) so JSX in
  // .tsx files resolves to preact/jsx-runtime. The old classic-runtime
  // `jsxInject` of `h`/`Fragment` collided with widget.ts's explicit
  // `import { h } from 'preact'` under oxc ("h has already been declared").
  oxc: {
    jsx: {
      runtime: 'automatic',
      importSource: 'preact',
    },
  },
  build: {
    target: 'es2019',
    lib: {
      entry: 'src/widget.ts',
      name: 'ReplyderWidget',
      formats: ['iife'],
      fileName: () => 'widget.js',
    },
    rollupOptions: {
      output: {
        extend: true,
      },
    },
    sourcemap: true,
    cssCodeSplit: false,
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    setupFiles: ['./test/setup.ts'],
  },
});
