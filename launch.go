package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type LaunchSpec struct {
	JavaBin    string
	MainClass  string
	Classpath  []string
	MC         *MCResolved
	AssetID    string
	Log4jCfg   string
	NativesDir string
}

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
		return
	}
	lines := strings.Split(string(b), "\n")
	changed := false
	for _, kv := range [][2]string{
		{"exclusiveFullscreen", "false"},
		{"preferredGraphicsBackend", `"opengl"`},
	} {
		key, val := kv[0], kv[1]
		found := false
		for i, ln := range lines {
			if strings.HasPrefix(strings.TrimSpace(ln), key+":") {
				found = true
				if strings.TrimSpace(ln) != key+":"+val {
					lines[i] = key + ":" + val
					changed = true
				}
			}
		}
		if !found {
			lines = append(lines, key+":"+val)
			changed = true
		}
	}
	if changed {
		a.Log.Printf("    Hinweis: OpenGL erzwungen (Minecraft 26.3 Vulkan-Absturz auf AMD umgangen)")
		_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
	a.patchSodiumOptions()
}

func (a *App) patchSodiumOptions() {
	path := filepath.Join(a.Paths.Config, "sodium-options.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s := string(b)
	if strings.Contains(s, `"use_no_error_g_l_context": true`) {
		s = strings.Replace(s, `"use_no_error_g_l_context": true`, `"use_no_error_g_l_context": false`, 1)
		if err := os.WriteFile(path, []byte(s), 0o644); err == nil {
			a.Log.Printf("    Hinweis: Sodium 'no error GL context' aus (AMD-Stabilitaet)")
		}
	}
}

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

	return a.runWithRetry(spec.JavaBin, argv[1:])
}

func runGame(bin string, args []string, dir string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (a *App) runWithRetry(bin string, args []string) error {
	const attempts = 3
	var err error
	for i := 0; i < attempts; i++ {
		err = runGame(bin, args, a.Paths.Root)
		if err == nil {
			return nil
		}
		if i < attempts-1 {
			a.Log.Printf("    Spiel abgestuerzt (%v) - automatischer Neustart %d/%d ...", err, i+2, attempts)
			time.Sleep(3 * time.Second)
		}
	}
	return err
}
