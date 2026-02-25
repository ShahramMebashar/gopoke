# macOS Packaging and Signing

## Local Build + Package

```bash
./scripts/release-macos.sh
```

Outputs:
- `artifacts/release/GoPoke.app`
- `artifacts/release/GoPoke-macos-<timestamp>.zip`
- `artifacts/release/gp-043-packaging-report-<timestamp>.md`

## Optional Code Signing

Set signing identity before running the script:

```bash
export MACOS_SIGN_IDENTITY="Developer ID Application: Your Team (TEAMID)"
./scripts/release-macos.sh
```

## Optional Notarization

Use an existing `notarytool` keychain profile:

```bash
export MACOS_NOTARY_PROFILE="gopoke-notary"
./scripts/release-macos.sh
```

The script will:
1. Submit zip to Apple notarization.
2. Wait for result.
3. Staple notarization ticket to `GoPoke.app`.

## CI Pipeline

- Workflow: `.github/workflows/release-macos.yml`
- Trigger: manual (`workflow_dispatch`)
- Uploaded artifacts:
  - release zip
  - packaging/signing report
