import os
import shutil
import stat
import subprocess
import sys
import sysconfig
from pathlib import Path
from unittest.mock import Mock, call

import pytest

from hooks import kubeconform_precommit as hook


KUBECONFORM_MISSING = (
    "kubeconform not found on PATH. Install it from "
    "https://github.com/yannh/kubeconform"
)
KUSTOMIZE_MISSING = (
    "kustomize not found on PATH. Install it from "
    "https://github.com/kubernetes-sigs/kustomize"
)
REPOSITORY = Path(__file__).resolve().parents[1]


def completed(returncode=0, stdout="", stderr=""):
    return subprocess.CompletedProcess([], returncode, stdout, stderr)


def test_no_input_succeeds_without_discovering_dependencies(monkeypatch):
    which = Mock()
    monkeypatch.setattr(hook.shutil, "which", which)

    assert hook.main(["--kustomize"]) == 0
    which.assert_not_called()


def test_missing_dependencies_are_checked_in_order_with_exact_errors(monkeypatch):
    calls = []

    def which(name):
        calls.append(name)
        return None

    monkeypatch.setattr(hook.shutil, "which", which)
    with pytest.raises(SystemExit) as error:
        hook.main(["manifest.yaml"])
    assert error.value.code == KUBECONFORM_MISSING
    assert calls == ["kubeconform"]

    calls[:] = []

    def kubeconform_only(name):
        calls.append(name)
        return "/tools/kubeconform" if name == "kubeconform" else None

    monkeypatch.setattr(hook.shutil, "which", kubeconform_only)
    with pytest.raises(SystemExit) as error:
        hook.main(["-k", "overlay"])
    assert error.value.code == KUSTOMIZE_MISSING
    assert calls == ["kubeconform", "kustomize"]


@pytest.mark.parametrize("flag", ["--kustomize", "-k"])
def test_kustomize_cli_aliases_forward_kubeconform_args(monkeypatch, flag):
    which = Mock(return_value="/tools/present")
    run_kustomize = Mock(return_value=17)
    monkeypatch.setattr(hook.shutil, "which", which)
    monkeypatch.setattr(hook, "kubeconform_kustomize", run_kustomize)

    assert hook.main([flag, "--kubeconform-args=--strict", "overlay"]) == 17
    assert which.call_args_list == [call("kubeconform"), call("kustomize")]
    run_kustomize.assert_called_once_with(["overlay"], extra_args="--strict")


def test_extra_args_use_literal_split_and_normal_mode_preserves_filenames(monkeypatch):
    run = Mock(return_value=completed(7))
    monkeypatch.setattr(hook.subprocess, "run", run)

    assert hook.kubeconform(
        ["a file.yaml", "*.yaml", ""], "--schema-location 'two words'  "
    ) == 7
    run.assert_called_once_with(
        ["kubeconform", "--schema-location", "'two", "words'", "a file.yaml", "*.yaml", ""]
    )
    assert "shell" not in run.call_args.kwargs
    assert hook._kubeconform_cmd("") == ["kubeconform"]


def test_main_normal_mode_keeps_wildcard_literal_and_propagates_status(monkeypatch):
    which = Mock(return_value="/tools/kubeconform")
    glob = Mock()
    run = Mock(return_value=completed(12))
    monkeypatch.setattr(hook.shutil, "which", which)
    monkeypatch.setattr(hook.glob, "glob", glob)
    monkeypatch.setattr(hook.subprocess, "run", run)

    assert hook.main(["--kubeconform-args=--strict --summary", "literal*.yaml"]) == 12
    which.assert_called_once_with("kubeconform")
    glob.assert_not_called()
    run.assert_called_once_with(
        ["kubeconform", "--strict", "--summary", "literal*.yaml"]
    )


def test_expand_glob_sorts_each_pattern_and_keeps_unmatched_duplicates(monkeypatch):
    glob = Mock(side_effect=lambda pattern, recursive: {
        "b*": ["b2.yaml", "b1.yaml"],
        "a*": ["a2.yaml", "a1.yaml"],
        "missing": [],
    }[pattern])
    monkeypatch.setattr(hook.glob, "glob", glob)

    assert hook.expand_glob(["b*", "missing", "a*", "b*"]) == [
        "b1.yaml",
        "b2.yaml",
        "missing",
        "a1.yaml",
        "a2.yaml",
        "b1.yaml",
        "b2.yaml",
    ]
    assert glob.call_args_list == [
        call("b*", recursive=True),
        call("missing", recursive=True),
        call("a*", recursive=True),
        call("b*", recursive=True),
    ]


def test_expand_glob_uses_filesystem_matches_in_pattern_order(tmp_path, monkeypatch):
    (tmp_path / "direct.yaml").write_text("")
    items = tmp_path / "items"
    items.mkdir()
    (items / "z.yaml").write_text("")
    (items / "a.yaml").write_text("")
    nested = tmp_path / "nested"
    nested.mkdir()
    (nested / "deep.yaml").write_text("")
    monkeypatch.chdir(tmp_path)

    assert hook.expand_glob(
        ["direct.yaml", "items/*.yaml", "**/deep.yaml", "missing*.yaml", "items/*.yaml"]
    ) == [
        "direct.yaml",
        "items/a.yaml",
        "items/z.yaml",
        "nested/deep.yaml",
        "missing*.yaml",
        "items/a.yaml",
        "items/z.yaml",
    ]


def test_kustomize_build_uses_captured_text_output_without_shell(monkeypatch):
    run = Mock(return_value=completed(stdout="built"))
    monkeypatch.setattr(hook.subprocess, "run", run)

    assert hook.kustomize_build("overlay path").stdout == "built"
    run.assert_called_once_with(
        ["kustomize", "build", "overlay path"], capture_output=True, text=True
    )
    assert "shell" not in run.call_args.kwargs


def test_kustomize_failure_writes_stderr_exactly_and_skips_validation(monkeypatch, capsys):
    run = Mock(return_value=completed(2, stderr="build failed\r\n"))
    monkeypatch.setattr(hook.subprocess, "run", run)

    assert hook.kubeconform_kustomize(["broken"]) == 1
    assert capsys.readouterr().err == "build failed\r\n"
    run.assert_called_once_with(
        ["kustomize", "build", "broken"], capture_output=True, text=True
    )


def test_kustomize_continues_after_failures_and_last_failure_sets_status(monkeypatch, capsys):
    run = Mock(
        side_effect=[
            completed(0, stdout="first\n"),
            completed(6),
            completed(3, stderr="second failed\n"),
            completed(0, stdout="third\n"),
            completed(0),
        ]
    )
    monkeypatch.setattr(hook.subprocess, "run", run)

    assert hook.kubeconform_kustomize(["first", "second", "third"], "--strict") == 1
    assert capsys.readouterr().err == "second failed\n"
    assert run.call_args_list == [
        call(["kustomize", "build", "first"], capture_output=True, text=True),
        call(["kubeconform", "--strict"], input="first\n", text=True),
        call(["kustomize", "build", "second"], capture_output=True, text=True),
        call(["kustomize", "build", "third"], capture_output=True, text=True),
        call(["kubeconform", "--strict"], input="third\n", text=True),
    ]
    assert all("shell" not in item.kwargs for item in run.call_args_list)


def test_kustomize_build_failure_then_validation_failure_keeps_validation_status(
    monkeypatch, capsys
):
    run = Mock(
        side_effect=[
            completed(2, stderr="build failed\n"),
            completed(0, stdout="invalid\n", stderr="discarded build stderr\n"),
            completed(7),
            completed(0, stdout="valid\n", stderr="also discarded\n"),
            completed(0),
        ]
    )
    monkeypatch.setattr(hook.subprocess, "run", run)

    assert hook.kubeconform_kustomize(["broken", "invalid", "valid"]) == 7
    assert capsys.readouterr().err == "build failed\n"
    assert run.call_args_list == [
        call(["kustomize", "build", "broken"], capture_output=True, text=True),
        call(["kustomize", "build", "invalid"], capture_output=True, text=True),
        call(["kubeconform"], input="invalid\n", text=True),
        call(["kustomize", "build", "valid"], capture_output=True, text=True),
        call(["kubeconform"], input="valid\n", text=True),
    ]
    assert all("shell" not in item.kwargs for item in run.call_args_list)


def run_command(command, cwd, env):
    result = subprocess.run(
        command, cwd=str(cwd), env=env, capture_output=True, text=True
    )
    assert result.returncode == 0, result.stdout + result.stderr


def make_fake_tool(directory, name, source):
    path = directory / name
    path.write_text(source)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def test_pre_commit_runs_both_public_hooks_against_fake_tools(tmp_path):
    provider = tmp_path / "provider"
    consumer = tmp_path / "consumer"
    tools = tmp_path / "tools"
    provider.mkdir()
    consumer.mkdir()
    tools.mkdir()
    shutil.copy2(REPOSITORY / "pyproject.toml", provider / "pyproject.toml")
    shutil.copy2(REPOSITORY / "README.md", provider / "README.md")
    shutil.copy2(REPOSITORY / "LICENSE-MIT", provider / "LICENSE-MIT")
    shutil.copy2(REPOSITORY / "LICENSE-APACHE", provider / "LICENSE-APACHE")
    shutil.copy2(REPOSITORY / ".pre-commit-hooks.yaml", provider / ".pre-commit-hooks.yaml")
    shutil.copytree(REPOSITORY / "hooks", provider / "hooks")

    environment = os.environ.copy()
    environment["GIT_AUTHOR_NAME"] = "Tests"
    environment["GIT_AUTHOR_EMAIL"] = "tests@example.invalid"
    environment["GIT_COMMITTER_NAME"] = "Tests"
    environment["GIT_COMMITTER_EMAIL"] = "tests@example.invalid"
    environment["PIP_NO_INDEX"] = "1"
    environment["PIP_NO_BUILD_ISOLATION"] = "1"
    pip_site = tmp_path / "pip-site"
    pip_site.mkdir()
    (pip_site / "sitecustomize.py").write_text(
        "import sys\n"
        "if sys.argv[1:2] == ['install']:\n"
        "    sys.argv.insert(2, '--no-build-isolation')\n"
    )
    purelib = sysconfig.get_path("purelib")
    pythonpath = environment.get("PYTHONPATH")
    environment["PYTHONPATH"] = (
        purelib + os.pathsep + str(pip_site)
        if not pythonpath
        else purelib + os.pathsep + str(pip_site) + os.pathsep + pythonpath
    )
    run_command(["git", "init"], provider, environment)
    run_command(["git", "add", "."], provider, environment)
    run_command(
        [
            "git",
            "-c",
            "commit.gpgSign=false",
            "-c",
            "core.hooksPath=/dev/null",
            "commit",
            "-m",
            "provider",
        ],
        provider,
        environment,
    )
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"], cwd=str(provider), capture_output=True, text=True,
        check=True,
    ).stdout.strip()

    log = tmp_path / "tools.log"
    environment["FAKE_TOOL_LOG"] = str(log)
    environment["PATH"] = str(tools) + os.pathsep + environment["PATH"]
    environment["PRE_COMMIT_HOME"] = str(tmp_path / "pre-commit-home")
    make_fake_tool(
        tools,
        "kubeconform",
        "#!" + sys.executable + "\n"
        "import os, sys\n"
        "with open(os.environ['FAKE_TOOL_LOG'], 'a') as stream:\n"
        "    stream.write('kubeconform argv=' + repr(sys.argv[1:]) + ' stdin=' + repr(sys.stdin.read()) + '\\n')\n",
    )
    make_fake_tool(
        tools,
        "kustomize",
        "#!" + sys.executable + "\n"
        "import os, sys\n"
        "with open(os.environ['FAKE_TOOL_LOG'], 'a') as stream:\n"
        "    stream.write('kustomize argv=' + repr(sys.argv[1:]) + ' stdin=' + repr(sys.stdin.read()) + '\\n')\n"
        "sys.stdout.write('apiVersion: v1\\nkind: ConfigMap\\nmetadata:\\n  name: built\\n')\n",
    )

    (consumer / ".pre-commit-config.yaml").write_text(
        "repos:\n"
        "  - repo: " + str(provider) + "\n"
        "    rev: " + revision + "\n"
        "    hooks:\n"
        "      - id: kubeconform\n"
        "      - id: kubeconform-kustomize\n"
        "        args: [overlays/test]\n"
    )
    (consumer / "manifest.yaml").write_text("apiVersion: v1\nkind: Namespace\n")
    (consumer / "overlays").mkdir()
    (consumer / "overlays" / "test").mkdir()
    run_command(["git", "init"], consumer, environment)
    run_command(["git", "add", "."], consumer, environment)
    run_command(
        [
            "git",
            "-c",
            "commit.gpgSign=false",
            "-c",
            "core.hooksPath=/dev/null",
            "commit",
            "-m",
            "consumer",
        ],
        consumer,
        environment,
    )

    for hook_id in ("kubeconform", "kubeconform-kustomize"):
        run_command(
            [sys.executable, "-m", "pre_commit", "run", hook_id, "--files", "manifest.yaml"],
            consumer,
            environment,
        )

    assert log.read_text().splitlines() == [
        "kubeconform argv=['manifest.yaml'] stdin=''",
        "kustomize argv=['build', 'overlays/test'] stdin=''",
        "kubeconform argv=[] stdin='apiVersion: v1\\nkind: ConfigMap\\nmetadata:\\n  name: built\\n'",
    ]
