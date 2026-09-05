#!/usr/bin/env node
"use strict";

// Temporary mock of the ploi.io API for promo screenshots.
// No dependencies. Serves 100% fictional data on 127.0.0.1 only.
// Run:  node mock/server.js            (or: ./mock/demo.sh)
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

const gbSize = (n) => ({
  bytes: Math.round(n * 1024 * 1024 * 1024),
  human: `${n % 1 ? n.toFixed(1) : n} GB`,
});
const mbSize = (n) => ({
  bytes: Math.round(n * 1024 * 1024),
  human: `${Math.round(n)} MB`,
});

// ---------------------------------------------------------------------------
// fixtures (all fictional; 203.0.113.x is reserved for documentation, RFC 5737)
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

const STATUS_IDS = { active: 1, building: 2, refreshing: 3, rebooting: 4, unreachable: 7 };

const scripts = {
  laravel: "git pull\ncomposer install --no-dev\nphp artisan migrate --force\nphp artisan config:cache",
  php: "git pull\ncomposer install",
  wordpress: "git pull",
  static: "git pull\nnpm ci\nnpm run build",
};

const webDirs = { laravel: "/public", php: "/public", wordpress: "/", static: "/dist" };

// Deterministic expiry mix (days): a few critical (<=7), some warning (<=30), mostly OK.
const expiryBuckets = [3, 21, 45, 62, 62, 90, 90, 180, 180, 12];

const servers = [];
const sitesByServer = {};
const databasesByServer = {};
const systemUsersByServer = {};
const certificatesBySite = {};

let nextServerID = 100;
let nextSiteID = 300;
let nextDatabaseID = 10;
let nextUserID = 4;
let nextCertificateID = 40;

function addServer(spec) {
  const id = ++nextServerID;
  const status = spec.status || "active";
  const ip = `203.0.113.${10 + servers.length}`;
  servers.push({
    id,
    type: "server",
    name: spec.name,
    ip_address: ip,
    php_version: spec.php,
    mysql_version: spec.mysql || 0,
    sites_count: spec.sites.length,
    status,
    status_id: STATUS_IDS[status] || 1,
    monitoring: spec.monitoring !== false,
    created_at: fmt(spec.created),
  });

  const userNames = spec.users || ["ploi"];
  systemUsersByServer[id] = userNames.map((name, i) => ({
    id: ++nextUserID,
    name,
    root: `/home/${name}`,
    created_at: fmt(new Date(spec.created.getTime() + (i + 1) * 60 * 1000)),
  }));

  const siteList = [];
  spec.sites.forEach((s, i) => {
    const siteID = ++nextSiteID;
    const type = s.type || "php";
    const owner = s.user || userNames[0];
    siteList.push({
      id: siteID,
      status: s.status || "active",
      server_id: id,
      domain: s.domain,
      deploy_script: s.script || scripts[type],
      web_directory: s.web || webDirs[type],
      project_type: type,
      project_root: `/home/${owner}/${s.domain}`,
      last_deploy_at: s.deployed === null ? null : fmt(s.deployed || hoursAgo(24 + i * 5)),
      system_user: owner,
      php_version: s.php || spec.php,
      health_url: s.health || null,
      has_repository: s.repo !== false,
      quick_deploy: s.quick === true,
      disk_usage: s.disk || mbSize(90 + ((siteID * 37) % 400)),
      created_at: fmt(new Date(spec.created.getTime() + (i + 1) * 3600 * 1000)),
    });

    const expiresInDays = expiryBuckets[siteID % expiryBuckets.length];
    const certCreated = new Date(
      Math.max(spec.created.getTime(), daysAhead(expiresInDays - 95).getTime())
    );
    const certs = [
      {
        id: ++nextCertificateID,
        status: "active",
        domain: s.domain,
        type: siteID % 9 === 4 ? "custom" : "letsencrypt",
        active: true,
        site_id: siteID,
        server_id: id,
        expires_at: fmt(daysAhead(expiresInDays)),
        created_at: fmt(certCreated),
      },
    ];
    if (s.www) {
      certs.push({
        id: ++nextCertificateID,
        status: "active",
        domain: s.www,
        type: "letsencrypt",
        active: true,
        site_id: siteID,
        server_id: id,
        expires_at: fmt(daysAhead(expiresInDays)),
        created_at: fmt(certCreated),
      });
    }
    certificatesBySite[`${id}-${siteID}`] = certs;
  });
  sitesByServer[id] = siteList;

  let dbs = spec.databases;
  if (!dbs && (spec.mysql || 0) > 0) {
    dbs = siteList
      .filter((s) => s.project_type !== "static")
      .map((s) => `${s.domain.replace(/\./g, "_")}:mysql`);
  }
  databasesByServer[id] = (dbs || []).map((entry, i) => {
    const [name, type] = entry.split(":");
    return {
      id: ++nextDatabaseID,
      type: type || "mysql",
      name,
      server_id: id,
      status: "active",
      created_at: fmt(new Date(spec.created.getTime() + (i + 2) * 600 * 1000)),
    };
  });
}

// -- northstar (flagship Laravel apps + an edge worker) ----------------------

addServer({
  name: "northstar-production", php: 8.3, mysql: 8.0,
  created: new Date("2024-03-14T09:12:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "app.example.io", type: "laravel", quick: true, user: "deploy",
      health: "https://app.example.io/up", deployed: minutesAgo(42), disk: gbSize(2.2), www: "www.app.example.io" },
    { domain: "api.example.io", type: "laravel", quick: true, user: "deploy",
      deployed: hoursAgo(5), disk: mbSize(775) },
    { domain: "dash.example.com", type: "php", web: "/dist",
      deployed: hoursAgo(26), disk: mbSize(400) },
  ],
});

addServer({
  name: "northstar-staging", php: 8.2, mysql: 8.0,
  created: new Date("2024-05-21T15:47:00Z"), users: ["ploi"],
  sites: [
    { domain: "staging.example.io", type: "laravel", quick: true,
      deployed: hoursAgo(3), disk: gbSize(1.5) },
    { domain: "preview.example.dev", type: "php", status: "deploying",
      deployed: null, repo: false, disk: mbSize(91) },
  ],
});

addServer({
  name: "edge-worker-04", php: 8.1, status: "rebooting", monitoring: false,
  created: new Date("2025-01-08T08:05:00Z"), users: ["ploi"],
  sites: [
    { domain: "status.example.net", type: "php", deployed: hoursAgo(50), disk: mbSize(50) },
  ],
});

// -- atlas (load-balanced API cluster) ---------------------------------------

addServer({
  name: "atlas-api-prod-01", php: 8.3, mysql: 8.0,
  created: new Date("2024-04-03T11:30:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "api.atlas.example.io", type: "laravel", quick: true, user: "deploy",
      health: "https://api.atlas.example.io/health", deployed: minutesAgo(17), disk: gbSize(1.9) },
    { domain: "api-v2.atlas.example.io", type: "laravel", quick: true, user: "deploy",
      deployed: hoursAgo(9), disk: gbSize(1.1) },
  ],
});

addServer({
  name: "atlas-api-prod-02", php: 8.3, mysql: 8.0,
  created: new Date("2024-04-03T11:45:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "api.atlas.example.io", type: "laravel", quick: true, user: "deploy",
      health: "https://api.atlas.example.io/health", deployed: minutesAgo(17), disk: gbSize(1.8) },
    { domain: "legacy-v1.atlas.example.io", type: "php", php: 8.1, user: "deploy",
      deployed: hoursAgo(310), disk: mbSize(340) },
  ],
});

addServer({
  name: "atlas-api-staging", php: 8.4, mysql: 8.0,
  created: new Date("2024-06-11T14:20:00Z"), users: ["ploi"],
  sites: [
    { domain: "staging.api.atlas.example.io", type: "laravel",
      deployed: hoursAgo(4), disk: gbSize(0.9) },
    { domain: "demo.api.atlas.example.io", type: "laravel",
      deployed: hoursAgo(52), disk: mbSize(210) },
  ],
});

addServer({
  name: "atlas-docs", php: 8.2, monitoring: false,
  created: new Date("2024-09-19T09:05:00Z"), users: ["ploi"],
  sites: [
    { domain: "docs.atlas.example.io", type: "static",
      deployed: hoursAgo(96), disk: mbSize(45) },
  ],
});

// -- horizon (web frontends) --------------------------------------------------

addServer({
  name: "horizon-web-prod-01", php: 8.3, mysql: 8.0,
  created: new Date("2024-05-02T08:10:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "horizon.example.io", type: "laravel", quick: true, user: "deploy",
      health: "https://horizon.example.io/up", deployed: minutesAgo(88), disk: gbSize(3.1) },
    { domain: "www.horizon.example.io", type: "php", user: "deploy",
      deployed: minutesAgo(88), disk: mbSize(4) },
    { domain: "assets.horizon.example.io", type: "static",
      deployed: hoursAgo(140), disk: mbSize(780) },
  ],
});

addServer({
  name: "horizon-web-prod-02", php: 8.3, mysql: 8.0,
  created: new Date("2024-05-02T08:25:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "horizon.example.io", type: "laravel", quick: true, user: "deploy",
      health: "https://horizon.example.io/up", deployed: hoursAgo(1), disk: gbSize(3.0) },
    { domain: "www.horizon.example.io", type: "php", user: "deploy",
      deployed: hoursAgo(1), disk: mbSize(4) },
    { domain: "assets.horizon.example.io", type: "static",
      deployed: hoursAgo(140), disk: mbSize(774) },
  ],
});

addServer({
  name: "horizon-web-staging", php: 8.2, mysql: 8.0, monitoring: false,
  created: new Date("2024-07-30T16:40:00Z"), users: ["ploi"],
  sites: [
    { domain: "staging.horizon.example.io", type: "laravel",
      deployed: hoursAgo(9), disk: gbSize(1.2) },
  ],
});

// -- lumen (queue workers) -----------------------------------------------------

addServer({
  name: "lumen-worker-01", php: 8.3,
  created: new Date("2024-08-14T10:00:00Z"), users: ["ploi"],
  sites: [
    { domain: "workers.lumen.example.io", type: "php",
      deployed: hoursAgo(11), disk: mbSize(38) },
  ],
});

addServer({
  name: "lumen-worker-02", php: 8.3,
  created: new Date("2024-08-14T10:15:00Z"), users: ["ploi"],
  sites: [
    { domain: "workers.lumen.example.io", type: "php",
      deployed: hoursAgo(11), disk: mbSize(41) },
  ],
});

addServer({
  name: "lumen-worker-03", php: 8.2,
  created: new Date("2024-10-02T13:55:00Z"), users: ["ploi"],
  sites: [
    { domain: "jobs.lumen.example.io", type: "php",
      deployed: hoursAgo(63), disk: mbSize(55) },
  ],
});

addServer({
  name: "lumen-worker-04", php: 8.3,
  created: new Date("2025-02-20T09:35:00Z"), users: ["ploi"],
  sites: [
    { domain: "workers.lumen.example.io", type: "php",
      deployed: hoursAgo(6), disk: mbSize(36) },
  ],
});

// -- beacon (database nodes) ---------------------------------------------------

addServer({
  name: "beacon-db-prod-01", php: 8.2, mysql: 8.0,
  created: new Date("2024-04-20T07:50:00Z"), users: ["ploi"],
  databases: ["beacon_primary:mysql", "beacon_reporting:mysql", "beacon_archive:mysql"],
  sites: [
    { domain: "pma.beacon.example.io", type: "php",
      deployed: hoursAgo(240), disk: mbSize(30) },
  ],
});

addServer({
  name: "beacon-db-prod-02", php: 8.2, mysql: 8.4,
  created: new Date("2024-04-20T08:05:00Z"), users: ["ploi"],
  databases: ["beacon_primary:mysql", "beacon_reporting:mysql"],
  sites: [
    { domain: "pma.beacon.example.io", type: "php",
      deployed: hoursAgo(240), disk: mbSize(30) },
  ],
});

// -- relay (edge/CDN) ----------------------------------------------------------

addServer({
  name: "relay-edge-01", php: 8.3, status: "building", monitoring: false,
  created: new Date("2025-08-28T12:00:00Z"), users: ["ploi"],
  sites: [
    { domain: "cdn.relay.example.net", type: "static", status: "repository-installing",
      deployed: null, disk: mbSize(2) },
  ],
});

addServer({
  name: "relay-edge-02", php: 8.3, monitoring: false,
  created: new Date("2024-11-05T15:20:00Z"), users: ["ploi"],
  sites: [
    { domain: "cdn.relay.example.net", type: "static",
      deployed: hoursAgo(47), disk: mbSize(410) },
  ],
});

addServer({
  name: "relay-edge-03", php: 8.3, monitoring: false,
  created: new Date("2024-11-05T15:35:00Z"), users: ["ploi"],
  sites: [
    { domain: "relay-status.example.net", type: "static",
      deployed: hoursAgo(120), disk: mbSize(12) },
  ],
});

addServer({
  name: "relay-edge-04", php: 8.3, monitoring: false,
  created: new Date("2025-03-12T10:45:00Z"), users: ["ploi"],
  sites: [
    { domain: "cdn.relay.example.net", type: "static",
      deployed: hoursAgo(47), disk: mbSize(405) },
  ],
});

// -- quest (WordPress shops) ---------------------------------------------------

addServer({
  name: "quest-store-prod", php: 8.2, mysql: 8.0,
  created: new Date("2024-02-27T09:40:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "shop.quest.example.com", type: "wordpress", quick: true, user: "deploy",
      deployed: hoursAgo(2), disk: gbSize(4.6) },
    { domain: "checkout.quest.example.com", type: "wordpress", quick: true, user: "deploy",
      deployed: hoursAgo(2), disk: gbSize(2.8) },
  ],
});

addServer({
  name: "quest-store-staging", php: 8.1, mysql: 8.0, monitoring: false,
  created: new Date("2024-08-08T11:25:00Z"), users: ["ploi"],
  sites: [
    { domain: "staging.shop.quest.example.com", type: "wordpress",
      deployed: hoursAgo(81), disk: gbSize(1.9) },
  ],
});

addServer({
  name: "quest-blog", php: 8.2, mysql: 8.0,
  created: new Date("2023-12-12T14:10:00Z"), users: ["ploi"],
  sites: [
    { domain: "blog.quest.example.com", type: "wordpress", quick: true,
      deployed: hoursAgo(31), disk: gbSize(6.2) },
  ],
});

// -- compass (internal tools) ---------------------------------------------------

addServer({
  name: "compass-admin", php: 8.3, mysql: 8.0,
  created: new Date("2024-06-25T10:15:00Z"), users: ["ploi", "deploy"],
  sites: [
    { domain: "admin.compass.example.io", type: "laravel", quick: true,
      health: "https://admin.compass.example.io/health", deployed: hoursAgo(6), disk: gbSize(0.8) },
    { domain: "tracker.compass.example.io", type: "laravel",
      deployed: hoursAgo(28), disk: gbSize(1.4) },
    { domain: "wiki.compass.example.io", type: "php",
      deployed: hoursAgo(210), disk: mbSize(320) },
  ],
});

// -- one-offs -------------------------------------------------------------------

addServer({
  name: "sentinel-monitoring", php: 8.3,
  created: new Date("2024-09-30T08:30:00Z"), users: ["ploi"],
  sites: [
    { domain: "uptime.sentinel.example.io", type: "php",
      health: "https://uptime.sentinel.example.io", deployed: hoursAgo(14), disk: mbSize(95) },
  ],
});

addServer({
  name: "aurora-analytics", php: 8.3, mysql: 8.0,
  created: new Date("2024-10-16T13:05:00Z"), users: ["ploi", "deploy"],
  databases: ["events:postgres", "metrics:postgres", "aurora_warehouse:postgres"],
  sites: [
    { domain: "dash.aurora.example.io", type: "laravel", quick: true,
      deployed: hoursAgo(5), disk: gbSize(1.6) },
    { domain: "ingest.aurora.example.io", type: "php",
      deployed: hoursAgo(8), disk: mbSize(140) },
  ],
});

addServer({
  name: "ember-cron", php: 8.2,
  created: new Date("2025-01-22T09:20:00Z"), users: ["ploi"],
  sites: [
    { domain: "cron.ember.example.io", type: "php",
      deployed: hoursAgo(44), disk: mbSize(28) },
  ],
});

addServer({
  name: "kestrel-mail", php: 8.2, status: "refreshing",
  created: new Date("2024-05-09T07:35:00Z"), users: ["ploi"],
  sites: [
    { domain: "mail.kestrel.example.io", type: "php",
      deployed: hoursAgo(510), disk: mbSize(64) },
  ],
});

addServer({
  name: "harbor-backup", php: 8.1, mysql: 8.0, status: "unreachable",
  created: new Date("2023-11-30T10:50:00Z"), users: ["ploi"],
  sites: [
    { domain: "backup.harbor.example.io", type: "php",
      deployed: hoursAgo(320), disk: gbSize(1.2) },
  ],
});

addServer({
  name: "meridian-sandbox", php: 8.4, monitoring: false,
  created: new Date("2025-09-01T14:30:00Z"), users: ["ploi"],
  sites: [
    { domain: "lab.meridian.example.dev", type: "php",
      deployed: hoursAgo(1), disk: mbSize(22) },
    { domain: "spike.meridian.example.dev", type: "php", status: "deploy-failed",
      deployed: null, disk: mbSize(5) },
  ],
});

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
const findSite = (serverID, siteID) => (sitesByServer[Number(serverID)] || []).find((s) => s.id === Number(siteID));

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
    return ok(list(databasesByServer[server.id] || []));
  }],

  ["GET", /^\/api\/servers\/(\d+)\/system-users$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    return ok(list(systemUsersByServer[server.id] || []));
  }],

  ["GET", /^\/api\/servers\/(\d+)\/sites$/, (m) => {
    const server = findServer(m[1]);
    if (!server) return notFound("Server");
    return ok(list(sitesByServer[server.id] || []));
  }],

  ["GET", /^\/api\/servers\/(\d+)\/sites\/(\d+)$/, (m) => {
    if (!findServer(m[1])) return notFound("Server");
    const site = findSite(m[1], m[2]);
    return site ? ok(single(site)) : notFound("Site");
  }],

  ["GET", /^\/api\/servers\/(\d+)\/sites\/(\d+)\/certificates$/, (m) => {
    if (!findServer(m[1])) return notFound("Server");
    if (!findSite(m[1], m[2])) return notFound("Site");
    return ok(list(certificatesBySite[`${Number(m[1])}-${Number(m[2])}`] || []));
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
  const siteTotal = Object.values(sitesByServer).reduce((n, s) => n + s.length, 0);
  console.log(`ploi-tui mock API listening on http://${HOST}:${PORT}`);
  console.log(`Serving ${servers.length} fictional servers / ${siteTotal} sites. All data is fake.`);
  console.log("Start the TUI with:");
  console.log(`  XDG_CACHE_HOME="$(mktemp -d)" PLOI_TUI_API_URL=http://${HOST}:${PORT} ./bin/ploi-tui`);
  console.log("Or simply: ./mock/demo.sh");
});
