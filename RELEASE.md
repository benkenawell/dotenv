# Releasing

Releases are cut by pushing a semver tag. The `Release` GitHub Actions workflow
(`.github/workflows/release.yml`) runs the tests, then uses GoReleaser
(`.goreleaser.yaml`) to build and publish.

## Cut a release

```sh
git tag v1.0.0
git push origin v1.0.0
```

Tags must match `vX.Y.Z` or `vX.Y.Z-<suffix>`. A suffix such as `v1.2.0-rc.1`
produces a release marked pre-release, which `latest` installs skip.

## What gets published

- Static binaries for Linux, macOS, and Windows on amd64 and arm64.
- One archive per platform (`tar.gz`, `zip` on Windows) containing the binary,
  `LICENSE`, and `README.md`, named `dotenv_<version>_<os>_<arch>`.
- `checksums.txt` covering every archive.
- A changelog generated from commit messages, excluding `docs:`, `test:`, and
  `ci:` prefixed commits.

The version is injected at build time, so `dotenv --version` reports the tag.

## Try it locally

With [mise](https://mise.jdx.dev) you can run GoReleaser without installing it:

```sh
mise x goreleaser@latest -- goreleaser release --snapshot --clean --skip=publish
ls dist/
```

`goreleaser check` reports "no remote configured" in a checkout without a git
remote; that is expected and not a config error.
