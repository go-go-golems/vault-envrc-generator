package main

import (
    "bytes"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

// This helper builds the web/ app using pnpm and copies the output into
// pkg/webserver/web/dist so it gets embedded by //go:embed.
//
// Run via: go run ./cmd/vault-envrc-generator/build-web
func main() {
    repoRoot, err := os.Getwd()
    if err != nil {
        fatalf("pwd: %v", err)
    }

    webDir := filepath.Join(repoRoot, "web")
    distSrc := filepath.Join(webDir, "dist")
    distDst := filepath.Join(repoRoot, "pkg", "webserver", "web", "dist")

    // Ensure pnpm is available
    mustRun(webDir, "node -v")
    mustRun(webDir, "pnpm -v")

    // Install and build
    mustRun(webDir, "pnpm install --silent")
    mustRun(webDir, "pnpm build")

    // Recreate destination
    if err := os.RemoveAll(distDst); err != nil {
        fatalf("remove dist dst: %v", err)
    }
    if err := os.MkdirAll(distDst, 0o755); err != nil {
        fatalf("mkdir dist dst: %v", err)
    }

    // Copy files
    mustRun(repoRoot, fmt.Sprintf("cp -a %s/. %s/", shEscape(distSrc), shEscape(distDst)))

    fmt.Println("web build complete:", distDst)
}

func mustRun(dir string, cmdline string) {
    cmd := exec.Command("bash", "-lc", cmdline)
    cmd.Dir = dir
    var out bytes.Buffer
    var errb bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &errb
    if err := cmd.Run(); err != nil {
        io.Copy(os.Stdout, &out)
        io.Copy(os.Stderr, &errb)
        fatalf("command failed: %s: %v", cmdline, err)
    }
}

func fatalf(f string, a ...any) {
    fmt.Fprintf(os.Stderr, f+"\n", a...)
    os.Exit(1)
}

func shEscape(s string) string {
    return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}


