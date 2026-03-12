# Kubeconform Pre-Commit

[![Python 3.9+](https://img.shields.io/badge/python-3.9+-blue.svg)](https://www.python.org/downloads/)
[![License: MIT/Apache 2.0](https://img.shields.io/badge/license-MIT%2FApache%202.0-green.svg)](./LICENSE-MIT)

A [pre-commit](https://pre-commit.com/) hook to validate Kubernetes manifests
using [kubeconform](https://github.com/yannh/kubeconform).

## Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Usage](#usage)
  - [Basic Setup](#basic-setup)
  - [Kustomize Mode](#kustomize-mode)
  - [Passing Extra Arguments](#passing-extra-arguments)
- [Configuration Examples](#configuration-examples)
- [Troubleshooting](#troubleshooting)
- [License](#license)
- [Contributing](#contributing)

## Features

- ✅ Validate Kubernetes manifests automatically on commit
- 🗂️ Support for Kustomize overlays with glob patterns
- 🔧 Pass custom kubeconform flags (strict mode, schema options, etc.)
- 🚀 Lightweight Python wrapper with helpful error messages

## Prerequisites

### Required

- **Python 3.9 or higher**
- **kubeconform** installed and available on `PATH`
  
  Installation options:
  ```bash
  # macOS / Linux (Homebrew)
  brew install kubeconform
  
  # Using Go
  go install github.com/yannh/kubeconform/cmd/kubeconform@latest
  
  # Or download from releases
  # https://github.com/yannh/kubeconform/releases
  ```

### Optional

- **kustomize** (only required if using the `kubeconform-kustomize` hook)
  
  Installation options:
  ```bash
  # macOS / Linux (Homebrew)
  brew install kustomize
  
  # Using Go
  go install sigs.k8s.io/kustomize/kustomize/v5@latest
  
  # Or see: https://kubectl.docs.kubernetes.io/installation/kustomize/
  ```

## Usage

### Basic Setup

Add the following to your `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: v0.1.0
    hooks:
      - id: kubeconform
```

This will validate all YAML files committed to your repository.

### Kustomize Mode

To validate kustomize overlays, use the `kubeconform-kustomize` hook and pass
the overlay paths via `args`. Glob patterns are supported:

```yaml
repos:
  - repo: https://github.com/neteye-platform/kubeconform-precommit
    rev: v0.1.0
    hooks:
      - id: kubeconform-kustomize
        args: [path/to/overlay1, path/to/overlay2]
```

Examples with glob patterns:

```yaml
- id: kubeconform-kustomize
  args: ["overlays/*"]  # All direct subdirectories in overlays/

- id: kubeconform-kustomize
  args: ["environments/**/*"]  # Recursively find all directories
```

### Passing Extra Arguments

Use `--kubeconform-args` to forward flags to kubeconform:

```yaml
hooks:
  - id: kubeconform
    args: [--kubeconform-args, "-strict -ignore-missing-schemas"]
```

See the [kubeconform documentation](https://github.com/yannh/kubeconform#usage)
for all available flags.

## Configuration Examples

### Strict Schema Validation

Enforce strict validation (fail on additional properties):

```yaml
- id: kubeconform
  args: [--kubeconform-args, "-strict"]
```

### Ignore Missing Schemas

Skip validation for resources without available schemas:

```yaml
- id: kubeconform
  args: [--kubeconform-args, "-ignore-missing-schemas"]
```

### Multiple Kubeconform Flags

Combine multiple kubeconform options:

```yaml
- id: kubeconform
  args: [--kubeconform-args, "-strict -ignore-missing-schemas -summary"]
```

### Specify Kubernetes Version

Validate against a specific Kubernetes version:

```yaml
- id: kubeconform
  args: [--kubeconform-args, "-kubernetes-version 1.27.0"]
```

### With File Exclusions

Exclude specific files from validation:

```yaml
- id: kubeconform
  exclude: ^(examples|tests)/
```

## Troubleshooting

### "kubeconform not found on PATH"

**Error:**
```
kubeconform not found on PATH. Install it from https://github.com/yannh/kubeconform
```

**Solution:** Install kubeconform using one of the methods in the [Prerequisites](#prerequisites) section.

### "kustomize not found on PATH"

**Error:**
```
kustomize not found on PATH. Install it from https://github.com/kubernetes-sigs/kustomize
```

**Solution:** Install kustomize (required only for `kubeconform-kustomize` hook) using one of the methods in the [Prerequisites](#prerequisites) section.

### Validation Failures

If kubeconform reports validation errors, the hook will fail with a non-zero exit code. To debug:

1. **Run kubeconform manually** on the failing file:
   ```bash
   kubeconform path/to/manifest.yaml
   ```

2. **Enable verbose output** for more details:
   ```yaml
   - id: kubeconform
     args: [--kubeconform-args, "-verbose"]
   ```

3. **Check schema availability**: Some custom resources may not have public schemas. Use `-ignore-missing-schemas` to skip them:
   ```yaml
   - id: kubeconform
     args: [--kubeconform-args, "-ignore-missing-schemas"]
   ```

### Kustomize Build Failures

If using `kubeconform-kustomize` and seeing build errors, test kustomize directly:

```bash
kustomize build path/to/overlay
```

This will show any issues with your kustomize configuration.

## License

Licensed under either of:

- **Apache License, Version 2.0** ([LICENSE-APACHE](LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- **MIT License** ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

at your option.

## Contributing

Contributions are welcome! To contribute:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run pre-commit checks: `pre-commit run --all-files`
5. Commit your changes (`git commit -am 'Add new feature'`)
6. Push to the branch (`git push origin feature/my-feature`)
7. Open a pull request

For security vulnerabilities, please see [SECURITY.md](SECURITY.md) for responsible disclosure guidelines.
