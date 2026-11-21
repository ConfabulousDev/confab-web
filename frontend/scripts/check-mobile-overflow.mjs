#!/usr/bin/env node
// Mobile horizontal-overflow guard.
//
// Serves a built Storybook (storybook-static/), renders every story at a
// 375x812 touch viewport, and fails when an element's right edge passes the
// viewport while its parent's does not. iOS Safari pans the whole page
// sideways when any child is wider than the screen, even though the app
// shell sets overflow: hidden, so each such element is a real bug on phones.
//
// Not flagged: descendants of horizontal scrollers (overflow-x: auto|scroll),
// anything inside <pre>/<code>, and entries in mobile-overflow-allowlist.json:
//   [{ "selector": "div.tableWrapper", "title": "Pages/X", "story": "Default", "reason": "why" }]
// `selector` is the reported tag.class form; `title` and `story` are optional
// and narrow the entry to one story file or one story. Every entry needs a reason.
//
// Usage: node scripts/check-mobile-overflow.mjs [--dir storybook-static] [--filter <text>]
//   --filter  only check stories whose id or title contains <text>
// Run via `npm run check:mobile` (builds Storybook first).

import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { extname, join, normalize, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright-core';

const VIEWPORT = { width: 375, height: 812 };
const CONCURRENCY = 4;
const SETTLE_MS = 300;
const NAV_TIMEOUT_MS = 20000;

const here = dirname(fileURLToPath(import.meta.url));
const args = process.argv.slice(2);
const argValue = (flag) => {
  const i = args.indexOf(flag);
  return i >= 0 ? args[i + 1] : undefined;
};
const root = resolve(argValue('--dir') ?? join(here, '..', 'storybook-static'));
const filter = argValue('--filter');
const allowlist = JSON.parse(await readFile(join(here, 'mobile-overflow-allowlist.json'), 'utf8'));

const MIME = {
  '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css',
  '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.jpg': 'image/jpeg',
  '.woff': 'font/woff', '.woff2': 'font/woff2', '.ico': 'image/x-icon', '.map': 'application/json',
};

const server = createServer(async (req, res) => {
  const path = normalize(decodeURIComponent(new URL(req.url, 'http://x').pathname));
  let file = join(root, path);
  if (!file.startsWith(root)) {
    res.writeHead(403).end();
    return;
  }
  try {
    if ((await stat(file)).isDirectory()) file = join(file, 'index.html');
    const body = await readFile(file);
    res.writeHead(200, { 'content-type': MIME[extname(file)] ?? 'application/octet-stream' }).end(body);
  } catch {
    res.writeHead(404).end();
  }
});
await new Promise((ok) => server.listen(0, '127.0.0.1', ok));
const base = `http://127.0.0.1:${server.address().port}`;

let index;
try {
  index = JSON.parse(await readFile(join(root, 'index.json'), 'utf8'));
} catch {
  console.error(`No Storybook build at ${root}. Run \`npm run build-storybook\` first.`);
  process.exit(2);
}
const stories = Object.values(index.entries).filter(
  (e) => e.type === 'story' && (!filter || e.id.includes(filter) || e.title.includes(filter)),
);

// Runs in the page. Returns "selector right=N" for each element that is the
// first in its ancestor chain to pass the viewport's right edge.
function findOverflow() {
  const vw = window.innerWidth;
  const name = (el) => {
    // CSS-module classes look like _name_hash_N; report the readable part.
    const classes = [...el.classList].map((c) => c.replace(/^_(.+)_[a-z0-9]{5}_\d+$/, '$1'));
    return [el.tagName.toLowerCase(), ...classes].join('.');
  };
  const isScroller = (el) => {
    const x = getComputedStyle(el).overflowX;
    return x === 'auto' || x === 'scroll';
  };
  const out = new Set();
  const visit = (el) => {
    if (el.tagName === 'PRE' || el.tagName === 'CODE') return;
    const r = el.getBoundingClientRect();
    const p = el.parentElement.getBoundingClientRect();
    if (r.width > 0 && r.height > 0 && r.right > vw + 1 && p.right <= vw + 1) {
      out.add(`${name(el)} right=${Math.round(r.right)}`);
    }
    if (isScroller(el)) return;
    for (const child of el.children) visit(child);
  };
  const rootEl = document.getElementById('storybook-root');
  for (const child of rootEl.children) visit(child);
  // Portaled content (popovers, tooltips, modals) renders under <body>.
  for (const child of document.body.children) {
    if (child !== rootEl && !child.id.startsWith('storybook-') && child.tagName !== 'SCRIPT') visit(child);
  }
  return [...out];
}

const allowed = (story, hit) =>
  allowlist.some(
    (a) => hit.startsWith(`${a.selector} `) && (!a.title || a.title === story.title) && (!a.story || a.story === story.name),
  );

const browser = await chromium.launch();
const failures = [];
const errors = [];
let next = 0;

async function worker() {
  const context = await browser.newContext({ viewport: VIEWPORT, isMobile: true, hasTouch: true, deviceScaleFactor: 1 });
  const page = await context.newPage();
  while (next < stories.length) {
    const story = stories[next++];
    try {
      await page.goto(`${base}/iframe.html?id=${story.id}&viewMode=story`, { waitUntil: 'load', timeout: NAV_TIMEOUT_MS });
      // Storybook sets one of these body classes once the story renders or throws.
      await page.waitForFunction(
        () => ['sb-show-main', 'sb-show-errordisplay'].some((c) => document.body.classList.contains(c)),
        null,
        { timeout: NAV_TIMEOUT_MS },
      );
      if (await page.evaluate(() => document.body.classList.contains('sb-show-errordisplay'))) {
        throw new Error('story failed to render');
      }
      await page.waitForTimeout(SETTLE_MS);
      for (const hit of await page.evaluate(findOverflow)) {
        if (!allowed(story, hit)) failures.push(`${story.title} :: ${story.name} :: ${hit}`);
      }
    } catch (e) {
      errors.push(`${story.title} :: ${story.name} :: ${String(e).split('\n')[0]}`);
    }
  }
  await context.close();
}

const started = Date.now();
await Promise.all(Array.from({ length: CONCURRENCY }, worker));
await browser.close();
server.close();

const secs = ((Date.now() - started) / 1000).toFixed(0);
for (const e of errors) console.error(`ERROR   ${e}`);
for (const f of failures.sort()) console.error(`OVERFLOW ${f}`);
console.log(
  `Checked ${stories.length} stories at ${VIEWPORT.width}px in ${secs}s: ` +
    `${failures.length} overflow(s), ${errors.length} error(s).`,
);
if (failures.length || errors.length) {
  console.log('Fix the element to wrap or truncate, or add an allowlist entry with a reason in scripts/mobile-overflow-allowlist.json.');
  process.exit(1);
}
