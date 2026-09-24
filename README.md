# Kubeconform Pre-Commit

[pre-commit](https://pre-commit.com/) hooks for validating Kubernetes manifests
with [kubeconform](https://github.com/yannh/kubeconform), including rendered
Kustomize overlays.

## Provisioning

Consumers need pre-commit 3.0.0 or later. They need no manual kubeconform,
kustomize, Go, Python, Docker, or PATH setup. pre-commit provisions an
isolated Go hook environment containing the hook and its pinned tools. The
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
kubeconform exactly, and pre-commit supplies matching YAML and JSON filenames.
kubeconform rejects files that are not Kubernetes manifests (for example
`.pre-commit-config.yaml` or `renovate.json`), so restrict the hook with
`files` or `exclude`. See the
[kubeconform flags](https://github.com/yannh/kubeconform#usage).

For reproducible validation, pin `-kubernetes-version` to the Kubernetes
version targeted by your cluster (`1.36.2` above is only an example).
Otherwise kubeconform validates against its upstream `master` schemas, which
change over time.

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
exits `0`. Kubeconform arguments after `--` are passed as exact native
arguments.

## Development

```console
prek run --all-files
testdata/smoke-test.sh prek
testdata/smoke-test.sh pre-commit
```

`prek run` covers formatting, golangci-lint, `go test`, and manifest
validation. The smoke test runs the public `kubeconform` hook from this
checkout against the fixtures in `testdata/`.

This is a Go project; it has no Python or uv package project.

## Security

Report vulnerabilities according to the
[security policy](https://github.com/neteye-platform/kubeconform-precommit/blob/main/SECURITY.md).

## License

This project is dual-licensed under MIT or Apache-2.0. You may choose either
license; see [LICENSE-MIT](LICENSE-MIT) and [LICENSE-APACHE](LICENSE-APACHE).
