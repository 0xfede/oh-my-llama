# Third-party licenses

Oh My Llama is an unofficial rebuild of [Ollama](https://github.com/ollama/ollama).
It is **not affiliated with, sponsored by, or endorsed by Ollama.**

Everything below is redistributed under a permissive license. There is no
copyleft component in this bundle. The authoritative license text for each
component lives in that project's own repository at the pinned revision; the
notices required for redistribution are reproduced here.

## Ollama

Oh My Llama is a modified version of Ollama. The full text is in
[LICENSE](LICENSE).

```
MIT License

Copyright (c) Ollama
```

## Bundled native components

Pinned revisions live in `LLAMA_CPP_VERSION`, `MLX_VERSION` and `MLX_C_VERSION`.

| Component | Upstream | License |
| --- | --- | --- |
| llama.cpp (and ggml) | https://github.com/ggml-org/llama.cpp | MIT — Copyright (c) 2023-2024 The ggml authors |
| MLX | https://github.com/ml-explore/mlx | MIT — Copyright (c) 2023 Apple Inc. |
| MLX-C | https://github.com/ml-explore/mlx-c | MIT — Copyright (c) 2023-2024 Apple Inc. |

## Vendored Go source

| Component | Path | License |
| --- | --- | --- |
| dialog | [app/dialog](app/dialog) | ISC — Copyright (c) 2018, the dialog authors. See [app/dialog/LICENSE](app/dialog/LICENSE). |

## Go module dependencies

Resolved dependencies are listed in [go.mod](go.mod) / [go.sum](go.sum) and are
BSD-3-Clause, MIT or Apache-2.0. Notable ones:

| Component | License |
| --- | --- |
| The Go standard library and `golang.org/x/...` | BSD-3-Clause — Copyright (c) 2009 The Go Authors |
| `github.com/mattn/go-sqlite3` | MIT — Copyright (c) 2014 Yasuhiro Matsumoto. Embeds SQLite, which is public domain. |

To regenerate a complete, machine-verified inventory of the Go dependency
licenses:

```sh
go install github.com/google/go-licenses@latest
go-licenses report ./... > go-licenses.csv
```

## Trademarks

"Ollama" and the Ollama logo are trademarks of their owner. This project uses
neither: it ships its own name, icon and bundle identifier. Copyright permission
to modify and redistribute the code (granted by the MIT license) is separate from
trademark permission, which is why this build is renamed rather than shipped as
"Ollama".
