package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LaunchSpec is the fully resolved information needed to start the game.
type LaunchSpec struct {
	JavaBin    string
	MainClass  string
	Classpath  []string
	MC         *MCResolved
	AssetID    string
	Log4jCfg   string
	NativesDir string
}

// prepareOptions guards against a known crash: Minecraft 26.x uses SDL3, and on
// some AMD drivers an *exclusive* fullscreen mode causes a native access
// violation (0xc0000005) on startup. Borderless fullscreen is unaffected, so we
// force `exclusiveFullscreen:false` while leaving the user's fullscreen choice
// (`fullscreen:true/false`) untouched.
func (a *App) prepareOptions() {
	path := filepath.Join(a.Paths.Root, "options.txt")

	if a.Cfg.ResetOptions {
		if fileExists(path) {
			if err := os.Remove(path); err == nil {
				a.Log.Printf("    options.txt zurueckgesetzt (--reset-options)")
			}
		}
		return
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return // no options yet; the game creates them safely on first run
	}
	lines := strings.Split(string(b), "\n")
	found, changed := false, false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "exclusiveFullscreen:") {
			found = true
			if strings.TrimSpace(ln) != "exclusiveFullscreen:false" {
				lines[i] = "exclusiveFullscreen:false"
				changed = true
			}
		}
	}
	if !found {
		lines = append(lines, "exclusiveFullscreen:false")
		changed = true
	}
	if changed {
		a.Log.Printf("    Hinweis: exclusiveFullscreen deaktiviert (verhindert AMD/SDL3-Absturz)")
		_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
}

// buildCommand assembles the JVM and game arguments.
func (a *App) buildCommand(spec LaunchSpec) []string {
	ram := a.Cfg.RamGB
	if ram < 1 {
		ram = 2
	}
	natives := spec.NativesDir

	jvm := []string{
		"-Xms1G",
		fmt.Sprintf("-Xmx%dG", ram),
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent=30",
		"-XX:G1MaxNewSizePercent=40",
		"-XX:G1HeapRegionSize=8M",
		"-XX:G1ReservePercent=20",
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:InitiatingHeapOccupancyPercent=15",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:MaxTenuringThreshold=1",
		"--enable-native-access=ALL-UNNAMED",
		"--add-exports", "java.base/jdk.internal.misc=ALL-UNNAMED",
		"-Djava.library.path=" + natives,
		"-Djna.tmpdir=" + filepath.Join(natives, "jna"),
		"-Dio.netty.native.workdir=" + filepath.Join(natives, "netty"),
		"-Dfile.encoding=UTF-8",
		"-Dminecraft.launcher.brand=" + appName,
		"-Dminecraft.launcher.version=" + version,
	}
	if spec.Log4jCfg != "" && fileExists(spec.Log4jCfg) {
		jvm = append(jvm, "-Dlog4j.configurationFile="+spec.Log4jCfg)
	}

	game := []string{
		"--username", a.Cfg.User,
		"--version", spec.MC.ID,
		"--gameDir", a.Paths.Root,
		"--assetsDir", a.Paths.Assets,
		"--assetIndex", spec.AssetID,
		"--uuid", "00000000000000000000000000000000",
		"--accessToken", "0",
		"--clientId", "",
		"--xuid", "",
		"--versionType", "release",
	}

	cmd := append([]string{}, jvm...)
	cmd = append(cmd, "-cp", strings.Join(spec.Classpath, classpathSep()), spec.MainClass)
	cmd = append(cmd, game...)
	return append([]string{spec.JavaBin}, cmd...)
}

// launch either prints or runs the assembled command.
func (a *App) launch(ctx context.Context, spec LaunchSpec) error {
	if !a.Cfg.DryRun {
		a.prepareOptions()
	}
	argv := a.buildCommand(spec)

	if a.Cfg.DryRun {
		a.Log.Printf("[8/8] --dry-run: Startbefehl:")
		for _, arg := range argv {
			a.Log.Printf("      %s", arg)
		}
		return nil
	}
	if a.Cfg.SetupOnly {
		a.Log.Printf("[8/8] --setup-only: fertig eingerichtet, Spiel wird nicht gestartet.")
		return nil
	}

	a.Log.Printf("")
	a.Log.Printf("  Starte Minecraft %s (Fabric %s, %s, %dG)", spec.MC.ID, a.fabricLoader, a.Cfg.User, a.Cfg.RamGB)
	a.Log.Printf("  Beenden: Strg+C  |  Voice-Chat-Port: UDP 24465 (Firewall noetig)")

	cmd := exec.CommandContext(ctx, spec.JavaBin, argv[1:]...)
	cmd.Dir = a.Paths.Root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
