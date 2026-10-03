# Releasing niimtui

## Current status

The first packaging target is a macOS Homebrew tap:

```bash
brew install davidsanchez222/tap/niimtui
```

Releases are built by [GoReleaser](https://goreleaser.com) in `.github/workflows/release.yml` on a macOS runner. Each release publishes:

- `darwin/arm64` and `darwin/amd64` `.tar.gz` archives (each with `LICENSE` and `README.md`)
- `checksums.txt`
- a GitHub Release with auto-generated notes, which you can edit by hand afterwards
- an updated `Formula/niimtui.rb` in `davidsanchez222/homebrew-tap`

Apple Silicon is the tested platform. Intel macOS archives are published but stay untested until verified on real hardware. Linux and Windows are not packaged yet.

Git tags are the only source of truth for versions (`v0.1.0`, `v0.1.1`, ...). Versions stay at `v0.x` until CLI and config behavior is stable.

## Prerequisites before the first publish

1. Create the public repo `davidsanchez222/homebrew-tap` with a `main` branch. If the default branch has a different name, update `brews[0].repository.branch` in `.goreleaser.yaml`.
2. Create a fine-grained personal access token:
   - Resource owner: `davidsanchez222`
   - Repository access: only `davidsanchez222/homebrew-tap`
   - Permissions: **Contents: Read and write** (Metadata: read is added automatically; nothing else)
   - Expiration: 1 year (see [Tap token expiry and renewal](#tap-token-expiry-and-renewal))
3. In `davidsanchez222/niimtui` → Settings → Secrets and variables → Actions, add it as `TAP_GITHUB_TOKEN`. The built-in `GITHUB_TOKEN` covers the GitHub Release, but it cannot push to the tap repo.
4. Confirm that GitHub Actions is enabled for the repo.
5. Confirm that the `ci` workflow is green on `main`.

Never commit tokens or credentials to either repo.

## Tap token expiry and renewal

`TAP_GITHUB_TOKEN` expires after 1 year. That length is a deliberate tradeoff:

- The token only has Contents read/write on the tap repo. A leak would let someone change the formula, which is visible and easy to revert.
- Releases are infrequent at `v0.x`. A short expiry would mean most releases hit an expired token.
- An expired token causes a partial release. GoReleaser publishes the GitHub Release first and pushes the tap formula second. If the tap push fails, the release exists but `brew install` still gets the old version.

Set a calendar reminder 1–2 weeks before expiry. GitHub also emails you before a token expires.

To renew:

1. Regenerate the token in GitHub → Settings → Developer settings → Fine-grained tokens, with the same repository and permissions.
2. Update the `TAP_GITHUB_TOKEN` secret in `davidsanchez222/niimtui` → Settings → Secrets and variables → Actions.

No repo changes are needed.

If a release has already failed at the tap step because the token expired, renew the token. Then re-run the `release` workflow against the same tag (`workflow_dispatch`, or re-run the failed job). Delete the partially published GitHub Release first so GoReleaser can recreate it cleanly.

## Local dry run

```bash
goreleaser check
goreleaser release --snapshot --clean --skip=publish
./dist/niimtui_darwin_arm64*/niimtui --version
```

Snapshot builds write to `dist/` (gitignored) and publish nothing.

## Release process

1. Merge the release prep changes into `main`.
2. Check that CI is green.
3. Tag and push:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

4. Watch the `release` workflow in the Actions tab. It runs `go test ./...`, `go vet ./...`, `goreleaser check`, then `goreleaser release --clean`.
5. Check the GitHub Release: both darwin archives and `checksums.txt` are attached, and checksums match:

   ```bash
   shasum -a 256 -c checksums.txt --ignore-missing
   ```

6. Check that the tap received a commit titled `Update niimtui to v0.1.0`.
7. Test on Apple Silicon:

   ```bash
   brew update
   brew install davidsanchez222/tap/niimtui
   niimtui --version   # niimtui v0.1.0 (commit abc1234, built YYYY-MM-DD)
   brew test niimtui
   ```

8. If an Intel Mac is available, test there too before marking Intel as verified in the README.
9. After the first release is live, update the README install section to present Homebrew as the active install path, and move the `go run` commands into the development section.

`workflow_dispatch` on the `release` workflow is for re-running a failed release. Run it against the tag ref, because GoReleaser needs a tag.

## Rollback

- Bad GitHub Release: delete it, or mark it as a pre-release and edit the notes to warn users.
- Bad formula: revert the formula commit in `davidsanchez222/homebrew-tap`.
- Prefer publishing a fixed patch tag (`v0.1.1`) over re-pushing or mutating an existing tag or its artifacts.

## Future packaging roadmap

Add channels only when they can be tested and maintained:

- **Windows:** winget first (likely `DavidSanchez.niimtui`), Scoop later. Chocolatey is not planned.
- **Linux:** Arch `niimtui-bin` first, then a source-built `niimtui`. Then apt via `.deb` files on GitHub Releases (GoReleaser + nFPM), then a PPA or project apt repo. Then a Nix flake, and later NUR or Nixpkgs.
- **Homebrew Core:** later, as a source-built formula, once the project is stable.

Linux and Windows stay experimental until BLE printing is verified on real hardware. A successful compile is not enough.
