#!/usr/bin/env node
// Saves one full-page screenshot of the public leaderboard per level to screens/
// (ignored by git). The level tabs are buttons with no address of their own, so a
// headless Chrome is driven over its DevTools port: click "Level N", wait for the table,
// capture. A throwaway profile keeps it away from the normal Chrome profile.
//
//   node scripts/screenshot_levels.mjs            # levels 8..1
//   node scripts/screenshot_levels.mjs 8 7        # only these
import { spawn } from "node:child_process";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const PAGE_URL = "https://spread.bahnexpress.com/leaderboard";
const CHROME = process.env.CHROME || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const WIDTH = 1500;
const MIN_HEIGHT = 1200;
const MAX_HEIGHT = 12000;
const LOAD_WAIT_MS = 6000;
const TAB_WAIT_MS = 3500;
const START_TIMEOUT_MS = 20000;
const CALL_TIMEOUT_MS = 30000;

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const levels = process.argv.slice(2).map(Number).filter((n) => n >= 1 && n <= 8);
if (levels.length === 0) levels.push(8, 7, 6, 5, 4, 3, 2, 1);

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function stamp() {
  const d = new Date();
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}`;
}

// startChrome launches a headless Chrome and resolves with its DevTools address.
function startChrome(profile) {
  const child = spawn(CHROME, [
    "--headless=new", `--user-data-dir=${profile}`, "--no-first-run", "--disable-gpu", "--hide-scrollbars",
    "--remote-debugging-port=0", `--window-size=${WIDTH},${MIN_HEIGHT}`, "about:blank",
  ], { stdio: ["ignore", "ignore", "pipe"] });
  const address = new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("Chrome did not report its DevTools port")), START_TIMEOUT_MS);
    let seen = "";
    child.stderr.on("data", (chunk) => {
      seen += chunk;
      const m = seen.match(/DevTools listening on (ws:\/\/\S+)/);
      if (m) {
        clearTimeout(timer);
        resolve(m[1]);
      }
    });
    child.on("error", reject);
    child.on("exit", (code) => reject(new Error(`Chrome exited early (code ${code})`)));
  });
  return { child, address };
}

// connect opens one DevTools session and returns a call(method, params) function.
async function connect(wsUrl) {
  const ws = new WebSocket(wsUrl);
  await new Promise((resolve, reject) => {
    ws.addEventListener("open", resolve, { once: true });
    ws.addEventListener("error", () => reject(new Error(`cannot reach ${wsUrl}`)), { once: true });
  });
  let nextId = 1;
  const waiting = new Map();
  ws.addEventListener("message", (ev) => {
    const msg = JSON.parse(ev.data);
    const w = waiting.get(msg.id);
    if (!w) return;
    waiting.delete(msg.id);
    clearTimeout(w.timer);
    if (msg.error) w.reject(new Error(`${w.method}: ${msg.error.message}`));
    else w.resolve(msg.result);
  });
  const call = (method, params = {}) => new Promise((resolve, reject) => {
    const id = nextId++;
    const timer = setTimeout(() => {
      waiting.delete(id);
      reject(new Error(`${method}: no answer`));
    }, CALL_TIMEOUT_MS);
    waiting.set(id, { resolve, reject, timer, method });
    ws.send(JSON.stringify({ id, method, params }));
  });
  return { call, close: () => ws.close() };
}

async function pageSession(browserWs) {
  const port = new URL(browserWs).port;
  const res = await fetch(`http://127.0.0.1:${port}/json/list`);
  if (!res.ok) throw new Error(`DevTools target list: HTTP ${res.status}`);
  const page = (await res.json()).find((t) => t.type === "page");
  if (!page) throw new Error("Chrome has no page to drive");
  return connect(page.webSocketDebuggerUrl);
}

async function capture(session, level, out) {
  const click = `(() => {
    const b = [...document.querySelectorAll("button")].find((x) => x.textContent.trim() === "Level ${level}");
    if (!b) return "no tab";
    b.click();
    return "ok";
  })()`;
  const clicked = await session.call("Runtime.evaluate", { expression: click, returnByValue: true });
  if (clicked.result.value !== "ok") throw new Error(`level ${level}: tab not found on the page`);
  await sleep(TAB_WAIT_MS);
  const size = await session.call("Runtime.evaluate", {
    expression: "Math.ceil(document.documentElement.scrollHeight)", returnByValue: true,
  });
  const height = Math.max(MIN_HEIGHT, Math.min(MAX_HEIGHT, Number(size.result.value) || MIN_HEIGHT));
  await session.call("Emulation.setDeviceMetricsOverride", { width: WIDTH, height, deviceScaleFactor: 1, mobile: false });
  await sleep(500);
  const shot = await session.call("Page.captureScreenshot", { format: "png" });
  writeFileSync(out, Buffer.from(shot.data, "base64"));
  console.log(`saved ${out}`);
}

async function main() {
  mkdirSync(join(root, "screens"), { recursive: true });
  const profile = mkdtempSync(join(tmpdir(), "hc-shot-"));
  const { child, address } = startChrome(profile);
  let failed = 0;
  try {
    const session = await pageSession(await address);
    await session.call("Page.enable");
    await session.call("Page.navigate", { url: PAGE_URL });
    await sleep(LOAD_WAIT_MS);
    const when = stamp();
    for (const level of levels) {
      try {
        await capture(session, level, join(root, "screens", `leaderboard-${when}-L${level}.png`));
      } catch (err) {
        failed++;
        console.error(`level ${level}: ${err.message}`);
      }
    }
    session.close();
  } finally {
    child.removeAllListeners("exit");
    child.kill();
    await sleep(300);
    rmSync(profile, { recursive: true, force: true });
  }
  if (failed > 0) process.exit(1);
}

main().catch((err) => {
  console.error(`screenshot failed: ${err.message}`);
  process.exit(1);
});
