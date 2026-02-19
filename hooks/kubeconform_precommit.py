from __future__ import annotations

import argparse
import glob
import shutil
import subprocess
import sys

def require_kubeconform() -> None:
    if not shutil.which("kubeconform"):
        sys.exit("kubeconform not found on PATH. Install it from https://github.com/yannh/kubeconform")

def require_kustomize() -> None:
    if not shutil.which("kustomize"):
        sys.exit("kustomize not found on PATH. Install it from https://github.com/kubernetes-sigs/kustomize")

def expand_glob(files: list[str]) -> list[str]:
    expanded = []
    for pattern in files:
        matches = glob.glob(pattern, recursive=True)
        if matches:
            expanded.extend(sorted(matches))
        else:
            expanded.append(pattern)
    return expanded

def kustomize_build(path: str) -> subprocess.CompletedProcess:
    return subprocess.run(["kustomize", "build", path], capture_output=True, text=True)

def _kubeconform_cmd(extra_args: str) -> list[str]:
    cmd = ["kubeconform"]
    if extra_args:
        cmd.extend(extra_args.split())
    return cmd

def kubeconform(files: list[str], extra_args="") -> int:
    cmd = _kubeconform_cmd(extra_args)
    cmd.extend(files)
    result = subprocess.run(cmd)
    return result.returncode

def kubeconform_kustomize(files: list[str], extra_args="") -> int:
    cmd = _kubeconform_cmd(extra_args)
    paths = expand_glob(files)
    return_code = 0
    for path in paths:
        build = kustomize_build(path)
        if build.returncode != 0:
            print(build.stderr, end="", file=sys.stderr)
            return_code = 1
            continue
        result = subprocess.run(cmd, input=build.stdout, text=True)
        if result.returncode != 0:
            return_code = result.returncode
    return return_code


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Run Kubeconform on Kubernetes manifest or Kustomization files.",
    )
    parser.add_argument(
        "filenames",
        nargs="*",
        help="File paths to validate.",
    )
    parser.add_argument(
        "--kubeconform-args",
        default="",
        help="Additional arguments to pass to kubeconform (space-separated).",
    )
    parser.add_argument(
        "-k",
        "--kustomize",
        action="store_true",
        help="Whether to activate the kustomization mode or not"
    )
    args = parser.parse_args(argv)

    if not args.filenames:
        return 0

    files = args.filenames

    # Fail if dependencies are not installed
    require_kubeconform()
    if args.kustomize:
        require_kustomize()
        return kubeconform_kustomize(files, extra_args=args.kubeconform_args)

    return kubeconform(files, extra_args=args.kubeconform_args)
    

if __name__ == "__main__":
    raise SystemExit(main())
