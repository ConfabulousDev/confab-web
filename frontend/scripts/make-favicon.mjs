#!/usr/bin/env node
// Writes frontend/public/favicon.svg: the Lobster "C" on the dark rounded
// square, drawn as an SVG <path>. Browsers load no fonts or other external
// resources inside an SVG used as an icon, so the glyph must be an outline.
// The docs site uses a copy (docs/public/favicon.svg).
//
// Re-run after changing the mark (from frontend/, after `npm install` so the
// self-hosted Lobster font resolves), then copy the result to the docs:
//
//   npx -y -p opentype.js node scripts/make-favicon.mjs
//   cp public/favicon.svg ../docs/public/favicon.svg
//
// opentype.js comes from npx, not package.json (it is only needed here). It
// reads WOFF but not WOFF2, hence the .woff file.
import { existsSync } from 'node:fs';
import { readFile, writeFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const require = createRequire(import.meta.url);

// `npx -p` puts the package's node_modules/.bin on PATH but not on the module
// search path, so look for opentype.js next to those .bin directories.
function resolveOpentype() {
  try {
    return require.resolve('opentype.js');
  } catch {
    for (const dir of (process.env.PATH ?? '').split(path.delimiter)) {
      if (!dir.endsWith(path.join('node_modules', '.bin'))) continue;
      const modules = path.dirname(dir);
      if (existsSync(path.join(modules, 'opentype.js'))) {
        return createRequire(path.join(modules, 'noop.js')).resolve('opentype.js');
      }
    }
  }
  throw new Error('opentype.js not found; run via `npx -y -p opentype.js node scripts/make-favicon.mjs`');
}

const { default: opentype } = await import(pathToFileURL(resolveOpentype()).href);

const woff = require.resolve('@fontsource/lobster/files/lobster-latin-400-normal.woff');
const out = fileURLToPath(new URL('../public/favicon.svg', import.meta.url));

const BOX = 32;
const FONT_SIZE = 22; // matches the old <text font-size="22"> favicon

const buf = await readFile(woff);
const font = opentype.parse(buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength));

// Centre the glyph's ink box in the 32x32 square.
const probe = font.getPath('C', 0, 0, FONT_SIZE).getBoundingBox();
const x = (BOX - (probe.x2 - probe.x1)) / 2 - probe.x1;
const y = (BOX - (probe.y2 - probe.y1)) / 2 - probe.y1;
const d = font.getPath('C', x, y, FONT_SIZE).toPathData(2);

const svg =
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${BOX} ${BOX}">` +
  `<rect width="${BOX}" height="${BOX}" rx="6" fill="#1a1a1a"/>` +
  `<path fill="#ffffff" d="${d}"/></svg>\n`;

await writeFile(out, svg);
console.log(`wrote ${path.relative(process.cwd(), out)}`);
