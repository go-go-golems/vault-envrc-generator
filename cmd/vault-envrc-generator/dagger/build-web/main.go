package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"

    "dagger.io/dagger"
)

func main() {
    pnpmVersion := os.Getenv("WEB_PNPM_VERSION")
    if pnpmVersion == "" {
        pnpmVersion = "10.15.0"
    }

    ctx := context.Background()
    client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
    if err != nil {
        log.Fatalf("connect dagger: %v", err)
    }
    defer func() { _ = client.Close() }()

    // Working directory is pkg/webserver when invoked via //go:generate
    wd, err := os.Getwd()
    if err != nil {
        log.Fatalf("getwd: %v", err)
    }
    repoRoot := filepath.Dir(filepath.Dir(wd)) // ascend from pkg/webserver → repo root
    webPath := filepath.Join(repoRoot, "web")
    outPath := filepath.Join(wd, "web", "dist") // pkg/webserver/web/dist

    // Host directories
    webDir := client.Host().Directory(webPath)

    // Optional PNPM cache
    pnpmCacheDir := os.Getenv("PNPM_CACHE_DIR")
    if pnpmCacheDir != "" && !filepath.IsAbs(pnpmCacheDir) {
        abs, err := filepath.Abs(pnpmCacheDir)
        if err != nil {
            log.Fatalf("resolve PNPM_CACHE_DIR: %v", err)
        }
        pnpmCacheDir = abs
    }

    // Select base image
    baseImage := "node:22"
    if bi := os.Getenv("WEB_BUILDER_IMAGE"); bi != "" {
        valid := false
        if strings.Contains(bi, "@") {
            valid = true
        } else if idx := strings.LastIndex(bi, ":"); idx > 0 && idx < len(bi)-1 {
            valid = true
        }
        if valid {
            baseImage = bi
        } else {
            log.Printf("Ignoring invalid WEB_BUILDER_IMAGE=%q; using %s", bi, baseImage)
        }
    }

    base := client.Container()
    if strings.HasPrefix(baseImage, "ghcr.io/") {
        user := os.Getenv("REGISTRY_USER")
        if user == "" {
            user = os.Getenv("GHCR_USERNAME")
        }
        token := os.Getenv("REGISTRY_TOKEN")
        if token == "" {
            token = os.Getenv("GHCR_TOKEN")
        }
        if user != "" && token != "" {
            sec := client.SetSecret("ghcr_token", token)
            base = base.WithRegistryAuth("ghcr.io", user, sec)
        }
    }

    var ctr *dagger.Container
    if pnpmCacheDir != "" {
        hostCache := client.Host().Directory(pnpmCacheDir)
        ctr = base.From(baseImage).
            WithMountedDirectory("/src", webDir).
            WithWorkdir("/src").
            WithEnvVariable("PNPM_HOME", "/pnpm").
            WithMountedDirectory("/pnpm/store", hostCache)
    } else {
        pnpmCache := client.CacheVolume("pnpm-store")
        ctr = base.From(baseImage).
            WithMountedDirectory("/src", webDir).
            WithWorkdir("/src").
            WithEnvVariable("PNPM_HOME", "/pnpm").
            WithMountedCache("/pnpm/store", pnpmCache)
    }

    if os.Getenv("WEB_BUILDER_IMAGE") == "" || !strings.Contains(os.Getenv("WEB_BUILDER_IMAGE"), ":") {
        ctr = ctr.WithExec([]string{"sh", "-lc", fmt.Sprintf("corepack enable && corepack prepare pnpm@%s --activate", pnpmVersion)})
    }
    ctr = ctr.
        WithExec([]string{"sh", "-lc", "pnpm --version"}).
        WithExec([]string{"sh", "-lc", "pnpm fetch --store-dir /pnpm/store"}).
        WithExec([]string{"sh", "-lc", "pnpm install --offline --frozen-lockfile --store-dir /pnpm/store --reporter=append-only"}).
        WithExec([]string{"sh", "-lc", "pnpm vite build"})

    dist := ctr.Directory("/src/dist")
    if _, err := dist.Export(ctx, outPath); err != nil {
        log.Fatalf("export dist: %v", err)
    }
    log.Printf("exported web dist to %s", outPath)
}


