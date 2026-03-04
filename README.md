# screenscope

Screenshot tool for gamescope-session on Linux.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Standard screenshot tools don't work inside gamescope-session because gamescope
runs its own nested compositor. `screenscope` captures frames via PipeWire from
gamescope's video source node, producing a PNG of whatever is currently on
screen (Steam UI, a running game, overlays, etc.).

On traditional X11 desktops, it falls back to X11 root window capture.

## Install

```sh
# Arch Linux (AUR)
yay -S screenscope-bin

# Debian / Ubuntu
sudo apt install ./screenscope_*.deb

# Fedora / RHEL
sudo dnf install ./screenscope_*.rpm

# Manual (from release tarball)
tar xzf screenscope_*_linux_amd64.tar.gz
sudo install -Dm755 screenscope /usr/local/bin/screenscope

# go install
go install github.com/dahui/screenscope/cmd/screenscope@latest
```

### Build from source

Requires `libpipewire-0.3` and `libdrm` development headers:

```sh
# Arch Linux
sudo pacman -S pipewire libdrm

# Debian / Ubuntu
sudo apt install libpipewire-0.3-dev libdrm-dev

# Fedora / RHEL
sudo dnf install pipewire-devel libdrm-devel
```

Then build:

```sh
git clone https://github.com/dahui/screenscope.git
cd screenscope
make build
sudo make install
```

## Usage

```sh
# Save to the current directory with an auto-generated filename
screenscope

# Save to a specific file
screenscope -f screenshot.png

# Save to a directory with an auto-generated filename
screenscope -d ~/Pictures

# Wait 5 seconds before capturing
screenscope --delay 5

# Combine flags
screenscope -D 3 -f ~/Pictures/steam.png
```

The output file path is printed to stdout on success. If no output flag is
given, the file is saved to the current directory as
`screenscope_YYYYMMDD_HHMMSS.png`.

### Flags

| Flag | Description |
|------|-------------|
| `-f`, `--file` | Write screenshot to this file path |
| `-d`, `--dir` | Write screenshot to this directory with an auto-generated filename |
| `-D`, `--delay` | Wait this many seconds before capturing |
| `-h`, `--help` | Show help |
| `--version` | Print version and exit |

`--file` and `--dir` are mutually exclusive.

## License

[Apache 2.0](LICENSE)
