#!/usr/bin/env python3
"""Build a release zip with the binary, project docs, and dependency licenses."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile
import zipfile


ROOT = Path(__file__).resolve().parents[1]
LICENSE_PREFIXES = ("LICENSE", "COPYING", "NOTICE")
PROJECT_FILES = ("README.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "SECURITY.md", "ROADMAP.md", "config.example.json")


def run_go(go: str, *args: str) -> str:
    try:
        result = subprocess.run(
            [go, *args], cwd=ROOT, check=True, text=True, encoding="utf-8",
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
    except FileNotFoundError as exc:
        raise RuntimeError(f"Go executable not found: {go}") from exc
    except subprocess.CalledProcessError as exc:
        details = exc.stderr.strip() or exc.stdout.strip() or str(exc)
        raise RuntimeError(f"Go command failed ({' '.join(args)}): {details}") from exc
    return result.stdout


def parse_json_stream(source: str) -> list[dict[str, object]]:
    decoder = json.JSONDecoder()
    values: list[dict[str, object]] = []
    offset = 0
    while offset < len(source):
        while offset < len(source) and source[offset].isspace():
            offset += 1
        if offset == len(source):
            break
        value, offset = decoder.raw_decode(source, offset)
        if not isinstance(value, dict):
            raise RuntimeError("go list returned a non-object module record")
        values.append(value)
    return values


def safe_component(value: str, label: str) -> str:
    if not value or value in {".", ".."} or not re.fullmatch(r"[A-Za-z0-9._+-]+", value):
        raise RuntimeError(f"Invalid {label}: {value!r}")
    return value


def zip_name(value: str) -> str:
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts:
        raise RuntimeError(f"Unsafe archive path: {value!r}")
    return path.as_posix()


def root_license_files(module_dir: Path) -> list[Path]:
    return sorted(
        (path for path in module_dir.iterdir()
         if path.is_file() and path.name.upper().startswith(LICENSE_PREFIXES)),
        key=lambda path: path.name.casefold(),
    )


def package(binary: Path, version: str, target_os: str, arch: str, output: Path, go: str) -> Path:
    version = safe_component(version, "version")
    target_os = safe_component(target_os, "OS")
    arch = safe_component(arch, "architecture")
    binary = binary.resolve(strict=True)
    if not binary.is_file():
        raise RuntimeError(f"Binary is not a file: {binary}")

    # Download all selected modules so go list provides source directories for
    # license inspection, including indirect dependencies.
    run_go(go, "mod", "download", "all")
    modules = parse_json_stream(run_go(go, "list", "-m", "-json", "all"))
    root_module = next((m.get("Path") for m in modules if m.get("Main") is True), None)
    package_name = f"wxmori-{version}-{target_os}-{arch}"
    output.mkdir(parents=True, exist_ok=True)
    archive_path = output / f"{package_name}.zip"
    digest_path = output / f"{package_name}.zip.sha256"
    binary_name = "wxmori.exe" if target_os.lower() == "windows" else "wxmori"

    project_paths = [ROOT / name for name in PROJECT_FILES]
    project_paths.extend(sorted((ROOT / "docs").rglob("*")))
    missing = [str(path.relative_to(ROOT)) for path in project_paths if not path.exists()]
    if missing:
        raise RuntimeError("Missing required package inputs: " + ", ".join(missing))

    archive_tmp: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(prefix=package_name + ".", suffix=".zip", dir=output, delete=False) as temp:
            archive_tmp = Path(temp.name)
        with zipfile.ZipFile(archive_tmp, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            archive.write(binary, zip_name(f"{package_name}/{binary_name}"))
            for path in project_paths:
                if path.is_file():
                    relative = path.relative_to(ROOT).as_posix()
                    archive.write(path, zip_name(f"{package_name}/{relative}"))

            for module in modules:
                if module.get("Main") is True or module.get("Path") == root_module:
                    continue
                module_path = module.get("Path")
                module_version = module.get("Version")
                module_dir_text = module.get("Dir")
                if not isinstance(module_path, str) or not isinstance(module_version, str):
                    raise RuntimeError(f"Go returned incomplete module metadata: {module!r}")
                if not isinstance(module_dir_text, str) or not module_dir_text:
                    raise RuntimeError(f"Go did not provide a downloaded directory for {module_path}@{module_version}")
                module_dir = Path(module_dir_text)
                if not module_dir.is_dir():
                    raise RuntimeError(f"Downloaded module directory is missing for {module_path}@{module_version}: {module_dir}")
                licenses = root_license_files(module_dir)
                if not licenses:
                    raise RuntimeError(f"No root LICENSE*, COPYING*, or NOTICE* files found for {module_path}@{module_version}")
                license_root = f"{package_name}/third-party-licenses/{module_path}@{module_version}"
                for license_file in licenses:
                    archive.write(license_file, zip_name(f"{license_root}/{license_file.name}"))

        hasher = hashlib.sha256()
        with archive_tmp.open("rb") as stream:
            for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                hasher.update(chunk)
        digest = hasher.hexdigest()
        archive_tmp.replace(archive_path)
        digest_path.write_text(f"{digest}  {archive_path.name}\n", encoding="ascii", newline="\n")
        return archive_path
    finally:
        if archive_tmp is not None and archive_tmp.exists():
            archive_tmp.unlink()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path, help="built single-binary executable")
    parser.add_argument("--version", required=True, help="release version, for example v0.1.0-alpha.1")
    parser.add_argument("--os", required=True, dest="target_os", help="target OS, for example windows")
    parser.add_argument("--arch", required=True, help="target architecture, for example amd64")
    parser.add_argument("--out", required=True, type=Path, help="directory for the zip and SHA-256 file")
    parser.add_argument("--go", default="go", help="Go executable used to enumerate modules (default: go)")
    args = parser.parse_args()
    try:
        archive = package(args.binary, args.version, args.target_os, args.arch, args.out.resolve(), args.go)
    except (OSError, RuntimeError, json.JSONDecodeError, zipfile.BadZipFile) as exc:
        print(f"package: error: {exc}", file=sys.stderr)
        return 1
    print(archive)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
