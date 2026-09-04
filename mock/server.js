#!/usr/bin/env node
"use strict";

// Temporary mock of the ploi.io API for promo screenshots.
// No dependencies. Serves 100% fictional data on 127.0.0.1 only.
// Run:  node mock/server.js
// Use:  XDG_CACHE_HOME="$(mktemp -d)" PLOI_TUI_API_URL=http://127.0.0.1:8787 ./bin/ploi-tui

const http = require("http");

const PORT = Number(process.env.PORT) || 8787;
const HOST = process.env.HOST || "127.0.0.1";

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

const pad = (n) => String(n).padStart(2, "0");

// ploi-tui parses "YYYY-MM-DD HH:mm:ss" (UTC) or RFC3339.
const fmt = (d) =>
  `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ` +
  `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;

const hoursAgo = (h) => new Date(Date.now() - h * 3600 * 1000);
const minutesAgo = (m) => new Date(Date.now() - m * 60 * 1000);
const daysAhead = (d) => new Date(Date.now() + d * 24 * 3600 * 1000);

const single = (data) => ({ data });
const list = (items) => ({
  data: items,
  links: { first: null, last: null, prev: null, next: null },
  meta: { current_page: 1, last_page: 1, from: 1, to: items.length, total: items.length, path: "/api" },
});

// deterministic pseudo-random in [0,1) so screenshots are reproducible
const frac = (v) => v - Math.floor(v);
const rnd = (seed, i) => frac(Math.sin(seed * 99.7 + i * 12.9898) * 43758.5453);
const round1 = (v) => Math.round(v * 10) / 10;
const round2 = (v) => Math.round(v * 100) / 100;
const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// ---------------------------------------------------------------------------
// fixtures (all fictional; 203.0.113.x is reserved for documentation)
// ---------------------------------------------------------------------------

const user = {
  avatar: "",
  name: "Demo Developer",
  email: "demo@example.com",
  country: "Netherlands",
  timezone: "UTC",
  created_at: fmt(new Date("2023-11-02T10:24:00Z")),
  plan: "Expert",
  plan_expires_at: null,
  billing_details: {},
};

const servers = [
  {
    id: 101,
    type: "server",
    name: "northstar-production",
    ip_address: "203.0.113.10",
    php_version: 8.3,
    mysql_version: 8.0,
    sites_count: 3,
    status: "active",
    status_id: 1,
    monitoring: true,
    created_at: fmt(new Date("2024-03-14T09:12:00Z")),
  },
  {
    id: 102,
    type: "server",
    name: "northstar-staging",
    ip_address: "203.0.113.27",
    php_version: 8.2,
    mysql_version: 8.0,
    sites_count: 2,
    status: "active",
    status_id: 1,
    monitoring: true,
    created_at: fmt(new Date("2024-05-21T15:47:00Z")),
  },
  {
    id: 103,
    type: "server",
    name: "edge-worker-04",
    ip_address: "203.0.113.42",
    php_version: 8.1,
    mysql_version: 0,
    sites_count: 1,
    status: "rebooting",
    status_id: 4,
    monitoring: false,
    created_at: fmt(new Date("2025-01-08T08:05:00Z")),
  },
];

const databases = {
  101: [
    { id: 11, type: "mysql", name: "app_example_prod", server_id: 101, status: "active", created_at: fmt(new Date("2024-03-15T10:02:00Z")) },
    { id: 12, type: "mysql", name: "api_example_prod", server_id: 101, status: "active", created_at: fmt(new Date("2024-04-02T13:40:00Z")) },
    { id: 13, type: "postgres", name: "dash_analytics", server_id: 101, status: "active", created_at: fmt(new Date("2024-06-19T09:15:00Z")) },
  ],
  102: [
    { id: 21, type: "mysql", name: "app_example_staging", server_id: 102, status: "active", created_at: fmt(new Date("2024-05-22T11:30:00Z")) },
  ],
  103: [],
};

const systemUsers = {
  101: [
    { id: 5, name: "ploi", root: "/home/ploi", created_at: fmt(new Date("2024-03-14T09:13:00Z")) },
    { id: 6, name: "deploy", root: "/home/deploy", created_at: fmt(new Date("2024-03-14T09:20:00Z")) },
  ],
  102: [
    { id: 7, name: "ploi", root: "/home/ploi", created_at: fmt(new Date("2024-05-21T15:48:00Z")) },
  ],
  103: [
    { id: 8, name: "ploi", root: "/home/ploi", created_at: fmt(new Date("2025-01-08T08:06:00Z")) },
  ],
};

const sites = {
  101: [
    {
      id: 301, status: "active", server_id: 101, domain: "app.example.io",
      deploy_script: "git pull\ncomposer install --no-dev\nphp artisan migrate --force",
      web_directory: "/public", project_type: "laravel", project_root: "/home/deploy/app.example.io",
      last_deploy_at: fmt(minutesAgo(42)), system_user: "deploy", php_version: 8.3,
      health_url: "https://app.example.io/up", has_repository: true, quick_deploy: true,
      disk_usage: { bytes: 2345678901, human: "2.2 GB" },
      created_at: fmt(new Date("2024-03-15T10:00:00Z")),
    },
    {
      id: 302, status: "active", server_id: 101, domain: "api.example.io",
      deploy_script: "git pull\ncomposer install",
      web_directory: "/public", project_type: "laravel", project_root: "/home/deploy/api.example.io",
      last_deploy_at: fmt(hoursAgo(5)), system_user: "deploy", php_version: 8.3,
      health_url: null, has_repository: true, quick_deploy: true,
      disk_usage: { bytes: 812340000, human: "775 MB" },
      created_at: fmt(new Date("2024-04-02T13:38:00Z")),
    },
    {
      id: 303, status: "active", server_id: 101, domain: "dash.example.com",
      deploy_script: "git pull\nnpm run build",
      web_directory: "/dist", project_type: "php", project_root: "/home/ploi/dash.example.com",
      last_deploy_at: fmt(hoursAgo(26)), system_user: "ploi", php_version: 8.3,
      health_url: null, has_repository: true, quick_deploy: false,
      disk_usage: { bytes: 419000000, human: "400 MB" },
      created_at: fmt(new Date("2024-06-19T09:12:00Z")),
    },
  ],
  102: [
    {
      id: 311, status: "active", server_id: 102, domain: "staging.example.io",
      deploy_script: "git pull\ncomposer install",
      web_directory: "/public", project_type: "laravel", project_root: "/home/ploi/staging.example.io",
      last_deploy_at: fmt(hoursAgo(3)), system_user: "ploi", php_version: 8.2,
      health_url: null, has_repository: true, quick_deploy: true,
      disk_usage: { bytes: 1560000000, human: "1.5 GB" },
      created_at: fmt(new Date("2024-05-22T11:28:00Z")),
    },
    {
      id: 312, status: "deploying", server_id: 102, domain: "preview.example.dev",
      deploy_script: "git pull",
      web_directory: "/public", project_type: "php", project_root: "/home/ploi/preview.example.dev",
      last_deploy_at: null, system_user: "ploi", php_version: 8.2,
      health_url: null, has_repository: false, quick_deploy: false,
      disk_usage: { bytes: 95000000, human: "91 MB" },
      created_at: fmt(new Date("2025-02-11T16:05:00Z")),
    },
  ],
  103: [
    {
      id: 321, status: "active", server_id: 103, domain: "status.example.net",
      deploy_script: "git pull",
      web_directory: "/public", project_type: "php", project_root: "/home/ploi/status.example.net",
      last_deploy_at: fmt(hoursAgo(50)), system_user: "ploi", php_version: 8.1,
      health_url: null, has_repository: true, quick_deploy: false,
      disk_usage: { bytes: 52000000, human: "50 MB" },
      created_at: fmt(new Date("2025-01-08T08:30:00Z")),
    },
  ],
};

const certificates = {
  "101-301": [
    { id: 41, status: "active", domain: "app.example.io", type: "letsencrypt", active: true, site_id: 301, server_id: 101, expires_at: fmt(daysAhead(62)), created_at: fmt(new Date("2024-03-15T10:05:00Z")) },
    { id: 42, status: "active", domain: "www.app.example.io", type: "letsencrypt", active: true, site_id: 301, server_id: 101, expires_at: fmt(daysAhead(62)), created_at: fmt(new Date("2024-03-15T10:06:00Z")) },
  ],
  "101-302": [
    { id: 43, status: "active", domain: "api.example.io", type: "letsencrypt", active: true, site_id: 302, server_id: 101, expires_at: fmt(daysAhead(21)), created_at: fmt(new Date("2024-04-02T13:41:00Z")) },
  ],
  "101-303": [
    { id: 44, status: "active", domain: "dash.example.com", type: "custom", active: true, site_id: 303, server_id: 101, expires_at: fmt(daysAhead(180)), created_at: fmt(new Date("2024-06-19T09:18:00Z")) },
  ],
  "102-311": [
    { id: 45, status: "active", domain: "staging.example.io", type: "letsencrypt", active: true, site_id: 311, server_id: 102, expires_at: fmt(daysAhead(5)), created_at: fmt(new Date("2024-05-22T11:32:00Z")) },
  ],
  "102-312": [],
  "103-321": [
    { id: 46, status: "active", domain: "status.example.net", type: "letsencrypt", active: true, site_id: 321, server_id: 103, expires_at: fmt(daysAhead(90)), created_at: fmt(new Date("2025-01-08T08:32:00Z")) },
  ],
};

// 24h of samples, every 15 minutes; deterministic waveforms per server.
function monitoringSeries(server) {
  const samples = [];
  const COUNT = 24 * 4;
  const STEP = 15 * 60 * 1000;
  const seed = server.id;
  for (let j = 0; j < COUNT; j++) {
    const r = rnd(seed, j);
    const cpu = clamp(14 + 12 * Math.sin(j / 6 + seed) + 10 * r + 6 * Math.sin(j / 23), 2, 92);
    const ram = clamp(52 + 10 * Math.sin(j / 11 + seed) + 4 * r, 20, 95);
    const disk = clamp(41 + j * 0.02 + 2 * r, 5, 97);
    const load = clamp(cpu / 28 + 0.35 * r, 0, 16);
    samples.push({
      cpu: round1(cpu),
      ram: round1(ram),
      disk: round1(disk),
      load_average: round2(load),
      date: fmt(new Date(Date.now() - (COUNT - 1 - j) * STEP)),
    });
  }
  return samples;
}

// ---------------------------------------------------------------------------
// routing
// ---------------------------------------------------------------------------

const notFound = (what) => ({ status: 404, body: { message: `${what} not found.` } });
const ok = (body) => ({ status: 200, body });

const findServer = (id) => servers.find((s) => s.id === Number(id));
const findSite = (serverID, siteID) => (sites[Number(serverID)] || []).find((s) => s.id === Number(siteID));

const routes = [
  ["GET", /^\/api\/user$/, () => ok(single(user))],

  ["GET", /^\/api\/servers$/, () => ok(list(servers))],

  ["GET", /^\/api\/servers\/(\d+)$/, (m) => {
    const server = findServer(m[1]);
    return server ? ok(single(server)) : notFound("Server");
  }],

  ["POST", /^\/api\/servers\/(\d+)\/restart$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    server.status = "rebooting";
    return ok({ data: { message: "Server reboot scheduled." } });
  }],

  ["GET", /^\/api\/servers\/(\d+)\/monitor$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    return ok({ data: server.monitoring ? monitoringSeries(server) : [] });
  }],

  ["GET", /^\/api\/servers\/(\d+)\/databases$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    return ok(list(databases[server.id] || []));
  }],

  ["GET", /^\/api\/servers\/(\d+)\/system-users$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    return ok(list(systemUsers[server.id] || []));
  }],

  ["GET", /^\/api\/servers\/(\d+)\/sites$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    return ok(list(sites[server.id] || []));
  }],

  ["GET", /^\/api\/servers\/(\d+)\/sites\/(\d+)$/, (m) => {
    if (!findServer(m[1])) return notFound("Server");
    const site = findSite(m[1], m[2]);
    return site ? ok(single(site)) : notFound("Site");
  }],

  ["GET", /^\/api\/servers\/(\d+)\/sites\/(\d+)\/certificates$/, (m) => {
    if (!findServer(m[1])) return notFound("Server");
    if (!findSite(m[1], m[2])) return notFound("Site");
    return ok(list(certificates[`${Number(m[1])}-${Number(m[2])}`] || []));
  }],
];

// ---------------------------------------------------------------------------
// server
// ---------------------------------------------------------------------------

let requestCount = 0;

const httpServer = http.createServer((req, res) => {
  const path = new URL(req.url, `http://${req.headers.host || "localhost"}`).pathname;

  // Never log headers or tokens — path only.
  console.log(`${new Date().toISOString()} ${req.method} ${path}`);

  const rateHeaders = {
    "X-RateLimit-Limit": "600",
    "X-RateLimit-Remaining": String(Math.max(0, 600 - ++requestCount)),
  };

  const route = routes.find(([method, pattern]) => method === req.method && pattern.test(path));
  if (!route) {
    res.writeHead(404, { "Content-Type": "application/json", ...rateHeaders });
    res.end(JSON.stringify({ message: "Endpoint not found." }));
    return;
  }

  const match = route[1].exec(path);
  const { status, body } = route[2](match);
  res.writeHead(status, { "Content-Type": "application/json", ...rateHeaders });
  res.end(JSON.stringify(body));
});

httpServer.listen(PORT, HOST, () => {
  console.log(`ploi-tui mock API listening on http://${HOST}:${PORT}`);
  console.log("All data is fictional. Start the TUI with:");
  console.log(`  XDG_CACHE_HOME="$(mktemp -d)" PLOI_TUI_API_URL=http://${HOST}:${PORT} ./bin/ploi-tui`);
});
