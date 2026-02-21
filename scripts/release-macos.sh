#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
app_root="$repo_root/cmd/gopad"
artifact_dir="${ARTIFACT_DIR:-$repo_root/artifacts/release}"
mkdir -p "$artifact_dir"

app_name="${APP_NAME:-GoPad}"
platform="${TARGET_PLATFORM:-darwin/arm64}"
build_tags="${BUILD_TAGS:-wails,desktop,production}"
wails_cli="${WAILS_CLI:-go run github.com/wailsapp/wails/v2/cmd/wails@v2.11.0}"

sign_identity="${MACOS_SIGN_IDENTITY:-}"
notary_profile="${MACOS_NOTARY_PROFILE:-}"

timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
report_file="$artifact_dir/gp-043-packaging-report-$timestamp.md"

build_status="FAILED"
sign_status="SKIPPED"
notary_status="SKIPPED"

cd "$app_root"

frontend_dist="$app_root/frontend/dist"
if [[ ! -d "$frontend_dist" ]]; then
  echo "frontend dist missing; building frontend assets"
  (cd "$app_root/frontend" && npm install && npm run build)
fi

$wails_cli build -clean -platform "$platform" -tags "$build_tags"

app_bundle="$app_root/build/bin/$app_name.app"
if [[ ! -d "$app_bundle" ]]; then
  echo "expected app bundle not found at $app_bundle"
  exit 1
fi
build_status="PASSED"

staged_app="$artifact_dir/$app_name.app"
rm -rf "$staged_app"
cp -R "$app_bundle" "$staged_app"

if [[ -n "$sign_identity" ]]; then
  codesign --force --deep --timestamp --options runtime --sign "$sign_identity" "$staged_app"
  codesign --verify --deep --strict --verbose=2 "$staged_app"
  sign_status="PASSED"
fi

zip_path="$artifact_dir/${app_name}-macos-${timestamp}.zip"
rm -f "$zip_path"
ditto -c -k --keepParent "$staged_app" "$zip_path"

if [[ -n "$notary_profile" ]]; then
  xcrun notarytool submit "$zip_path" --keychain-profile "$notary_profile" --wait
  xcrun stapler staple "$staged_app"
  notary_status="PASSED"
fi

cat > "$report_file" <<EOF_REPORT
# GP-043 Packaging and Signing Report

- Generated At (UTC): $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- App Root: \`$app_root\`
- Platform: \`$platform\`
- Build Tags: \`$build_tags\`
- Wails CLI: \`$wails_cli\`

| Step | Status |
|---|---|
| Build app bundle | $build_status |
| Code signing | $sign_status |
| Notarization + staple | $notary_status |

## Artifacts

- App bundle: \`$staged_app\`
- Installer zip: \`$zip_path\`

## Signing Inputs

- \`MACOS_SIGN_IDENTITY\`: ${sign_identity:+configured}${sign_identity:-not configured}
- \`MACOS_NOTARY_PROFILE\`: ${notary_profile:+configured}${notary_profile:-not configured}
EOF_REPORT

echo "release artifact: $zip_path"
echo "report: $report_file"
