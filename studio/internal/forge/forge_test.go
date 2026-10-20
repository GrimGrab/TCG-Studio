package forge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const forgeCmd2014 = "@echo off\r\n\r\npushd %~dp0\r\n\r\njava -version 1>nul 2>nul || (\r\n   echo no java installed\r\n)\r\n" +
	"if %jver% GEQ 17 (\r\n  java -Xmx4096m -Dio.netty.tryReflectionSetAccessible=true -Dfile.encoding=UTF-8 -jar forge-gui-desktop-2.0.14-jar-with-dependencies.jar\r\n  popd\r\n)\r\n"

func TestJvmArgs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "forge.cmd")
	if err := os.WriteFile(p, []byte(forgeCmd2014), 0o644); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(jvmArgs(p), " ")
	if got != "-Xmx4096m -Dio.netty.tryReflectionSetAccessible=true -Dfile.encoding=UTF-8" {
		t.Fatalf("jvmArgs = %q", got)
	}
	if len(jvmArgs(filepath.Join(t.TempDir(), "missing.cmd"))) != 3 {
		t.Fatal("fallback args expected")
	}
}

func TestSafeJoin(t *testing.T) {
	dst := t.TempDir()
	if _, err := safeJoin(dst, "../evil.txt"); err == nil {
		t.Fatal("path escape not caught")
	}
	if _, err := safeJoin(dst, "res/cardsfolder/cardsfolder.zip"); err != nil {
		t.Fatal(err)
	}
}

// TestUntarRelease unpacks a locally downloaded release (FORGE_TARBALL=path) — slow, so opt-in.
func TestUntarRelease(t *testing.T) {
	tarball := os.Getenv("FORGE_TARBALL")
	if tarball == "" {
		t.Skip("set FORGE_TARBALL to a forge-installer-*.tar.bz2")
	}
	dst := t.TempDir()
	if err := untarBz2(context.Background(), tarball, dst, func(rel string) bool { return !skipFiles[rel] }, func() {}); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dst, forgeJar)) || !fileExists(filepath.Join(dst, "res", "cardsfolder", "cardsfolder.zip")) {
		t.Fatal("desktop jar or card scripts missing")
	}
	if fileExists(filepath.Join(dst, mobileJar)) {
		t.Fatal("skipped file was extracted")
	}
	if got := strings.Join(jvmArgs(filepath.Join(dst, "forge.cmd")), " "); !strings.Contains(got, "-Xmx") {
		t.Fatalf("jvmArgs from release = %q", got)
	}
}

// The mod refuses to start the bridge on any Forge but the one it is built for, so Studio must install exactly that one.
func TestPinMatchesModBridge(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "src", "TCGCustomCards", "Runtime", "Mtg", "ForgeLauncher.cs"))
	if err != nil {
		t.Fatal(err)
	}
	want := `BridgeForgeVersion = "` + Version + `"`
	if !strings.Contains(string(b), want) {
		t.Fatalf("ForgeLauncher.cs has no %s: the mod's bridge and Studio's pinned Forge differ", want)
	}
}
