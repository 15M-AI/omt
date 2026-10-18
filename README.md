# Open Mind TUI

**Open Mind TUI** is a Go terminal app for chatting with and managing local
Open Mind models. Its workflow is inspired by the simplicity of `ollama run`
and Ollama's local-first experience, but the implementation is independent:
it runs **OMF** models through the Open Mind `om` runtime and never calls or
requires Ollama.

The interface is built with Go and the Bubble Tea terminal UI framework. It is
designed for keyboard use, starts the local runtime API when you begin a chat,
and shuts down processes it started when you exit.

## What you can do

- Select, browse, inspect, and install local `.omf` models.
- Start a chat from the main screen; the selected model's local OMA server
  starts automatically.
- Start or stop the local API server separately.
- Build the Open Mind runtime from its source checkout.
- Keep model files, prompts, and inference on your machine.

## Requirements

- Go 1.24 or newer.
- A terminal with ANSI support (Linux/macOS; Windows users can use WSL).
- The Open Mind runtime and a local `.omf` model for chat/inference.

Build the runtime first:

```bash
cd ~/OpenMind
cargo build --release
```

## Build and install globally for your user

```bash
cd ~/OpenMind-TUI
make install
```

`~/.local/bin` is a user-wide install location and does not require `sudo`.
Ensure it is on your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
omt
```

To make the PATH change persistent, add the export line to `~/.bashrc` or the
startup file for your shell. Verify the executable with:

```bash
omt --version
```

## First run

Install a model using the core runtime:

```bash
cd ~/OpenMind
./target/release/om add /path/to/model.omf
```

Then launch `omt`, choose **Choose a model**, select the model, and
open **Chat**. The TUI starts `om serve` on `127.0.0.1:8080` automatically when
you enter chat. It stops that server on exit. Choose **Start / stop API server**
to control the API without opening chat.

## Keyboard controls

| Key | Action |
|---|---|
| `↑` / `↓` or `J` / `K` | Navigate the home screen and model list |
| `Enter` | Open or select the highlighted item |
| `1`–`6` | Select a home-screen action directly |
| `A` | Add a local `.omf` file from the model list |
| `I` | Inspect the highlighted model |
| `Enter` in chat | Send the current message |
| `Esc` | Return to the previous screen |
| `Ctrl+L` | Clear the current conversation |
| `Q` on the home screen / `Ctrl+C` anywhere | Quit and stop the API process started by the TUI |

## Configuration

```bash
omt \
  --runtime ~/OpenMind/target/release/om \
  --source ~/OpenMind \
  --host 127.0.0.1 \
  --port 8080
```

The app also reads `OPEN_MIND_RUNTIME` and `OPEN_MIND_SOURCE`. It automatically
looks for `~/OpenMind/target/release/om`, then `om` on `PATH`.
`OPEN_MIND_HOME` changes the shared model directory; by default the runtime and
TUI use `~/.open-mind/models`. API logs are written to
`~/.open-mind/logs/tui-server.log`.

## Development

```bash
go test ./...
go build ./cmd/omt
```

The only inference backend is the Open Mind CPU runtime and OMA API. This TUI
does not download models, support GGUF, or use Ollama as a backend. Response
quality depends on the selected model and its training data. Streaming,
conversation persistence, and authentication are not implemented yet.

## Repository name

Create one GitHub repository named **`open-mind-tui`** for this standalone Go
application. Keep the core project in the separate **`open-mind`** repository.
Replace `YOUR_USERNAME` in the GitHub URLs with your actual GitHub account or
organization name.
