"""A dependency-free curses interface for managing local Open Mind models."""

from __future__ import annotations

import argparse
import curses
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path
from typing import Sequence


MENU = (
    ("Add a model", "a"),
    ("Browse installed models", "b"),
    ("Inspect a model", "i"),
    ("Generate text", "g"),
    ("Start or stop the local API", "s"),
    ("Build the Open Mind runtime", "r"),
    ("Quit", "q"),
)


class OpenMindTUI:
    def __init__(self, runtime: Path | None, source: Path | None, host: str, port: int) -> None:
        self.runtime = runtime or self._find_runtime()
        self.source = source or self._find_source()
        self.host = host
        self.port = port
        self.server: subprocess.Popen[bytes] | None = None
        self.server_log = None
        self.status = "Ready"

    @staticmethod
    def _find_runtime() -> Path | None:
        configured = os.environ.get("OPEN_MIND_RUNTIME")
        if configured:
            return Path(configured).expanduser()
        source = OpenMindTUI._find_source()
        if source:
            candidate = source / "target" / "release" / ("om.exe" if os.name == "nt" else "om")
            if candidate.is_file():
                return candidate
        found = shutil.which("om")
        return Path(found) if found else None

    @staticmethod
    def _find_source() -> Path | None:
        configured = os.environ.get("OPEN_MIND_SOURCE")
        if configured:
            return Path(configured).expanduser()
        candidate = Path.home() / "OpenMind"
        return candidate if (candidate / "Cargo.toml").is_file() else None

    @property
    def model_directory(self) -> Path:
        home = Path(os.environ.get("OPEN_MIND_HOME", Path.home() / ".open-mind")).expanduser()
        return home / "models"

    def models(self) -> list[Path]:
        if not self.model_directory.is_dir():
            return []
        return sorted(
            (path for path in self.model_directory.iterdir() if path.is_file() and path.suffix == ".omf"),
            key=lambda path: path.name.casefold(),
        )

    def run(self) -> None:
        try:
            curses.wrapper(self._main)
        finally:
            self._stop_server()

    def _main(self, screen: curses.window) -> None:
        curses.curs_set(0)
        screen.keypad(True)
        try:
            curses.start_color()
            curses.use_default_colors()
            curses.init_pair(1, curses.COLOR_CYAN, -1)
            curses.init_pair(2, curses.COLOR_GREEN, -1)
            curses.init_pair(3, curses.COLOR_YELLOW, -1)
        except curses.error:
            pass

        selected = 0
        while True:
            selected = self._render(screen, selected)
            key = screen.getch()
            if key in (curses.KEY_UP, ord("k")):
                selected = (selected - 1) % len(MENU)
            elif key in (curses.KEY_DOWN, ord("j")):
                selected = (selected + 1) % len(MENU)
            elif key in (curses.KEY_ENTER, 10, 13):
                if self._dispatch(screen, MENU[selected][1]):
                    return
            elif key != -1:
                choice = chr(key).lower() if 0 <= key < 256 else ""
                if choice in dict((shortcut, label) for label, shortcut in MENU):
                    if self._dispatch(screen, choice):
                        return

    def _render(self, screen: curses.window, selected: int) -> int:
        screen.erase()
        height, width = screen.getmaxyx()
        if height < 18 or width < 64:
            self._write(screen, 0, 0, "Please resize the terminal to at least 64 columns by 18 rows.", width)
            screen.refresh()
            return selected

        self._write(screen, 1, 3, "OPEN MIND", width, curses.color_pair(1) | curses.A_BOLD)
        self._write(screen, 2, 3, "Local model manager  ·  OMF runtime", width)
        self._write(screen, 4, 3, f"Runtime: {self.runtime or 'not found — press R to build'}", width)
        server_state = f"running at http://{self.host}:{self.port}" if self._server_running() else "stopped"
        self._write(screen, 5, 3, f"API server: {server_state}", width)

        for index, (label, shortcut) in enumerate(MENU):
            row = 8 + index
            if row >= height - 3:
                break
            prefix = "›" if index == selected else " "
            text = f" {prefix}  [{shortcut.upper()}]  {label}"
            style = curses.A_REVERSE if index == selected else curses.A_NORMAL
            self._write(screen, row, 3, text, width, style)

        model_count = len(self.models())
        self._write(screen, height - 3, 3, f"{model_count} installed OMF model(s)  ·  {self.model_directory}", width)
        self._write(screen, height - 2, 3, self.status, width, curses.color_pair(2))
        self._write(screen, height - 1, 3, "↑/↓ or J/K move · Enter selects · Q quits", width)
        screen.refresh()
        return selected

    @staticmethod
    def _write(screen: curses.window, row: int, column: int, text: str, width: int, style: int = 0) -> None:
        if row < 0 or column >= width:
            return
        available = max(0, width - column - 1)
        try:
            screen.addnstr(row, column, text, available, style)
        except curses.error:
            pass

    def _dispatch(self, screen: curses.window, action: str) -> bool:
        try:
            if action == "q":
                return True
            if action == "a":
                self._add_model(screen)
            elif action == "b":
                self._show_models(screen)
            elif action == "i":
                self._inspect_model(screen)
            elif action == "g":
                self._generate(screen)
            elif action == "s":
                self._toggle_server(screen)
            elif action == "r":
                self._build_runtime(screen)
        except (OSError, RuntimeError, subprocess.SubprocessError) as error:
            self.status = f"Error: {error}"
            self._message(screen, "Operation failed", [str(error)])
        return False

    def _require_runtime(self) -> Path:
        if self.runtime is None or not self.runtime.is_file():
            raise RuntimeError("Runtime executable not found. Set OPEN_MIND_RUNTIME or build it with [R].")
        return self.runtime

    def _call_runtime(self, arguments: Sequence[str], timeout: int = 300) -> str:
        executable = self._require_runtime()
        result = subprocess.run(
            [str(executable), *arguments],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
            env=os.environ.copy(),
        )
        output = result.stdout.strip()
        if result.returncode:
            detail = result.stderr.strip() or output or f"runtime exited with code {result.returncode}"
            raise RuntimeError(detail)
        return output or result.stderr.strip() or "Done."

    def _add_model(self, screen: curses.window) -> None:
        filename = self._ask(screen, "Path to a local .omf file")
        if not filename:
            return
        output = self._call_runtime(["add", filename])
        self.status = output.splitlines()[-1]
        self._message(screen, "Model installed", output.splitlines())

    def _show_models(self, screen: curses.window) -> None:
        output = self._call_runtime(["list"])
        self._message(screen, "Installed models", output.splitlines() or ["No models installed yet."])

    def _inspect_model(self, screen: curses.window) -> None:
        model = self._choose_model(screen)
        if model:
            output = self._call_runtime(["show", model.name])
            self._message(screen, f"Model details: {model.name}", output.splitlines())

    def _generate(self, screen: curses.window) -> None:
        model = self._choose_model(screen)
        if not model:
            return
        prompt = self._ask(screen, "Prompt")
        if not prompt:
            return
        amount = self._ask(screen, "Maximum new tokens (default 64)")
        max_tokens = amount.strip() or "64"
        if not max_tokens.isdigit() or not 1 <= int(max_tokens) <= 1024:
            raise RuntimeError("Token count must be between 1 and 1024.")
        output = self._call_runtime(["run", model.name, "--prompt", prompt, "--max-tokens", max_tokens])
        self._message(screen, f"Generated text · {model.name}", [prompt + output])

    def _toggle_server(self, screen: curses.window) -> None:
        if self._server_running():
            self._stop_server()
            self.status = "Local API server stopped."
            self._message(screen, "API server stopped", [f"http://{self.host}:{self.port}"])
            return
        model = self._choose_model(screen)
        if not model:
            return
        executable = self._require_runtime()
        log_path = self.model_directory.parent / "logs" / "tui-server.log"
        log_path.parent.mkdir(parents=True, exist_ok=True)
        self.server_log = log_path.open("ab")
        self.server = subprocess.Popen(
            [str(executable), "serve", "--model", model.name, "--host", self.host, "--port", str(self.port)],
            stdout=self.server_log,
            stderr=subprocess.STDOUT,
            env=os.environ.copy(),
        )
        time.sleep(0.2)
        if self.server.poll() is not None:
            self._stop_server()
            raise RuntimeError(f"API server exited during startup. Check {log_path}")
        self.status = f"API server started at http://{self.host}:{self.port}"
        self._message(screen, "API server started", [self.status, f"Log: {log_path}", "Press S again to stop it."])

    def _server_running(self) -> bool:
        return self.server is not None and self.server.poll() is None

    def _stop_server(self) -> None:
        if self.server is not None:
            if self.server.poll() is None:
                self.server.terminate()
                try:
                    self.server.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    self.server.kill()
                    self.server.wait(timeout=5)
            self.server = None
        if self.server_log is not None:
            self.server_log.close()
            self.server_log = None

    def _build_runtime(self, screen: curses.window) -> None:
        if self.source is None or not (self.source / "Cargo.toml").is_file():
            raise RuntimeError("Open Mind source not found. Set OPEN_MIND_SOURCE to the monorepo root.")
        height, width = screen.getmaxyx()
        self._write(screen, height - 3, 3, "Building optimized runtime; this can take a few minutes...", width, curses.color_pair(3))
        screen.refresh()
        result = subprocess.run(
            ["cargo", "build", "--release"],
            cwd=self.source,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
            env=os.environ.copy(),
        )
        if result.returncode:
            raise RuntimeError("\n".join(result.stdout.splitlines()[-12:]))
        self.runtime = self.source / "target" / "release" / ("om.exe" if os.name == "nt" else "om")
        self.status = f"Runtime built: {self.runtime}"
        self._message(screen, "Runtime build complete", [self.status])

    def _choose_model(self, screen: curses.window) -> Path | None:
        models = self.models()
        if not models:
            self._message(screen, "No installed models", ["Use [A] Add a model to install a local .omf file first."])
            return None
        if len(models) == 1:
            return models[0]
        names = [path.name for path in models]
        self._message(screen, "Choose a model", [f"{index + 1}. {name}" for index, name in enumerate(names)])
        choice = self._ask(screen, "Model number or exact filename")
        if choice.isdigit() and 1 <= int(choice) <= len(models):
            return models[int(choice) - 1]
        for model in models:
            if choice == model.name:
                return model
        if choice:
            self.status = f"No installed model named {choice!r}."
        return None

    def _ask(self, screen: curses.window, label: str) -> str:
        height, width = screen.getmaxyx()
        row = height - 4
        self._write(screen, row, 3, f"{label}: ", width, curses.color_pair(3) | curses.A_BOLD)
        screen.refresh()
        curses.echo()
        try:
            curses.curs_set(1)
        except curses.error:
            pass
        try:
            raw = screen.getstr(row, min(width - 2, len(label) + 6), max(1, min(4096, width - len(label) - 10)))
            return raw.decode("utf-8", errors="replace").strip()
        except curses.error:
            return ""
        finally:
            curses.noecho()
            try:
                curses.curs_set(0)
            except curses.error:
                pass

    def _message(self, screen: curses.window, title: str, lines: Sequence[str]) -> None:
        height, width = screen.getmaxyx()
        screen.erase()
        self._write(screen, 1, 3, title, width, curses.color_pair(1) | curses.A_BOLD)
        visible = max(1, height - 5)
        for index, line in enumerate(lines[-visible:]):
            self._write(screen, 3 + index, 3, line, width)
        self._write(screen, height - 2, 3, "Press any key to return.", width)
        screen.refresh()
        screen.getch()


def main() -> None:
    parser = argparse.ArgumentParser(description="Keyboard-friendly local manager for Open Mind OMF models")
    parser.add_argument("--runtime", type=Path, help="path to the compiled `om` executable")
    parser.add_argument("--source", type=Path, help="Open Mind monorepo root, used by Build Runtime")
    parser.add_argument("--host", default="127.0.0.1", help="local API bind address (default: 127.0.0.1)")
    parser.add_argument("--port", type=int, default=8080, help="local API port (default: 8080)")
    args = parser.parse_args()
    if not 1 <= args.port <= 65535:
        parser.error("--port must be between 1 and 65535")
    if not sys.stdin.isatty() or not sys.stdout.isatty():
        parser.error("the TUI requires an interactive terminal")
    try:
        OpenMindTUI(args.runtime, args.source, args.host, args.port).run()
    except curses.error as error:
        print(f"Unable to start the terminal UI: {error}", file=sys.stderr)
        raise SystemExit(1) from error
