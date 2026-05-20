# sshelob

**A clean, modern SSH tunnel manager with beautiful TUI.**

Manage all your local/remote/dynamic port forwards from a single TUI tool — no more scattered scripts.

![Status](https://img.shields.io/badge/Status-Early%20Development-blue)
![Go](https://img.shields.io/badge/Go-1.23+-00ADD8)

## Features

- YAML configuration (single config file for MVP)
- Rich terminal UI (Bubble Tea) with live status, logs, and keyboard controls
- Start / stop / restart / status for individual tunnels, plus start-all / stop-all
- Pure Go SSH runtime with mandatory auto-reconnect
- Optional TCP health checks per tunnel
- Single static binary — works on macOS, Linux, and Windows
