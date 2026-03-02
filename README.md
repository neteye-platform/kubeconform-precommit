# Kubeconform Pre-Commit

A [pre-commit](https://pre-commit.com/) hook to validate Kubernetes manifests
using [kubeconform](https://github.com/yannh/kubeconform).

## Prerequisites

- [kubeconform](https://github.com/yannh/kubeconform) installed and
  available on `PATH`
- [kustomize](https://github.com/kubernetes-sigs/kustomize)
  (only if using the kustomize hook)

## Usage

Add the following to your `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: v0.1.0
    hooks:
      - id: kubeconform
```

### Kustomize mode

To validate kustomize overlays, use the `kubeconform-kustomize` hook and pass
the overlay paths via `args`:

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: v0.1.0
    hooks:
      - id: kubeconform-kustomize
        args: [path/to/overlay1, path/to/overlay2]
```

### Passing extra arguments to kubeconform

Use `--kubeconform-args` to forward flags to kubeconform:

```yaml
hooks:
  - id: kubeconform
    args: [--kubeconform-args, "-strict -ignore-missing-schemas"]
```
