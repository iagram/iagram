# iagram (PyPI launcher)

`pip install iagram` installs a small launcher. On first run it downloads the
iagram release matching this package's version for your platform from
https://github.com/iagram/iagram/releases, verifies it against the release
`SHA256SUMS`, caches it under `~/.iagram/bin/`, and runs it. Every later run
executes the cached binary directly.

- `IAGRAM_BINARY=/path/to/iagram` uses that binary instead.
- `IAGRAM_HOME` relocates the cache (default `~/.iagram`).
- No Python dependencies; nothing else is installed or contacted.

The tool itself is documented at https://github.com/iagram/iagram.
