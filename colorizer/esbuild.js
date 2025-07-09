const esbuild = require("esbuild");

esbuild.build({
    entryPoints: ['frontend/Application.tsx'],
    minify: true,
    outdir: '../_static/_esbuild'
});
