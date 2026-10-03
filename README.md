# minecx

A fast, native Minecraft (Fabric) launcher written in **Go**.

One source tree compiles to a single, dependency-free binary for:

| Platform | Target | Notes |
|---|---|---|
| Windows 10/11 | `minecx-windows-amd64-gui.exe` (silent) or `minecx-windows-amd64.exe` | no installer needed |
| Arch Linux | `minecx-linux-amd64` | static ELF |
| SteamOS (Steam Deck) | `minecx-linux-amd64` | runs from `~/`, root FS is read-only |
| Linux ARM64 | `minecx-linux-arm64` | for ARM servers/SBCs |

`minecx` downloads the **official** Minecraft client, the Fabric loader, all
libraries, native libraries and assets from Mojang/FabricMC's public servers,
and installs a curated set of performance/feature mods from Modrinth. It then
starts the game in one step.

There is **no Python** and **no runtime dependency** (not even a system Java
install — a JDK is downloaded automatically if one is missing).

---

## ⚠️ Legal notice — read this

- You **must own** a legitimate copy of Minecraft Java Edition.
- `minecx` launches the game in **offline mode**. That is intended for
  **personal single-player use on a game you own**. Using it to avoid buying
  the game, or to impersonate an account you do not own, violates the Minecraft
  EULA and may violate the DMCA and other laws. **That is your responsibility.**
- This project contains **no Minecraft code, assets, sounds or textures**. It
  only downloads them at runtime from the official/public sources.
- Not affiliated with Mojang, Microsoft, FabricMC or Modrinth.

See [`NOTICE`](NOTICE) for the full text.

---

## Quick start

### Option A — use a prebuilt binary (easiest, recommended)

1. Open the [**Releases**](../../releases) page.
2. Download the file for your OS:
   - Windows → `minecx-windows-amd64-gui.exe` (silent, no console — recommended) or `minecx-windows-amd64.exe` (shows a console for debugging)
   - SteamOS / Arch Linux → `minecx-linux-amd64`
3. Run it.

Linux:
```bash
chmod +x minecx-linux-amd64
./minecx-linux-amd64
```

Windows (PowerShell):
```powershell
.\minecx-windows-amd64.exe
```

> **Silent vs. console (Windows):** `minecx-windows-amd64-gui.exe` is built as a
> GUI app, so **no black console/log window** ever appears (like TLauncher et al.),
> and it launches the game with `javaw`. Everything is still written to
> `<install>/logs/`, and real errors show a popup. The plain
> `minecx-windows-amd64.exe` keeps the console if you want to watch output.

The first run downloads Minecraft (a few hundred MB) and then starts it.
Every later run only downloads what changed.

### Option B — build it yourself

You need [Go](https://go.dev/dl/) 1.23 or newer. Nothing else.

```bash
# Linux / SteamOS / Arch
git clone https://github.com/92mxs21/minecx
cd minecx
./build.sh          # -> dist/minecx-linux-amd64 etc.
```

```powershell
# Windows
git clone https://github.com/92mxs21/minecx
cd minecx
.\build.ps1         # -> dist\minecx-windows-amd64.exe etc.
```

Or just:

```bash
go build -o minecx .
```

---

## Usage

```
minecx [options]
```

| Option | Meaning | Default |
|---|---|---|
| `--mc <version>` | Minecraft version, e.g. `1.21.4` | `latest` |
| `--user <name>` | Offline player name | your OS user name |
| `--skin <path\|url>` | Skin PNG (local file or URL) | off |
| `--config <file>` | Use a specific `config.txt` | auto-detect |
| `--ram <gb>` | Max Java heap | auto (½ RAM, max 8G) |
| `--dir <path>` | Install / game folder | `%APPDATA%\minecx` (Win), `~/.local/share/minecx` (Linux) |
| `--jobs <n>` | Parallel downloads | auto (2× cores, max 24) |
| `--update` | Move to newest MC + Fabric + mods | off |
| `--setup-only` | Install but do not launch | off |
| `--dry-run` | Print the final launch command | off |
| `--no-mods` | Skip installing mods | off |
| `--reset-options` | Delete `options.txt` (recover from a graphics crash) | off |
| `--clean` | Delete the install folder | off |
| `-v, --verbose` | Debug logging | off |
| `--version` | Print version | |
| `-h, --help` | Help | |

Environment variable `MINECX_HOME` sets the install folder.

Examples:

```bash
# First setup, then start
./minecx-linux-amd64

# Install only, no launch, 8 GB RAM, fixed version
./minecx-linux-amd64 --setup-only --ram 8 --mc 1.21.4

# What command would be run?
./minecx-linux-amd64 --dry-run --no-mods

# Jump everything to the newest version
./minecx-linux-amd64 --update
```

---

## `config.txt` (easiest way to set name + skin)

Put a file called `config.txt` **next to the binary** (or in the install
folder) and minecx reads it on every start. If it doesn't exist, minecx creates
it automatically with an **empty name** — you must set your name before playing:

```ini
name: YourName
skin: C:\Users\you\Pictures\skin.png
model: steve
ram: 6
mc: latest
```

| Key | Meaning |
|---|---|
| `name` | your player name (**required**, starts empty) |
| `skin` | a `.png` file **or** an `https://...` URL |
| `model` | `steve` (classic) or `alex` (slim) |
| `ram` | max RAM in GB |
| `mc` | Minecraft version |

That's it. Any key is optional; command-line flags override the file.

For skins, minecx automatically installs the **OfflineSkins** mod and drops
your PNG in `config/offlineskins/<name>.png`, so your skin shows in
single-player without any account. (In-game, `/offlineskins change <name>`
switches skins.)

---

## Where does it put files?

By default the whole game lives in one folder:

- **Windows:** `%APPDATA%\minecx`
- **Linux / SteamOS:** `~/.local/share/minecx` (respects `XDG_DATA_HOME`)

```
minecx/
├── versions/        # client jar per version
├── libraries/       # Fabric + vanilla libraries
├── assets/          # textures, sounds, languages
├── natives/         # extracted .dll / .so
├── mods/            # Modrinth mods
├── config/          # mod configs
├── saves/           # your worlds
├── runtime/         # auto-downloaded JDK (if needed)
└── logs/            # setup logs + latest game log
```

Override it with `--dir` or `MINECX_HOME` if you want it somewhere else.

---

## Why Go?

- **Native single binary**, no interpreter, no Python, no JRE required to launch.
- **Cross-compiles** to Windows, Linux/amd64 and Linux/arm64 from one codebase.
- **Zero third-party dependencies** — only the Go standard library.
- Perfect for **SteamOS**, whose root filesystem is immutable/read-only: the
  binary needs no system packages and writes everything under your home.

---

## Troubleshooting

- **"go: command not found" when building** → install Go, or just use a
  prebuilt binary from Releases.
- **Game doesn't start / crashes on launch** → check the newest log in
  `<install>/logs/`. The launcher prints a warning if any required library
  failed to download; run it again to retry.
- **Game crashes right after `Backend library: LWJGL` / `OpenGL Version`**
  → this is a known Minecraft 26.3 bug: the new **Vulkan** renderer crashes on
  many AMD setups. minecx automatically forces the OpenGL backend
  (`preferredGraphicsBackend:"opengl"`) so it starts first try. You can also
  run `minecx --reset-options`.
- **SteamOS "Read-only file system"** → you are writing next to the binary in
  a system path. Use a folder in your home, e.g.
  `./minecx-linux-amd64 --dir ~/minecx`.
- **Anti-virus flags the Windows .exe** → it's an unsigned Go binary; this is a
  common false positive. Build it yourself or review the source.

---

## License

MIT — see [`LICENSE`](LICENSE). Legal terms in [`NOTICE`](NOTICE).
