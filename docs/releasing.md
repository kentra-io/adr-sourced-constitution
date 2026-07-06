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

A fresh container (a new product's claudebox, CI, any Debian/Ubuntu image) has
no `constitution` binary. Provision it at image-build time so nothing falls back
to building from source. The one-liner: pipe [`install.sh`](../install.sh) with a
pinned tag and a system `BINDIR` (root at build time, so `/usr/local/bin` is on
everyone's PATH):

```dockerfile
# Install the constitution CLI at a pinned version (arch-aware, checksum-verified).
ARG CONSTITUTION_VERSION=v0.1.0
RUN curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh \
      | BINDIR=/usr/local/bin sh -s -- "${CONSTITUTION_VERSION}"
```

`install.sh` detects OS/arch, downloads the matching release archive, verifies it
against `checksums.txt`, and drops the binary in `BINDIR`. It never installs a Go
toolchain or builds from source. Defaults: latest release, `BINDIR=~/.local/bin`
(user space — no root needed) when run outside a Dockerfile.

If you prefer no pipe-to-`sh`, the deterministic asset URL
(`constitution_<version>_<os>_<arch>.tar.gz`, produced by
`archives.name_template` in [`.goreleaser.yaml`](../.goreleaser.yaml)) lets you
download, `sha256sum -c --ignore-missing checksums.txt`, and `tar -xzf … -C
/usr/local/bin constitution` by hand.

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
