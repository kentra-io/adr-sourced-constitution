# Installing & releasing `constitution`

## Install

### Homebrew (macOS / Linux)

```sh
brew install kentra-io/tap/constitution
```

The cask is published to [`kentra-io/homebrew-tap`](https://github.com/kentra-io/homebrew-tap)
by the release pipeline. The binary is not code-signed; the cask strips the
macOS quarantine attribute on install so it runs without a Gatekeeper prompt.

### `go install`

```sh
go install github.com/kentra-io/adr-sourced-constitution/cmd/constitution@latest
```

`go install` builds locally, so `constitution --version` reports the module
pseudo-version + VCS revision rather than a release tag (both are wired in
`cmd/constitution/version.go`).

### Release archive (direct download)

Every release publishes per-platform archives plus a `checksums.txt`. Asset
names follow a fixed template:

```
constitution_<version>_<os>_<arch>.tar.gz     # linux, darwin
constitution_<version>_<os>_<arch>.zip        # windows
```

where `<version>` is the tag without its leading `v` (tag `v0.1.0` -> `0.1.0`).
So the linux/amd64 tarball for `v0.1.0` is at the deterministic URL:

```
https://github.com/kentra-io/adr-sourced-constitution/releases/download/v0.1.0/constitution_0.1.0_linux_amd64.tar.gz
```

### claudebox / Docker

Because the asset URL is deterministic, a container image can install a pinned
version with a plain `COPY`-free download. Add to your `.claudebox/Dockerfile`
(or any Debian/Ubuntu-based image):

```dockerfile
# Install the constitution CLI at a pinned version.
ARG CONSTITUTION_VERSION=0.1.0
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) goarch=amd64 ;; \
      arm64) goarch=arm64 ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    base="https://github.com/kentra-io/adr-sourced-constitution/releases/download/v${CONSTITUTION_VERSION}"; \
    asset="constitution_${CONSTITUTION_VERSION}_linux_${goarch}.tar.gz"; \
    cd /tmp; \
    curl -fsSL "$base/$asset" -o "$asset"; \
    curl -fsSL "$base/checksums.txt" -o checksums.txt; \
    # Verify the download against the release checksums before extracting.
    # --ignore-missing checks only the asset we fetched, not every line.
    sha256sum -c --ignore-missing checksums.txt; \
    tar -xzf "$asset" -C /usr/local/bin constitution; \
    rm "$asset" checksums.txt; \
    constitution --version
```

The `<version>_<os>_<arch>` template is produced by the `archives.name_template`
in [`.goreleaser.yaml`](../.goreleaser.yaml); keep the two in sync if either
changes.

## How a release is cut

Releases are fully automated by [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which triggers on any `v*` tag.

**Prerequisites (one-time, owner-side):**

- `kentra-io/homebrew-tap` exists (public).
- The `HOMEBREW_TAP_TOKEN` Actions secret is set on this repo: a fine-grained PAT
  with `Contents: read/write` on `homebrew-tap`. GoReleaser needs it to push the
  cask cross-repo — the default `GITHUB_TOKEN` is scoped to this repo only.

**Cutting a release:**

```sh
git tag v0.1.0
git push origin v0.1.0
```

CI then runs `goreleaser release --clean`, which:

1. Builds all targets (linux/darwin/windows × amd64/arm64).
2. Creates the GitHub Release with the archives and `checksums.txt`.
3. Pushes the updated Homebrew cask to `kentra-io/homebrew-tap`.

Validate the config locally before tagging:

```sh
goreleaser check                                    # config is valid
goreleaser release --snapshot --clean --skip=publish  # dry run, builds everything into ./dist
```

`./dist` is git-ignored; snapshot artifacts are never committed.

## If a release fails midway

A tag push that fails partway (e.g. the cask push errors after the GitHub
Release was created) leaves a partial release and a published tag. GoReleaser
does not overwrite an existing release, so re-running against the same tag will
not recover it — tear the partial state down, fix the cause, and re-tag. Delete
the partial release, delete the remote tag, then re-create both once fixed:

```sh
gh release delete v0.1.0 --yes             # remove the partial GitHub Release
git push --delete origin v0.1.0            # remove the remote tag
git tag -d v0.1.0                          # remove the local tag
# ...fix the cause (config, secret, etc.), then re-cut:
git tag v0.1.0
git push origin v0.1.0
```
