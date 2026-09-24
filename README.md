# Kubeconform Pre-Commit

[pre-commit](https://pre-commit.com/) hooks for validating Kubernetes manifests
with [kubeconform](https://github.com/yannh/kubeconform), including rendered
Kustomize overlays.

## Provisioning

The hooks work with upstream pre-commit (3.0.0 or later) and with
[prek](https://github.com/j178/prek). The hook manager provisions an isolated
environment with the pinned Go toolchain, kubeconform, kustomize, and this
hook repository, so no manual Go, Python, Docker, or PATH setup is needed. The
first installation uses the network; later runs reuse the cached environment.
kubeconform schema sources can still use the network, depending on the flags
you configure.

## Use the hooks

### Validate manifest files

Replace `<next-release-tag>` with the first release containing this Go
migration; `v0.1.0` is the former Python hook.

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: <next-release-tag>
    hooks:
      - id: kubeconform
        files: ^manifests/
        args:
          - -strict
          - -kubernetes-version
          - "1.36.2"
```

The plain hook is native kubeconform: its configured arguments are passed to
kubeconform exactly, and the hook manager supplies matching YAML and JSON
filenames.
kubeconform rejects files that are not Kubernetes manifests (for example
`.pre-commit-config.yaml` or `renovate.json`), so restrict the hook with
`files` or `exclude`. See the
[kubeconform flags](https://github.com/yannh/kubeconform#usage).

Pin `-kubernetes-version` to the Kubernetes version targeted by your cluster
(`1.36.2` above is only an example). This makes the target version explicit,
but kubeconform's default schemas are still fetched from the `master` branch
of their upstream repository, so their contents can change. For fully
reproducible validation, point `-schema-location` at schemas pinned to an
immutable commit, or at a local schema snapshot.

### Build and validate Kustomize overlays

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: <next-release-tag>
    hooks:
      - id: kubeconform-kustomize
        args:
          - overlays/development
          - overlays/production
          - --
          - -strict
          - -schema-location
          - https://example.invalid/schemas/{{.ResourceKind}}.json
```

`kubeconform-kustomize` has `pass_filenames: false`; configure every overlay
path explicitly. It does no custom globbing, so paths are passed literally to
`kustomize build`. The first `--` separates overlay paths from native
kubeconform arguments. The separator is optional; without it, every argument
is an overlay. Arguments after it are forwarded exactly as configured (including
spaces within one YAML string), without shell parsing.

The rendered output of `kustomize build` is always what kubeconform validates,
through stdin. Arguments after `--` must therefore be kubeconform flags:
positional file or folder inputs, `-h`, and `-v` are rejected with exit `2`,
because they would make kubeconform skip the rendered overlay.

The public Kustomize hook intentionally has no file-type filter because a
generator can produce non-YAML source files. Consumers that want selective
execution can add a `files` regular expression in their own hook configuration.

### Migrating `--kubeconform-args`

For the plain hook, replace the old two-string, whitespace-split argument
style:

```yaml
- id: kubeconform
  args:
    - --kubeconform-args
    - "-strict -ignore-missing-schemas"
```

with native discrete kubeconform arguments:

```yaml
- id: kubeconform
  args:
    - -strict
    - -ignore-missing-schemas
```

For Kustomize, replace:

```yaml
- id: kubeconform-kustomize
  args:
    - overlays/development
    - --kubeconform-args
    - "-strict -ignore-missing-schemas"
```

with explicit overlay and native argument boundaries:

```yaml
- id: kubeconform-kustomize
  args:
    - overlays/development
    - --
    - -strict
    - -ignore-missing-schemas
```

## Exit behavior

For the Kustomize wrapper, no overlays prints usage and exits `2`. It resolves
both tools before processing overlays, runs `kustomize build` for each overlay,
and passes the exact rendered bytes to kubeconform stdin. It continues after
build or validation failures and exits `1` if any overlay failed; otherwise it
exits `0`. Kubeconform flags after `--` are passed as exact native
arguments; positional inputs, `-h`, and `-v` exit `2` before any overlay is
built.

## Development

```console
prek run --all-files
testdata/smoke-test.sh prek
testdata/smoke-test.sh pre-commit
```

`prek run` covers formatting, golangci-lint, `go test`, and manifest
validation. The smoke test runs both public hooks from this checkout's `HEAD`
commit against the fixtures in `testdata/`, so commit changes before running
it.

This is a Go project; it has no Python or uv package project.

## Security

Report vulnerabilities according to the
[security policy](https://github.com/neteye-platform/kubeconform-precommit/blob/main/SECURITY.md).

## License

This project is dual-licensed under MIT or Apache-2.0. You may choose either
license; see [LICENSE-MIT](LICENSE-MIT) and [LICENSE-APACHE](LICENSE-APACHE).
