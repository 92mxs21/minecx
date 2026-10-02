package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type App struct {
	Cfg          *Config
	Paths        Paths
	Log          *Logger
	DL           *Downloader
	fabricLoader string
}

func defaultUser() string {
	for _, env := range []string{"MINECRAFT_USER", "USER", "USERNAME", "LOGNAME"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return "Player"
}

func autoJobs() int {
	n := runtime.NumCPU() * 2
	if n > 24 {
		n = 24
	}
	if n < 4 {
		n = 4
	}
	return n
}

func autoRamGB() int {
	total := totalRAMGB()
	if total <= 0 {
		return 4
	}
	ram := total / 2
	if ram > 8 {
		ram = 8
	}
	if ram < 2 {
		ram = 2
	}
	return ram
}

func usage(fs *flag.FlagSet) {
	out := fs.Output()
	fmt.Fprintf(out, `%s %s - nativer Minecraft-Fabric-Launcher (Windows / Linux / SteamOS)

  Nutzung: %s [Optionen]

  Optionen:
    --mc <version>     Minecraft-Version (Standard: latest, z.B. 1.21.4)
    --user <name>      Offline-Spielername (Standard: %s)
    --skin <pfad|url>  Skin als PNG-Datei oder URL (OfflineSkins)
    --config <datei>   config.txt verwenden
    --ram <gb>         Maximaler Java-RAM (Standard: automatisch)
    --dir <pfad>       Installations-/Spielordner (Standard: %s)
    --jobs <n>         Parallele Downloads (Standard: automatisch)
    --update           Auf neueste Minecraft-/Fabric-Version aktualisieren
    --setup-only       Nur installieren, nicht starten
    --dry-run          Startbefehl ausgeben, Spiel nicht starten
    --no-mods          Keine Mods installieren
    --reset-options    options.txt loeschen (Hilfe bei Grafik-Absturz)
    --clean            Installationsordner loeschen und beenden
    -v, --verbose      Ausfuehrliches Debug-Log
    --version          Programmversion anzeigen
    -h, --help         Diese Hilfe

  Umgebung:
    MINECX_HOME        Installationsordner (ueberschrieben von --dir)

  Rechtliches:
    Du musst Minecraft Java Edition besitzen. Offline-Modus ist nur fuer den
    persoenlichen Einzelspieler-Betrieb mit einer legalen Kopie gedacht.
    Details: NOTICE
`, appName, version, appName, defaultUser(), defaultDataDir())
}

func parseFlags(args []string) (*Config, bool) {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { usage(fs) }

	var (
		cfg     Config
		showVer bool
		help    bool
	)
	fs.StringVar(&cfg.MCVersion, "mc", "latest", "Minecraft-Version")
	fs.StringVar(&cfg.User, "user", defaultUser(), "Spielername")
	fs.StringVar(&cfg.Skin, "skin", "", "Skin (URL oder PNG-Datei)")
	fs.StringVar(&cfg.ConfigFile, "config", "", "config.txt Pfad")
	fs.IntVar(&cfg.RamGB, "ram", 0, "RAM in GB")
	fs.StringVar(&cfg.Dir, "dir", "", "Installationsordner")
	fs.IntVar(&cfg.Jobs, "jobs", 0, "Parallele Downloads")
	fs.BoolVar(&cfg.Update, "update", false, "Auf neueste Version aktualisieren")
	fs.BoolVar(&cfg.SetupOnly, "setup-only", false, "Nur installieren")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Startbefehl ausgeben")
	fs.BoolVar(&cfg.NoMods, "no-mods", false, "Keine Mods installieren")
	fs.BoolVar(&cfg.ResetOptions, "reset-options", false, "options.txt zuruecksetzen")
	fs.BoolVar(&cfg.Clean, "clean", false, "Installationsordner loeschen")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Debug-Log")
	fs.BoolVar(&cfg.Verbose, "v", false, "Debug-Log")
	fs.BoolVar(&showVer, "version", false, "Version anzeigen")
	fs.BoolVar(&help, "h", false, "Hilfe")
	fs.BoolVar(&help, "help", false, "Hilfe")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}
	if help {
		usage(fs)
		os.Exit(0)
	}
	if showVer {
		fmt.Printf("%s %s (%s/%s)\n", appName, version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if cfg.Dir == "" {
		cfg.Dir = os.Getenv("MINECX_HOME")
	}
	if cfg.Dir == "" {
		cfg.Dir = defaultDataDir()
	}

	if path := findConfigFile(cfg.ConfigFile, cfg.Dir); path != "" {
		if fc, err := parseConfigFile(path); err == nil {
			cfg.ConfigFile = path
			if fc.Name != "" && !set["user"] {
				cfg.User = fc.Name
			}
			if fc.Skin != "" && !set["skin"] {
				cfg.Skin = fc.Skin
			}
			if fc.Model != "" {
				cfg.SkinModel = fc.Model
			}
			if fc.MC != "" && !set["mc"] {
				cfg.MCVersion = fc.MC
			}
			if fc.Ram > 0 && !set["ram"] {
				cfg.RamGB = fc.Ram
			}
			if fc.Jobs > 0 && !set["jobs"] {
				cfg.Jobs = fc.Jobs
			}
		}
	}

	if strings.TrimSpace(cfg.User) == "" {
		cfg.User = defaultUser()
	}
	if cfg.Jobs <= 0 {
		cfg.Jobs = autoJobs()
	}
	if cfg.RamGB <= 0 {
		cfg.RamGB = autoRamGB()
	}
	return &cfg, true
}

func main() {
	cfg, _ := parseFlags(os.Args[1:])

	root, err := filepath.Abs(cfg.Dir)
	if err != nil {
		root = cfg.Dir
	}

	if cfg.Clean {
		fmt.Printf("Loesche %s ...\n", root)
		if err := os.RemoveAll(root); err != nil {
			fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Fertig.")
		return
	}

	paths := buildPaths(root)
	if err := paths.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Fehler: Installationsordner nicht nutzbar: %v\n", err)
		os.Exit(1)
	}

	logPath := filepath.Join(paths.Logs, "setup-"+time.Now().Format("20060102-150405")+".log")
	log := NewLogger(logPath, cfg.Verbose)
	defer log.Close()

	app := &App{Cfg: cfg, Paths: paths, Log: log}
	app.DL = NewDownloader(cfg.Jobs, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	printHeader(app)

	if err := app.run(ctx); err != nil {
		log.Printf("FEHLER: %v", err)
		log.Printf("Abbruch. Details: %s", logPath)
		os.Exit(1)
	}
}

func printHeader(a *App) {
	a.Log.Printf("================================================================")
	a.Log.Printf(" %s %s - Fabric Setup", appName, version)
	a.Log.Printf(" Zeitpunkt : %s", time.Now().Format("2006-01-02 15:04:05"))
	a.Log.Printf(" System    : %s/%s", runtime.GOOS, runtime.GOARCH)
	a.Log.Printf(" Ordner    : %s", a.Paths.Root)
	a.Log.Printf(" Parallel  : %d Downloads", a.Cfg.Jobs)
	a.Log.Printf(" RAM       : %dG", a.Cfg.RamGB)
	a.Log.Printf("================================================================")
}

func (a *App) run(ctx context.Context) error {

	a.Log.Printf("[0/8] System pruefen")
	a.Log.Printf("    Arch      : %s", runtime.GOARCH)

	a.Log.Printf("[1/8] Minecraft-Version ermitteln")
	mc, err := a.resolveMinecraft(ctx)
	if err != nil {
		return err
	}
	a.Log.Printf("    Minecraft : %s (Java %d, Assets %s)", mc.ID, mc.JavaMajor, mc.AssetID)

	a.Log.Printf("[2/8] Java bereitstellen")
	var javaBin string
	if a.Cfg.DryRun {
		if bin, major := a.findJava(mc.JavaMajor); bin != "" {
			javaBin = bin
			a.Log.Printf("    Java      : %s (Version %d)", bin, major)
		} else {
			javaBin = "java"
			a.Log.Printf("    Java      : nicht gefunden (dry-run: wuerde Temurin %d installieren)", mc.JavaMajor)
		}
	} else {
		var err error
		javaBin, err = a.ensureJava(ctx, mc.JavaMajor)
		if err != nil {
			return err
		}
	}

	a.Log.Printf("[3/8] Fabric installieren")
	fabric, err := a.resolveFabric(ctx, mc)
	if err != nil {
		return err
	}
	a.fabricLoader = fabric.Loader
	a.Log.Printf("    Loader    : %s", fabric.Loader)
	a.Log.Printf("    MainClass : %s", fabric.MainClass)

	vanillaLibs, nativeLibs := collectVanilla(mc.Version)
	classpathLibs := append(append([]ResolvedLib{}, vanillaLibs...), fabric.Libraries...)

	cpLibs := append(append([]ResolvedLib{}, classpathLibs...), nativeLibs...)

	clientJar := filepath.Join(a.Paths.Versions, mc.ID, filepath.Base(filepath.FromSlash(mc.ClientPath)))

	var clientTasks []DownloadTask
	clientTasks = append(clientTasks, DownloadTask{
		URL: mc.ClientURL, Dest: clientJar, SHA1: mc.ClientSHA1, Size: mc.ClientSize,
	})
	log4jCfg := ""
	if mc.Log4jURL != "" && mc.Log4jID != "" {
		log4jCfg = filepath.Join(a.Paths.Logs, mc.Log4jID)
		clientTasks = append(clientTasks, DownloadTask{
			URL: mc.Log4jURL, Dest: log4jCfg, SHA1: mc.Log4jSHA1,
		})
	}

	if a.Cfg.DryRun {
		spec := LaunchSpec{
			JavaBin:    javaBin,
			MainClass:  fabric.MainClass,
			Classpath:  a.classpathFor(cpLibs, clientJar),
			MC:         mc,
			AssetID:    mc.AssetID,
			Log4jCfg:   log4jCfg,
			NativesDir: a.Paths.Natives,
		}
		return a.launch(ctx, spec)
	}

	a.Log.Printf("[4/8] Client, Libraries und Natives laden")
	a.Log.Printf("    Libraries : %d JARs, Natives: %d JARs", len(classpathLibs), len(nativeLibs))

	if failures := a.DL.Run(ctx, "Libraries", toTasks(a.Paths.Libraries, classpathLibs)); len(failures) > 0 {
		for _, f := range failures {
			a.Log.Printf("    WARNUNG: %s", f)
		}
		return fmt.Errorf("es fehlen Bibliotheken - Minecraft wuerde nicht starten (erneut ausfuehren)")
	}
	if failures := a.DL.Run(ctx, "Natives", toTasks(a.Paths.Libraries, nativeLibs)); len(failures) > 0 {
		for _, f := range failures {
			a.Log.Printf("    WARNUNG: %s", f)
		}
	}
	if failures := a.DL.Run(ctx, "Client", clientTasks); len(failures) > 0 {
		for _, f := range failures {
			a.Log.Printf("    WARNUNG: %s", f)
		}
		return fmt.Errorf("Client-JAR/Log4j konnte nicht geladen werden")
	}

	a.Log.Printf("[5/8] Native Bibliotheken entpacken")
	nativeFiles := 0
	for _, lib := range nativeLibs {
		jar := filepath.Join(a.Paths.Libraries, filepath.FromSlash(lib.RelPath))
		if !fileExists(jar) {
			continue
		}
		n, err := unzipFlat(jar, a.Paths.Natives)
		if err != nil {
			a.Log.Verbosef("Natives %s: %v", jar, err)
			continue
		}
		nativeFiles += n
	}
	a.Log.Printf("    Entpackt  : %d native Dateien -> %s", nativeFiles, a.Paths.Natives)

	a.Log.Printf("[6/8] Assets (Texturen, Sounds, Sprachen)")
	indexPath := filepath.Join(a.Paths.Assets, "indexes", mc.AssetID+".json")
	if failures := a.DL.Run(ctx, "Asset-Index", []DownloadTask{{
		URL: mc.AssetURL, Dest: indexPath, SHA1: mc.AssetSHA1,
	}}); len(failures) > 0 {
		return fmt.Errorf("Asset-Index nicht ladbar: %s", failures[0])
	}
	var index AssetIndex
	if b, err := os.ReadFile(indexPath); err == nil {
		if err := json.Unmarshal(b, &index); err != nil {
			return fmt.Errorf("Asset-Index ungueltig: %w", err)
		}
	} else {
		return fmt.Errorf("Asset-Index nicht lesbar: %w", err)
	}
	a.Log.Printf("    Asset-Index: %s (%d Objekte)", mc.AssetID, len(index.Objects))
	if failures := a.DL.Run(ctx, "Assets", toTasks(a.Paths.Assets, collectAssets(index))); len(failures) > 0 {
		a.Log.Printf("    WARNUNG: %d Assets fehlen (einzelne Sounds/Texturen fehlen evtl.)", len(failures))
	}

	if a.Cfg.NoMods {
		a.Log.Printf("[7/8] Mods uebersprungen (--no-mods)")
	} else {
		if err := a.installMods(ctx, mc.ID); err != nil {
			a.Log.Printf("    WARNUNG: Mods konnten nicht vollstaendig installiert werden: %v", err)
		}
	}

	if strings.TrimSpace(a.Cfg.Skin) != "" {
		if err := a.setupSkin(ctx); err != nil {
			a.Log.Printf("    WARNUNG: Skin konnte nicht eingerichtet werden: %v", err)
		}
	}

	state := State{
		MCVersion:   mc.ID,
		Loader:      fabric.Loader,
		JavaMajor:   mc.JavaMajor,
		AssetsIndex: mc.AssetID,
		SetupTime:   time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := saveState(a.Paths.StateFile, state); err != nil {
		a.Log.Printf("    WARNUNG: Zustand nicht speicherbar: %v", err)
	}

	classpath := a.classpathFor(cpLibs, clientJar)

	spec := LaunchSpec{
		JavaBin:    javaBin,
		MainClass:  fabric.MainClass,
		Classpath:  classpath,
		MC:         mc,
		AssetID:    mc.AssetID,
		Log4jCfg:   log4jCfg,
		NativesDir: a.Paths.Natives,
	}
	return a.launch(ctx, spec)
}

func (a *App) classpathFor(libs []ResolvedLib, clientJar string) []string {
	cp := make([]string, 0, len(libs)+1)
	for _, lib := range libs {
		cp = append(cp, filepath.Join(a.Paths.Libraries, filepath.FromSlash(lib.RelPath)))
	}
	return append(cp, clientJar)
}

func toTasks(base string, libs []ResolvedLib) []DownloadTask {
	out := make([]DownloadTask, 0, len(libs))
	for _, l := range libs {
		if l.URL == "" || l.RelPath == "" {
			continue
		}
		out = append(out, DownloadTask{
			URL:  l.URL,
			Dest: filepath.Join(base, filepath.FromSlash(l.RelPath)),
			SHA1: l.SHA1,
			Size: l.Size,
		})
	}
	return out
}
