"""Locate (or fetch once, checksum-verified) the iagram binary and exec it."""

import hashlib
import io
import os
import platform
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile

from . import __version__

REPO = "iagram/iagram"


def _platform():
    system = platform.system().lower()
    machine = platform.machine().lower()
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(machine)
    if system not in ("darwin", "linux", "windows") or arch is None:
        sys.exit(f"iagram: no prebuilt binary for {system}/{machine}; build from source: https://github.com/{REPO}")
    if system == "windows" and arch == "arm64":
        sys.exit("iagram: no prebuilt binary for windows/arm64")
    return system, arch


def _home():
    return os.environ.get("IAGRAM_HOME") or os.path.join(os.path.expanduser("~"), ".iagram")


def _fetch(url):
    with urllib.request.urlopen(url, timeout=120) as r:  # noqa: S310 (fixed GitHub URL)
        return r.read()


def _download(version, dest):
    system, arch = _platform()
    ext = "zip" if system == "windows" else "tar.gz"
    archive = f"iagram_{version}_{system}_{arch}.{ext}"
    base = f"https://github.com/{REPO}/releases/download/v{version}"
    sys.stderr.write(f"iagram: downloading v{version} ({system}/{arch})...\n")
    sums = _fetch(f"{base}/iagram_{version}_SHA256SUMS").decode()
    want = next((line.split()[0] for line in sums.splitlines() if line.endswith(" " + archive)), None)
    if not want:
        sys.exit(f"iagram: no checksum for {archive} in release v{version}")
    data = _fetch(f"{base}/{archive}")
    got = hashlib.sha256(data).hexdigest()
    if got != want:
        sys.exit(f"iagram: checksum mismatch for {archive}: got {got}, want {want}")
    name = "iagram.exe" if system == "windows" else "iagram"
    if ext == "zip":
        with zipfile.ZipFile(io.BytesIO(data)) as z:
            payload = z.read(name)
    else:
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as t:
            member = t.extractfile(name)
            if member is None:
                sys.exit("iagram: binary missing from archive")
            payload = member.read()
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=os.path.dirname(dest))
    with os.fdopen(fd, "wb") as f:
        f.write(payload)
    os.chmod(tmp, os.stat(tmp).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    os.replace(tmp, dest)


def binary_path():
    override = os.environ.get("IAGRAM_BINARY")
    if override:
        return override
    if __version__ == "0.0.0":
        # Development install: fall back to a binary on PATH.
        found = shutil.which("iagram")
        if found and os.path.realpath(found) != os.path.realpath(sys.argv[0]):
            return found
        sys.exit("iagram: development install without a release version; set IAGRAM_BINARY")
    suffix = ".exe" if platform.system().lower() == "windows" else ""
    dest = os.path.join(_home(), "bin", f"iagram-{__version__}{suffix}")
    if not os.path.exists(dest):
        _download(__version__, dest)
    return dest


def main():
    path = binary_path()
    args = [path] + sys.argv[1:]
    if os.name == "nt":
        sys.exit(subprocess.call(args))
    os.execv(path, args)


if __name__ == "__main__":
    main()
