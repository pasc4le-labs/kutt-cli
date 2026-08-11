# kutt-cli

Minimal CLI for **self-hosted Kutt** (v2 API). Single Go file, stdlib only, no dependencies.

Built because the classic [raahii/kutt-go](https://github.com/raahii/kutt-go) CLI targets the
removed v1 API (`/api/url/*`, last commit 2022) and 404s against current Kutt instances,
which use `POST/GET /api/links` and `DELETE /api/links/:id`.

## Install

```sh
go install github.com/pasc4le-labs/kutt-cli@latest
```

## Setup

```sh
kutt setup --host https://to.example.com --apikey <your-api-key>
```

Config is saved to `~/.kutt` (`host=` / `apikey=` lines, mode 0600).
Env vars `KUTT_HOST` and `KUTT_API_KEY` override the file.
A legacy `~/.kutt` containing only a bare API key is still read.

## Usage

```
kutt submit <url> [-c custom] [-p password] [-r] [-d domain]   shorten a URL (prints the short link)
kutt list [-n limit]                               list recent links
kutt domains                                       list configured domains
kutt delete <id>                                   delete a link
```

`submit` with `-d r.peppe.dev` shortens on that custom domain; without it, Kutt's default domain is used.

## API

| Command        | HTTP call                 |
|----------------|---------------------------|
| `submit`       | `POST /api/links`         |
| `list`         | `GET /api/links?limit=N`  |
| `delete <id>`  | `DELETE /api/links/:id`   |

Auth via `X-API-Key` header (Kutt API key from Settings → API).
