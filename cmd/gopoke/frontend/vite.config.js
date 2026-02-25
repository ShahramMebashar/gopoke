import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import importMetaUrlPlugin from "@codingame/esbuild-import-meta-url-plugin";

// Rollup doesn't resolve wildcard exports (./vscode/* → ./vscode/src/*) from
// @codingame/monaco-vscode-api. This plugin rewrites the import paths so Rollup
// can find the actual files.
function resolveMonacoVscodeExports() {
  const prefix = "@codingame/monaco-vscode-api/vscode/";
  return {
    name: "resolve-monaco-vscode-exports",
    resolveId(source, importer) {
      if (!source.startsWith(prefix)) return null;
      const subpath = source.slice(prefix.length);
      const resolved = path.resolve(
        __dirname,
        "node_modules/@codingame/monaco-vscode-api/vscode/src",
        subpath + ".js"
      );
      return resolved;
    },
  };
}

export default defineConfig({
  plugins: [react(), resolveMonacoVscodeExports()],
  worker: {
    format: "es",
  },
  resolve: {
    dedupe: ["vscode"],
  },
  optimizeDeps: {
    include: ["vscode-textmate", "vscode-oniguruma"],
    esbuildOptions: {
      plugins: [importMetaUrlPlugin],
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
