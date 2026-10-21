import assert from "node:assert/strict";
import {
    existsSync,
    mkdirSync,
    mkdtempSync,
    readFileSync,
    rmSync,
    writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { isAbsolute, join } from "node:path";
import { test } from "node:test";
import { apps, buildConfig, prepareAssets } from "./deploy.mjs";

const env = {
    ENTE_WORKERS_API_ORIGIN: "https://museum.example.com",
    ENTE_WORKERS_SUBDOMAIN: "example",
};

test("Deployment rejects unknown apps and unsafe origins or Worker names", () => {
    assert.throws(
        () => buildConfig("../../secret", env),
        /Unknown deployment app/,
    );
    for (const value of [
        undefined,
        "http://museum.example.com",
        "https://user:pass@museum.example.com",
        "https://museum.example.com/api",
        "https://museum.example.com?key=secret",
        "https://museum.example.com#fragment",
    ]) {
        assert.throws(
            () =>
                buildConfig("photos", {
                    ...env,
                    ENTE_WORKERS_API_ORIGIN: value,
                }),
            /HTTPS origin/,
        );
    }
    for (const value of [
        undefined,
        "",
        "bad.domain",
        "-example",
        "example-",
        "a".repeat(64),
    ]) {
        assert.throws(
            () =>
                buildConfig("photos", {
                    ...env,
                    ENTE_WORKERS_SUBDOMAIN: value,
                }),
            /DNS label/,
        );
    }
    for (const value of ["", "../escape", "bad domain", "a".repeat(60)]) {
        assert.throws(
            () =>
                buildConfig("photos", {
                    ...env,
                    ENTE_WEB_WORKER_PREFIX: value,
                }),
            /valid Worker names/,
        );
    }
});

test("All frontend exports use absolute assets and only SPA apps have fallback", () => {
    assert.equal(apps.length, 16);
    for (const app of apps) {
        const config = buildConfig(app, env);
        assert.equal(config.name, `ente-web-${app}`);
        assert.equal(config.workers_dev, true);
        assert.equal(config.main, undefined);
        assert.equal(isAbsolute(config.assets.directory), true);
        assert.equal(
            config.assets.not_found_handling,
            [
                "legacy",
                "memories",
                "paste",
                "payments",
                "share",
                "space",
                "twoof3",
            ].includes(app)
                ? "single-page-application"
                : "404-page",
        );
        assert.equal(
            config.assets.directory.endsWith(
                app === "docs"
                    ? "/server/workers/.wrangler/deploy/docs-assets"
                    : `/web/apps/${app}/${app === "payments" ? "dist" : "out"}`,
            ),
            true,
        );
    }
});

test("Workers replaces index.html catch-all rewrites with SPA fallback and retains other redirects", (t) => {
    const directory = mkdtempSync(join(tmpdir(), "ente-deploy-redirects-"));
    t.after(() => rmSync(directory, { recursive: true, force: true }));
    for (const app of apps.filter((app) => app !== "docs")) {
        const source = new URL(
            `../../web/apps/${app}/public/`,
            import.meta.url,
        );
        const redirects = new URL("_redirects", source);
        if (!existsSync(redirects)) continue;
        const original = readFileSync(redirects, "utf8");
        const config = buildConfig(app, env);
        config.assets.directory = join(directory, app);
        mkdirSync(config.assets.directory);
        prepareAssets(app, config, env);
        const generated = readFileSync(
            join(config.assets.directory, "_redirects"),
            "utf8",
        );
        const rules = (value) =>
            value.split(/\r?\n/).filter((line) => line.trim());
        const catchAll = (line) => /^\/\*\s+\/index\.html\s+200\s*$/.test(line);
        const expected = rules(original).filter((line) => !catchAll(line));
        const pattern = (line) => /[*:]/.test(line.split(/\s+/)[0]);
        assert.deepEqual(rules(decodeURI(generated)), [
            ...expected.filter((line) => !pattern(line)),
            ...expected.filter(pattern),
        ]);
        if (app === "space") {
            assert.match(
                generated,
                /\/app\/messages\/\*\s+\/app\/messages\/%5BspaceId%5D\s+200/,
            );
            assert.match(
                generated,
                /\/app\/posts\/\*\s+\/app\/posts\/%5BspaceId%5D\/%5BpostId%5D\s+200/,
            );
        }
        if (rules(original).some(catchAll)) {
            assert.equal(
                config.assets.not_found_handling,
                "single-page-application",
            );
        }
        assert.equal(readFileSync(redirects, "utf8"), original);
        prepareAssets(app, config, env);
        assert.equal(
            readFileSync(join(config.assets.directory, "_redirects"), "utf8"),
            generated,
        );
    }
});

test("Space direct links serve exported templates without canonical redirect loops", async () => {
    const directory = mkdtempSync(join(tmpdir(), "ente-deploy-space-routing-"));
    let worker;
    try {
        const config = buildConfig("space", env);
        config.name = "ente-deploy-routing-test";
        config.assets.directory = join(directory, "assets");
        const fixtures = [
            ["index.html", "home"],
            ["profile-link.html", "profile"],
            ["ios-pwa.js", "console.log('pwa');"],
            ["sw.js", "console.log('service-worker');"],
            ["manifest.webmanifest", '{"name":"Space"}'],
            ["app/messages/[spaceId].html", "message"],
            ["app/posts/[spaceId]/[postId].html", "post"],
        ];
        for (const [filename, body] of fixtures) {
            const target = join(config.assets.directory, filename);
            mkdirSync(join(target, ".."), { recursive: true });
            writeFileSync(target, body);
        }
        prepareAssets("space", config, env);
        const filename = join(directory, "wrangler.json");
        writeFileSync(filename, JSON.stringify(config));
        const { unstable_startWorker } = await import("wrangler");
        worker = await unstable_startWorker({
            config: filename,
            dev: { persist: false },
        });
        for (const [path, body] of [
            ["/", "home"],
            ["/alice", "profile"],
            ["/ios-pwa.js", "console.log('pwa');"],
            ["/sw.js", "console.log('service-worker');"],
            ["/manifest.webmanifest", '{"name":"Space"}'],
            ["/app/messages/space-123", "message"],
            ["/app/posts/space-123/post-456", "post"],
        ]) {
            const response = await worker.fetch(`http://localhost${path}`, {
                redirect: "manual",
                headers: { "Sec-Fetch-Mode": "navigate" },
            });
            assert.equal(
                response.status,
                200,
                `${path}: ${response.headers.get("location")}`,
            );
            assert.equal(await response.text(), body);
        }
    } finally {
        await worker?.dispose();
        rmSync(directory, { recursive: true, force: true });
    }
});

test("Album and embed exports serve robots.txt before public-link rewrites", async () => {
    const directory = mkdtempSync(
        join(tmpdir(), "ente-deploy-public-routing-"),
    );
    try {
        const { unstable_startWorker } = await import("wrangler");
        for (const app of ["albums", "embed"]) {
            const config = buildConfig(app, env);
            config.assets.directory = join(directory, app);
            mkdirSync(config.assets.directory);
            writeFileSync(
                join(config.assets.directory, "index.html"),
                "public album",
            );
            const robotsContent = readFileSync(
                new URL(
                    `../../web/apps/${app}/public/robots.txt`,
                    import.meta.url,
                ),
                "utf8",
            );
            writeFileSync(
                join(config.assets.directory, "robots.txt"),
                robotsContent,
            );
            prepareAssets(app, config, env);
            const filename = join(directory, `${app}.json`);
            writeFileSync(filename, JSON.stringify(config));
            const worker = await unstable_startWorker({
                config: filename,
                dev: { persist: false },
            });
            try {
                const robots = await worker.fetch(
                    "http://localhost/robots.txt",
                );
                assert.equal(robots.status, 200);
                assert.equal(await robots.text(), robotsContent);
                const link = await worker.fetch(
                    "http://localhost/public-token",
                    { redirect: "manual" },
                );
                assert.equal(link.status, 200);
                assert.equal(await link.text(), "public album");
            } finally {
                await worker.dispose();
            }
        }
    } finally {
        rmSync(directory, { recursive: true, force: true });
    }
});

test("Museum keeps its Durable Object identity, build directory, public apps, and custom domain", () => {
    const original = JSON.parse(
        readFileSync(new URL("./wrangler.jsonc", import.meta.url), "utf8"),
    );
    const config = buildConfig("museum", env);
    assert.equal(config.name, original.name);
    assert.deepEqual(config.durable_objects, original.durable_objects);
    assert.deepEqual(config.migrations, original.migrations);
    assert.equal(isAbsolute(config.main), true);
    assert.equal(isAbsolute(config.build.cwd), true);
    assert.equal(config.build.command, original.build.command);
    assert.equal(config.keep_vars, true);
    assert.equal(
        config.vars.ENTE_APPS_PUBLIC_ALBUMS,
        "https://ente-web-albums.example.workers.dev",
    );
    assert.equal(
        config.vars.ENTE_APPS_ACCOUNTS,
        "https://ente-web-accounts.example.workers.dev",
    );
    assert.equal(
        config.vars.ENTE_APPS_FAMILY,
        "https://ente-web-accounts.example.workers.dev/family",
    );
    assert.equal(
        Object.keys(config.vars).filter((key) => key.startsWith("ENTE_APPS_"))
            .length,
        10,
    );
    assert.ok(
        config.routes.some(
            (route) =>
                route.pattern === "museum.example.com" &&
                route.custom_domain === true,
        ),
    );
    assert.equal(
        Object.keys(config.vars).some((key) =>
            /TOKEN|SECRET|ENCRYPTION/.test(key),
        ),
        false,
    );
    assert.throws(
        () =>
            buildConfig("museum", {
                ...env,
                ENTE_WORKERS_API_ORIGIN: "https://museum.example.com:8443",
            }),
        /cannot specify a port/,
    );
});

test("Museum workers.dev origins must match the existing Worker, while custom frontend prefixes propagate", () => {
    const config = buildConfig("museum", {
        ...env,
        ENTE_WORKERS_API_ORIGIN: "https://ente-museum.example.workers.dev",
        ENTE_WEB_WORKER_PREFIX: "my-web",
    });
    assert.equal(config.name, "ente-museum");
    assert.equal(
        config.vars.ENTE_APPS_SPACE,
        "https://my-web-space.example.workers.dev",
    );
    assert.throws(
        () =>
            buildConfig("museum", {
                ...env,
                ENTE_WORKERS_API_ORIGIN:
                    "https://new-museum.example.workers.dev",
            }),
        /existing Museum Worker/,
    );
});

test("Docs assets retain the built /help/ base path and remove stale staging files", (t) => {
    const directory = mkdtempSync(join(tmpdir(), "ente-deploy-docs-"));
    t.after(() => rmSync(directory, { recursive: true, force: true }));
    const source = join(directory, "source");
    const destination = join(directory, "destination");
    mkdirSync(join(source, "assets"), { recursive: true });
    writeFileSync(
        join(source, "index.html"),
        '<script src="/help/assets/app.js"></script>',
    );
    writeFileSync(join(source, "assets/app.js"), 'console.log("docs");');
    mkdirSync(destination);
    writeFileSync(join(destination, "stale.html"), "removed page");
    const config = buildConfig("docs", env);
    config.assets.directory = destination;
    prepareAssets("docs", config, env, source);
    assert.equal(
        readFileSync(join(destination, "_redirects"), "utf8"),
        "/ /help/ 302\n",
    );
    assert.match(
        readFileSync(join(destination, "help/index.html"), "utf8"),
        /\/help\/assets\/app.js/,
    );
    assert.equal(
        readFileSync(join(destination, "help/assets/app.js"), "utf8"),
        'console.log("docs");',
    );
    assert.equal(existsSync(join(destination, "stale.html")), false);
    assert.equal(existsSync(join(source, "index.html")), true);
});

test("Space adds only exact API/R2 origins to its generated CSP, preserving source and other directives", (t) => {
    const directory = mkdtempSync(join(tmpdir(), "ente-deploy-space-"));
    t.after(() => rmSync(directory, { recursive: true, force: true }));
    const source = join(directory, "source");
    const destination = join(directory, "destination");
    mkdirSync(source);
    mkdirSync(destination);
    const headers = readFileSync(
        new URL("../../web/apps/space/public/_headers", import.meta.url),
        "utf8",
    );
    writeFileSync(join(source, "_headers"), headers);
    const config = buildConfig("space", env);
    config.assets.directory = destination;
    const accountID = "a".repeat(32);
    const deployEnv = { ...env, CLOUDFLARE_ACCOUNT_ID: accountID };
    prepareAssets("space", config, deployEnv, source);
    const generated = readFileSync(join(destination, "_headers"), "utf8");
    const r2Origin = `https://${accountID}.r2.cloudflarestorage.com`;
    assert.equal(
        generated.replace(` ${env.ENTE_WORKERS_API_ORIGIN} ${r2Origin}`, ""),
        headers,
    );
    assert.equal(readFileSync(join(source, "_headers"), "utf8"), headers);
    prepareAssets("space", config, deployEnv, source);
    assert.equal(
        readFileSync(join(destination, "_headers"), "utf8"),
        generated,
    );
    writeFileSync(join(source, "_headers"), generated);
    prepareAssets("space", config, deployEnv, source);
    assert.equal(
        readFileSync(join(destination, "_headers"), "utf8"),
        generated,
    );
    for (const origin of [
        "https://museum.example.com/; script-src *",
        "https://museum.example.com?key=secret",
    ]) {
        assert.throws(
            () =>
                prepareAssets(
                    "space",
                    config,
                    { ...deployEnv, ENTE_WORKERS_API_ORIGIN: origin },
                    source,
                ),
            /HTTPS origin/,
        );
    }
    assert.throws(
        () =>
            prepareAssets(
                "space",
                config,
                {
                    ...deployEnv,
                    CLOUDFLARE_ACCOUNT_ID: "account; script-src *",
                },
                source,
            ),
        /32 hexadecimal/,
    );
    assert.equal(
        readFileSync(join(destination, "_headers"), "utf8"),
        generated,
    );
});
