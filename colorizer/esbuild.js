const esbuild = require("esbuild");

esbuild.build({
    entryPoints: ['frontend/Application.tsx'],
    minify: true,
    bundle: true,
    outdir: '../_static/_esbuild'
});
