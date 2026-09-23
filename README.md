# Kubeconform Pre-Commit

[pre-commit](https://pre-commit.com/) hooks for validating Kubernetes manifests
with [kubeconform](https://github.com/yannh/kubeconform). Use the
`kubeconform` hook for manifest files, or `kubeconform-kustomize` to build and
validate Kustomize overlays.

## Prerequisites

Install [pre-commit](https://pre-commit.com/) and install
[kubeconform](https://github.com/yannh/kubeconform) separately so that it is
available on `PATH`. Kustomize mode also requires
[kustomize](https://github.com/kubernetes-sigs/kustomize) on `PATH`.

The Python hook environment does not install either native tool. Confirm the
tools visible to the hook with `command -v kubeconform` and, for Kustomize
mode, `command -v kustomize`.

## Use the hooks

### Validate manifest files

Add the normal hook to `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: v0.1.0
    hooks:
      - id: kubeconform
```

pre-commit supplies matching YAML filenames to this hook. They are passed to
`kubeconform` in that order as literal filenames; the normal hook does not
expand filename globs.

### Build and validate Kustomize overlays

Pass overlay paths explicitly to the Kustomize hook:

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: v0.1.0
    hooks:
      - id: kubeconform-kustomize
        args: [overlays/development, overlays/production]
```

This public hook has `pass_filenames: false`: changed filenames are not added
to its command. Instead, it receives only the overlay arguments configured
above. For each path it runs `kustomize build`, then sends that rendered stdout
to kubeconform on standard input.

Overlay arguments support globs. Each input pattern is processed in input
order with recursive glob matching (`**` traverses directories), and its
matches are sorted lexically before the next pattern. An unmatched pattern
remains a literal path, and duplicate matches are not removed. For example:

```yaml
args: ["overlays/*/", overlays/production]
```

### Pass kubeconform flags

Use `--kubeconform-args` with one string value:

```yaml
hooks:
  - id: kubeconform
    args:
      - --kubeconform-args
      - "-strict -ignore-missing-schemas"
```

The value is split on whitespace with Python `str.split()` and is not parsed by
a shell. Quoting-looking text is not grouped; for example, `"-schema-location
'two words'"` becomes three arguments. See the
[kubeconform flags](https://github.com/yannh/kubeconform#usage) for available
options.

## Exit behavior and troubleshooting

With no input files, the hook exits successfully before checking for tools. In
normal mode it returns kubeconform's exit status. Kustomize mode continues with
later overlays after a failure: build failures write their stderr and count as
status `1`; validation failures use kubeconform's status. The most recently
encountered failure determines the final status; later successful items do not
reset it.

If a hook reports that a tool is missing, install it outside the hook
environment and check `command -v kubeconform` or `command -v kustomize` from
the same shell that runs pre-commit. Check that configured overlay paths and
any globs resolve relative to the repository root.

From a checkout of this repository, test manually with tools already available
on `PATH`:

```console
uv run kubeconform-precommit manifest.yaml
uv run kubeconform-precommit --kustomize overlays/development
```

## Development

```console
uv sync --locked --dev
uv run pytest
uv run pre-commit validate-manifest .pre-commit-hooks.yaml
prek run --all-files
uv build
```

`prek run --all-files` requires [prek](https://github.com/j178/prek) to be
installed separately.

## Security

Report vulnerabilities according to the
[security policy](https://github.com/neteye-platform/kubeconform-precommit/blob/main/SECURITY.md).

## License

This project is dual-licensed under MIT or Apache-2.0. You may choose either
license; see [LICENSE-MIT](LICENSE-MIT) and [LICENSE-APACHE](LICENSE-APACHE).
