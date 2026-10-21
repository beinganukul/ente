import { execFileSync } from "node:child_process";
import {
    cpSync,
    existsSync,
    mkdirSync,
    readFileSync,
    rmSync,
    writeFileSync,
} from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const workers = fileURLToPath(new URL(".", import.meta.url));
const repository = resolve(workers, "../..");

export const apps = [
    "accounts",
    "albums",
    "auth",
    "cast",
    "embed",
    "ensu",
    "legacy",
    "locker",
    "memories",
    "paste",
    "payments",
    "photos",
    "share",
    "space",
    "twoof3",
    "docs",
];

const spaApps = [
    "legacy",
    "memories",
    "paste",
    "payments",
    "share",
    "space",
    "twoof3",
];

export function buildConfig(app, env = process.env) {
    if (app !== "museum" && !apps.includes(app))
        throw new Error(`Unknown deployment app: ${app}`);
    let api;
    try {
        api = new URL(env.ENTE_WORKERS_API_ORIGIN);
    } catch {
        throw new Error("ENTE_WORKERS_API_ORIGIN must be an HTTPS origin");
    }
    if (
        api.protocol !== "https:" ||
        api.username ||
        api.password ||
        api.pathname !== "/" ||
        api.search ||
        api.hash
    ) {
        throw new Error(
            "ENTE_WORKERS_API_ORIGIN must be an HTTPS origin without credentials, path, query, or fragment",
        );
    }
    const subdomain = env.ENTE_WORKERS_SUBDOMAIN;
    const prefix = env.ENTE_WEB_WORKER_PREFIX ?? "ente-web";
    const dnsLabel = /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/;
    if (!subdomain || subdomain.length > 63 || !dnsLabel.test(subdomain))
        throw new Error("ENTE_WORKERS_SUBDOMAIN must be a DNS label");
    if (
        !dnsLabel.test(prefix) ||
        prefix.length + 1 + Math.max(...apps.map((value) => value.length)) > 63
    ) {
        throw new Error(
            "ENTE_WEB_WORKER_PREFIX must form valid Worker names of at most 63 characters",
        );
    }
    if (app !== "museum") {
        return {
            name: `${prefix}-${app}`,
            compatibility_date: "2026-10-03",
            workers_dev: true,
            assets: {
                directory:
                    app === "docs"
                        ? resolve(workers, ".wrangler/deploy/docs-assets")
                        : resolve(
                              repository,
                              `web/apps/${app}/${app === "payments" ? "dist" : "out"}`,
                          ),
                not_found_handling: spaApps.includes(app)
                    ? "single-page-application"
                    : "404-page",
            },
        };
    }

    const config = JSON.parse(
        readFileSync(resolve(workers, "wrangler.jsonc"), "utf8"),
    );
    config.main = resolve(workers, config.main);
    config.build = { ...config.build, cwd: workers };
    config.keep_vars = true;
    const origin = (slug) =>
        `https://${prefix}-${slug}.${subdomain}.workers.dev`;
    const appVariables = {
        PUBLIC_ALBUMS: "albums",
        EMBED_ALBUMS: "embed",
        PUBLIC_LOCKER: "share",
        PUBLIC_PASTE: "paste",
        CAST: "cast",
        ACCOUNTS: "accounts",
        SPACE: "space",
        PUBLIC_MEMORIES: "memories",
        LEGACY: "legacy",
    };
    config.vars = {
        ...config.vars,
        ...Object.fromEntries(
            Object.entries(appVariables).map(([key, slug]) => [
                `ENTE_APPS_${key}`,
                origin(slug),
            ]),
        ),
        ENTE_APPS_FAMILY: `${origin("accounts")}/family`,
    };
    if (!api.hostname.endsWith(".workers.dev")) {
        if (api.port)
            throw new Error("Museum custom domains cannot specify a port");
        config.routes = [
            ...(config.routes ?? []).filter(
                (route) =>
                    typeof route !== "object" || route.pattern !== api.hostname,
            ),
            { pattern: api.hostname, custom_domain: true },
        ];
    } else if (
        api.origin !== `https://${config.name}.${subdomain}.workers.dev`
    ) {
        throw new Error(
            "ENTE_WORKERS_API_ORIGIN must match the existing Museum Worker name and subdomain",
        );
    }
    return config;
}

export function prepareAssets(
    app,
    config,
    env = process.env,
    sourceDirectory = resolve(
        repository,
        app === "docs" ? "docs/docs/.vitepress/dist" : `web/apps/${app}/public`,
    ),
) {
    if (app === "museum") return;
    buildConfig(app, env);
    if (app === "docs") {
        rmSync(config.assets.directory, { recursive: true, force: true });
        mkdirSync(config.assets.directory, { recursive: true });
        cpSync(sourceDirectory, resolve(config.assets.directory, "help"), {
            recursive: true,
        });
        writeFileSync(
            resolve(config.assets.directory, "_redirects"),
            "/ /help/ 302\n",
        );
        return;
    }

    const redirects = resolve(sourceDirectory, "_redirects");
    if (existsSync(redirects)) {
        // Workers rejects index.html catch-all rewrites; use its SPA fallback.
        writeFileSync(
            resolve(config.assets.directory, "_redirects"),
            readFileSync(redirects, "utf8")
                .replace(
                    /^\/\*[ \t]+\/index\.html[ \t]+200[ \t]*(?:\r?\n|$)/gm,
                    "",
                )
                // Raw template brackets trigger Workers' canonical URL redirects.
                .replaceAll("[", "%5B")
                .replaceAll("]", "%5D"),
        );
    }
    if (app !== "space") return;

    const origins = [new URL(env.ENTE_WORKERS_API_ORIGIN).origin];
    if (env.CLOUDFLARE_ACCOUNT_ID) {
        if (!/^[a-f0-9]{32}$/i.test(env.CLOUDFLARE_ACCOUNT_ID))
            throw new Error(
                "CLOUDFLARE_ACCOUNT_ID must be 32 hexadecimal characters",
            );
        origins.push(
            `https://${env.CLOUDFLARE_ACCOUNT_ID.toLowerCase()}.r2.cloudflarestorage.com`,
        );
    }
    let policies = 0;
    const headers = readFileSync(
        resolve(sourceDirectory, "_headers"),
        "utf8",
    ).replace(
        /(Content-Security-Policy:[^\r\n]*?\bconnect-src)([^;\r\n]*)(;)/g,
        (_, directive, sources, end) => {
            policies++;
            const existing = sources.trim().split(/\s+/);
            return (
                directive +
                sources +
                origins
                    .filter((origin) => !existing.includes(origin))
                    .map((origin) => ` ${origin}`)
                    .join("") +
                end
            );
        },
    );
    if (policies !== 1)
        throw new Error("Expected one Space CSP connect-src directive");
    writeFileSync(resolve(config.assets.directory, "_headers"), headers);
}

if (
    process.argv[1] &&
    resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
    try {
        const [app, ...options] = process.argv.slice(2);
        if (
            options.some((option) =>
                /^(--(?:config|name|env)(?:=|$)|-c$|-e$)/.test(option),
            )
        ) {
            throw new Error(
                "Worker identity and config are set by the deployment app; --config, --name, and --env cannot override them",
            );
        }
        const config = buildConfig(app);
        prepareAssets(app, config);
        const directory = resolve(workers, ".wrangler/deploy");
        mkdirSync(directory, { recursive: true });
        const filename = resolve(directory, `${app}.json`);
        writeFileSync(filename, `${JSON.stringify(config, null, 2)}\n`);
        execFileSync(
            process.execPath,
            [
                resolve(workers, "node_modules/wrangler/bin/wrangler.js"),
                "deploy",
                "--config",
                filename,
                ...options,
            ],
            { cwd: workers, stdio: "inherit" },
        );
    } catch (error) {
        console.error(error.message);
        process.exitCode = 1;
    }
}
