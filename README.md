# k8s-ui — Kubernetes TUI

Multi-cluster Kubernetes resource browser with fuzzy search, YAML view, live log streaming, and automatic CRD discovery.

## Install

```bash
git clone https://github.com/dave/kube-tui
cd kube-tui
./build.sh install
```

Ensure `~/.local/bin` is in your `PATH`.

## Usage

```
k8s-ui
```

### Navigation

| Key | Action |
|-----|--------|
| `↑`/`↓` or `k`/`j` | Navigate |
| `pgup`/`pgdown` | Page up/down |
| `enter` | Select |
| `backspace`/`esc`/`q` | Go back |
| `ctrl+c` | Quit |

### Features

**Cluster view** — Select from multiple kubeconfigs (`~/.kube/config` + `~/.kube/configs/*`).

**Type selector** — Lists all resource types including discovered CRDs. `/` to fuzzy-search by name/plural/short. Shows 7 by default with "… and N more — press / to search".

**Resource list** — All resources of the selected type with status and age. `/` to fuzzy-search by name or status.

**Detail view** — Summary with labels and status.

| Key | Action |
|-----|--------|
| `y` | Full YAML (scrollable, `/` search, `n`/`N` cycle) |
| `d` | Describe output (formatted like `kubectl describe`) |
| `l` | Live log stream |

**Log view** — Streaming logs with container selector. `/` to search.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Scroll line-by-line |
| `pgup`/`pgdown` | Page scroll |
| `f` | Toggle follow mode |
| `/` | Search, `n`/`N` cycle matches |
| `g`/`G` | Top/bottom |

## Requirements

- Go 1.21+
- `~/.kube/config` or `~/.kube/configs/*` with valid kubeconfigs
- Cluster access

## License

MIT
