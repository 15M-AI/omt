# Open Mind TUI

A lightweight, keyboard-friendly terminal interface for managing and running
local Open Mind models. It is inspired by the simple local model workflows of
tools such as Ollama, but it is an independent application for **OMF models**
and does not install, call, or require Ollama.

The TUI uses Python's standard-library `curses` module and has no third-party
Python dependencies. Model loading and inference are delegated to the Open Mind
`om` runtime.

## Features

- Install a local `.omf` model and browse installed models.
- Inspect model configuration and parameter counts.
- Generate text from a prompt in the terminal.
- Start and stop the local OMA API server.
- Build the Open Mind runtime from its source tree.
- Keyboard navigation, with one-letter shortcuts and no mouse requirement.

## Requirements

- Python 3.10+.
- A terminal with curses support (Linux/macOS; Windows users can use WSL).
- The Open Mind monorepo and its Rust runtime.

Build the runtime once, or choose **Build the Open Mind runtime** in the TUI:

```bash
cd ~/OpenMind
cargo build --release
```

## Install and launch

From this repository:

```bash
cd ~/OpenMind-TUI
python -m venv .venv
source .venv/bin/activate
pip install -e .
open-mind-tui
```

Or run without installation:

```bash
python -m open_mind_tui --runtime ~/OpenMind/target/release/om
```

The application automatically looks for `~/OpenMind/target/release/om`, then
for `om` on `PATH`. Override discovery with:

```bash
OPEN_MIND_RUNTIME=/path/to/om \
OPEN_MIND_SOURCE=/path/to/OpenMind \
open-mind-tui
```

The model home is shared with the runtime: `~/.open-mind/models`, or
`$OPEN_MIND_HOME/models` when `OPEN_MIND_HOME` is set.

## Controls

| Key | Action |
|---|---|
| `↑` / `↓` or `J` / `K` | Move through the menu |
| `Enter` | Select the highlighted action |
| `A` | Add a model from a local `.omf` file |
| `B` | Browse installed models |
| `I` | Inspect a model |
| `G` | Generate text |
| `S` | Start or stop the local OMA API |
| `R` | Build the Open Mind runtime |
| `Q` | Quit |

The API server is started as a child process and stopped when the TUI exits.
Its output is written to `~/.open-mind/logs/tui-server.log`. The server binds
to `127.0.0.1` by default; use `--host` only when you understand the security
implications.

## Limitations

This is an early local frontend, not a model downloader or hosted service.
It does not fetch model weights, support GGUF, or depend on Ollama. Generation
quality depends on the local model and its training data. The OMA API does not
yet support streaming or authentication.
