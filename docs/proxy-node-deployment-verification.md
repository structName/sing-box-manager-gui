# Proxy Node Deployment Verification

This note records the repeatable verification used for the Proxy Node Deployment implementation and the remaining manual deployment checks.

## Automated checks

Run from the repository root:

```bash
go test ./...
npm --prefix web run build
bash -n build.sh scripts/package-deployment-assets.sh scripts/verify-deployment-vps.sh scripts/verify-deployment-vps-matrix.sh internal/deploy/templates/singbox-vless-reality.sh scripts/templates/singbox-vless-reality.sh internal/deploy/templates/probe-system.sh scripts/templates/probe-system.sh internal/deploy/templates/security-basic.sh scripts/templates/security-basic.sh scripts/runtime-cache/download-singbox.sh
go test ./internal/deploy -run 'TestVerifyDeploymentVPSMatrix'
git diff --check
```

When a local sing-box binary is available, validate the generated config shapes with the real parser:

```bash
SBM_SING_BOX_CHECK_BIN=/path/to/sing-box go test ./internal/api ./internal/deploy -run 'TestTemporaryDeploymentEntrypointConfigPassesSingBoxCheckWhenAvailable|TestVLESSRealityServerConfigPassesSingBoxCheckWhenAvailable'
```

The API tests also include real SSH handshakes for plain/encrypted private-key auth and custom SOCKS5/HTTP CONNECT proxies:

```bash
go test ./internal/api -run 'TestRealDeploymentSSHTesterUses(PassphrasePrivateKeyAuth|PrivateKeyAuth|Custom(Socks5|HTTPConnect)Proxy)'
```

Generated-node import includes a default read-only protocol boundary plus an explicit advanced-edit opt-in:

```bash
go test ./internal/api -run 'TestImportDeploymentGeneratedNodeRejectsProtocolOverridesWithoutAdvancedOptIn|TestImportDeploymentGeneratedNodeAppliesAdvancedProtocolOverrides'
```

Imported deployment nodes remain visible in existing manual-node and grouped-node workflows, and can still be used inside proxy-chain config generation:

```bash
go test ./internal/api ./internal/builder -run 'TestImportDeploymentGeneratedNodeAppearsInExistingNodeLists|TestDeploymentImportedVLESSRealityNodeCanBeUsedInProxyChain'
```

Run history keeps deployment credentials, managed-route identities, and generated-node secrets redacted; the explicit review endpoint returns the unredacted generated node only for successful non-dry-run deployments:

```bash
go test ./internal/api -run 'TestCreateDryRunDeploymentRedactsPrivateKeyPassphrase|TestCreateDryRunDeploymentRedactsManagedCandidateIdentity|TestDeploymentRunHistoryViewRedactsLegacyManagedCandidateIdentity|TestDeploymentRunHistoryRedactsGeneratedNodeSecrets|TestDeploymentGeneratedNodeReviewReturnsUnredactedPayloadExplicitly|TestDeploymentGeneratedNodeReviewRejectsDryRun'
```

Deployment-imported VLESS Reality nodes stay compatible with the full sing-box config builder and the real sing-box parser:

```bash
SBM_SING_BOX_CHECK_BIN=/path/to/sing-box go test ./internal/builder -run TestBuildJSONWithDeploymentImportedVLESSRealityNodePassesSingBoxCheckWhenAvailable
```

Dry-run deployment records task progress through the shared task mechanism:

```bash
go test ./internal/api -run TestCreateDryRunDeploymentStoresRedactedRunHistory
```

Malformed structured script markers fail clearly while preserving raw deployment stdout:

```bash
go test ./internal/api -run TestCreateDeploymentRunPreservesOutputWhenStructuredMarkerIsMalformed
```

Template metadata includes stable identity, runtime metadata, and parameter metadata for the wizard/API contract:

```bash
go test ./internal/deploy ./internal/api -run 'TestTemplateRegistryIncludesProbeAndVLESSRealityWithChecksums|TestListDeploymentTemplatesExposesSupportedTemplateMetadata'
```

The optional `security-basic` template is scoped to a first-version allowlist:

- read-only system, package-manager, privilege, and firewall inspection
- optional base-package installation only when `SBM_SECURITY_INSTALL_BASE_PACKAGES=true`
- default package allowlist: `curl ca-certificates tar gzip unzip`
- firewall mode is `inspect_only` only
- no SSH port changes, no root-login changes, no password-login changes, no firewall mutation, and no service restart

It exposes only `SBM_SECURITY_INSTALL_BASE_PACKAGES`, `SBM_SECURITY_BASE_PACKAGES`, and `SBM_SECURITY_FIREWALL_MODE`, emits `security_inspect`, `base_packages`, `firewall_inspect`, and `SBM_RESULT`, and is packaged as a standalone readable script:

```bash
go test ./internal/deploy ./internal/api -run 'TestSecurityBasic|TestCreateSecurityBasic|TestDeploymentAssetsPackageContainsStandaloneScripts'
```

The VLESS Reality template supports passwordless sudo-capable non-root users by routing privileged operations through `sudo -n`:

```bash
go test ./internal/deploy -run TestVLESSRealityTemplateSupportsPasswordlessSudo
```

Release packaging keeps standalone deployment scripts available as a release asset:

```bash
go test ./internal/deploy -run 'TestDeploymentAssetsPackageContainsStandaloneScripts|TestWorkflowsRunGoTestsWithSupportedToolchain'
./build.sh deployment-assets
tar -tzf dist/sbm-deployment-assets.tar.gz | sort
```

History listing and managed-route inventory have focused regression coverage, including a stable empty-array response shape for empty inventories:

```bash
go test ./internal/api ./internal/deploy -run 'TestListDeploymentRunsFiltersStatusNewestFirst|TestDiscoverConnectionCandidatesEmptyInventoryReturnsJSONEmptyArray|TestListDeploymentConnectionCandidatesWithoutStoreReturnsEmptyArray|TestDiscoverConnectionCandidatesIncludesManagedRoutesAndTunnels|TestNormalizeDeploymentManagedProxyReusesNodeLocalEndpoint|TestDeploymentConnectionTestCleansTemporaryManagedEntrypointOnSSHFailure'
```

Full deployment runs also prove that managed routes reuse an existing local tunnel when available, otherwise start and clean up a temporary SOCKS5 entrypoint:

```bash
go test ./internal/api -run 'TestCreateDeploymentRunReusesExistingManagedTunnel|TestCreateDeploymentRunUsesTemporaryManagedEntrypoint'
```

The VPS verifier redacts its own managed-route logs and fails if API responses expose the selected managed candidate id or SSH private-key file content:

```bash
go test ./internal/deploy -run 'TestVerifyDeploymentVPSScriptAutoSelectsManagedCandidate|TestVerifyDeploymentVPSScriptUsesRequestedManagedCandidate|TestVerifyDeploymentVPSScriptRejectsManagedCandidateIdentityLeak|TestVerifyDeploymentVPSScriptRejectsPrivateKeyFileLeak'
```

External reachability checks use the selected Deployment Connection for both SOCKS5 and HTTP CONNECT custom proxies:

```bash
go test ./internal/api -run 'TestTCPDeploymentReachabilityCheckerUses(CustomProxy|HTTPConnectProxy)'
```

Runtime cache status reports pinned linux/amd64 and linux/arm64 entries as `valid`, `invalid`, or `missing`:

```bash
go test ./internal/api ./internal/deploy -run 'TestRuntimeCacheListsPinnedArchiveStatus|TestRuntimeCacheRejectsNonPinnedRuntime|TestUploadDeploymentRuntimeArchiveRejectsChecksumMismatch|TestRuntimeCacheAPIRejectsNonPinnedRuntime|TestRuntimeCacheHelperScriptMatchesPinnedMetadata'
```

Deployment run creation rejects unsupported runtime sources before creating run history or background tasks:

```bash
go test ./internal/api -run TestCreateDeploymentRunRejectsInvalidRuntimeSourceBeforeCreatingTask
```

The runtime-cache helper can be smoke-tested against GitHub releases without touching the real data directory:

```bash
tmpdir="$(mktemp -d /tmp/sbm-runtime-cache.XXXXXX)"
scripts/runtime-cache/download-singbox.sh --version 1.13.13 --data-dir "$tmpdir"
find "$tmpdir/runtime-cache/sing-box/1.13.13" -maxdepth 1 -type f -printf '%f %s bytes\n' | sort
rm -rf "$tmpdir"
```

Verification guidance:

```bash
go test ./...
npm --prefix web run build
bash -n scripts/verify-deployment-vps.sh scripts/verify-deployment-vps-matrix.sh scripts/package-deployment-assets.sh scripts/runtime-cache/download-singbox.sh scripts/templates/*.sh internal/deploy/templates/*.sh
git diff --check
go test ./internal/deploy -run 'TestVerifyDeploymentVPSMatrix'
scripts/verify-deployment-vps-matrix.sh --plan --skip-unavailable
SBM_SING_BOX_CHECK_BIN=/path/to/sing-box go test ./internal/api ./internal/deploy ./internal/builder -run 'TestTemporaryDeploymentEntrypointConfigPassesSingBoxCheckWhenAvailable|TestVLESSRealityServerConfigPassesSingBoxCheckWhenAvailable|TestBuildJSONWithDeploymentImportedVLESSRealityNodePassesSingBoxCheckWhenAvailable' -count=1
```

The automated matrix covers plan-mode redaction, environment propagation, runtime-cache preparation, direct and proxied connection modes, import/export, cancellation, and preserved remote-run-directory behavior. When `SBM_SING_BOX_CHECK_BIN` points to a compatible local binary, generated server, managed-entrypoint, and imported-node configurations are checked by the real parser.

Authenticated HTTP and browser smoke tests should use a temporary data directory, a non-default local port, and only documentation addresses such as `203.0.113.10`. Use obviously fake credentials such as `example-manager-password` and `example-ssh-password`. Do not commit real node names, IP addresses, hostnames, credentials, run identifiers, copied profile paths, or temporary directory names.

The temporary managed-entrypoint live test is explicitly opt-in and should use a disposable profile:

```bash
work="$(mktemp -d)"
SBM_DEPLOY_LIVE_TEST=1 \
SBM_DEPLOY_LIVE_DATA_DIR="$work" \
go test ./internal/api -run 'TestManagedDeploymentEntrypoint' -count=1 -v
rm -rf "$work"
```

For real-host verification, keep environment-specific endpoints and detailed results in private test logs. The public repository should contain only repeatable commands, documentation-only addresses, and generalized pass/fail expectations.

## Manual VPS checks

Use a clean Linux `amd64` or `arm64` host.

The full repeatable matrix can be previewed without contacting the API/VPS:

```bash
scripts/verify-deployment-vps-matrix.sh --plan --skip-unavailable
```

When the VPS, manager password, and optional proxy endpoints are configured, run:

```bash
SBM_PASSWORD='manager-password' \
SBM_SSH_HOST='203.0.113.10' \
SBM_SSH_PASSWORD='ssh-password' \
SBM_SOCKS5_PROXY_HOST=127.0.0.1 \
SBM_SOCKS5_PROXY_PORT=1080 \
SBM_HTTP_CONNECT_PROXY_HOST=127.0.0.1 \
SBM_HTTP_CONNECT_PROXY_PORT=18080 \
SBM_HTTP_CONNECT_PROXY_USERNAME=ops \
SBM_HTTP_CONNECT_PROXY_PASSWORD='proxy-password' \
SBM_MATRIX_PREPARE_CACHE=true \
SBM_SING_BOX_CHECK_BIN=/path/to/sing-box \
scripts/verify-deployment-vps-matrix.sh --run
```

Set `SBM_MATRIX_SKIP_UNAVAILABLE=true` only for an intentionally partial run when a SOCKS5 or HTTP CONNECT proxy is not available. Full release evidence should include every matrix case that the target environment can support.

The repeatable API smoke for a real host is:

```bash
SBM_PASSWORD='manager-password' \
SBM_SSH_HOST='203.0.113.10' \
SBM_SSH_PASSWORD='ssh-password' \
scripts/verify-deployment-vps.sh
```

For a managed route or a manually entered deployment proxy, set the same connection fields that the UI exposes:

The UI and verifier load the same managed-route inventory. When at least one route is usable and the operator has not manually changed the connection mode, the create form defaults to `managed_proxy` and selects the first usable route. The verifier mirrors this behavior: `SBM_CONNECTION_MODE=managed_proxy` without `SBM_MANAGED_CANDIDATE_ID` automatically selects the first usable managed route, so the SSH connection test and the later deployment run use the same SOCKS5-backed path. Set `SBM_MANAGED_CANDIDATE_ID` only when a specific route must be exercised.

```bash
SBM_CONNECTION_MODE=managed_proxy \
SBM_PASSWORD='manager-password' \
SBM_SSH_HOST='203.0.113.10' \
SBM_SSH_PASSWORD='ssh-password' \
scripts/verify-deployment-vps.sh
```

```bash
SBM_CONNECTION_MODE=managed_proxy \
SBM_MANAGED_CANDIDATE_ID='node:example-edge' \
SBM_PASSWORD='manager-password' \
SBM_SSH_HOST='203.0.113.10' \
SBM_SSH_PASSWORD='ssh-password' \
scripts/verify-deployment-vps.sh
```

```bash
SBM_CONNECTION_MODE=custom_proxy \
SBM_PROXY_TYPE=socks5 \
SBM_PROXY_HOST=127.0.0.1 \
SBM_PROXY_PORT_LOCAL=1080 \
SBM_PASSWORD='manager-password' \
SBM_SSH_HOST='203.0.113.10' \
SBM_SSH_PASSWORD='ssh-password' \
scripts/verify-deployment-vps.sh
```

```bash
SBM_CONNECTION_MODE=custom_proxy \
SBM_PROXY_TYPE=http_connect \
SBM_PROXY_HOST=127.0.0.1 \
SBM_PROXY_PORT_LOCAL=18080 \
SBM_PROXY_USERNAME=ops \
SBM_PROXY_PASSWORD='proxy-password' \
SBM_PASSWORD='manager-password' \
SBM_SSH_HOST='203.0.113.10' \
SBM_SSH_PASSWORD='ssh-password' \
scripts/verify-deployment-vps.sh
```

1. Start a direct deployment run with password SSH auth, `node_server` defaulting to `ssh.host`, proxy port `443`, and runtime source `remote`.
2. While the run is still active, confirm the history view starts showing the probe result, stdout, and `probe_system` progress marker.
3. Confirm the finished run records exit code, generated node, external reachability marker, and redacted parameters.
4. Confirm the history/API view redacts generated-node secrets such as UUIDs while the import action still creates a working manual node.
5. Confirm the remote host has an active `sing-box` systemd service and listens on the selected proxy port.
6. Import the generated node, then build/export sing-box config and run `sing-box check` against the generated client config. The repeatable verifier can cover this with:

```bash
SBM_IMPORT_NODE=true \
SBM_CHECK_EXPORTED_CONFIG=true \
SBM_SING_BOX_CHECK_BIN=/path/to/sing-box \
scripts/verify-deployment-vps.sh
```

7. Repeat with runtime source `cache` after prefetching:

```bash
./scripts/runtime-cache/download-singbox.sh --version 1.13.13 --data-dir ~/.singbox-manager
SBM_RUNTIME_SOURCE=cache scripts/verify-deployment-vps.sh
```

After the SSH connection test, `scripts/verify-deployment-vps.sh` fails before creating the deployment run unless the target reports Linux and a supported `amd64` or `arm64` architecture. If `SBM_RUNTIME_ARCH` is set, the script treats it as an expected architecture and fails when it does not match the remote probe result. When `SBM_RUNTIME_SOURCE=cache` is set, the script requires `/api/deployments/runtime-cache` to report the selected linux archive as `valid`. The script also checks connection-test, run creation, final run history, generated-node review, and import responses for leaked SSH passwords, inline private keys, private-key file contents, private-key passphrases, proxy passwords, and selected managed candidate ids. To avoid verifier logs becoming a second leak source, failed API response bodies are not printed by default; set `SBM_DEBUG_PRESERVE_VERIFY_TMPDIR=true` to retain the verifier temp directory and inspect the saved response JSON locally. When `SBM_IMPORT_NODE=true` is set, it verifies the generated-node review `imported_node_id` and confirms `/api/manual-nodes` contains the imported node with `node_origin=deployed_self_hosted`, `entry_method=deployment_import`, and the matching `deployment_run_id`. When `SBM_CHECK_EXPORTED_CONFIG=true` is also set, it exports `/api/config/export` after import and runs `sing-box check -c` against that generated client config. Set `SBM_SING_BOX_CHECK_BIN` if `sing-box` is not on `PATH` or at `~/.singbox-manager/bin/sing-box`. When `SBM_CANCEL_AFTER_CREATE=true` is set, it cancels immediately after run creation and verifies the run reaches `cancelled`; do not combine that mode with import checks. When `SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true` is set, it verifies `progress_markers.remote_run_dir` is marked `preserved` and points to `/tmp/sbm-deploy-*`.

8. Repeat the SSH connection test and one deployment run using a managed route:
   - existing local mixed/SOCKS inbound when available
   - otherwise an enabled node or proxy chain that requires a temporary entrypoint
9. Confirm cancellation marks the run as `cancelled` and temporary managed entrypoint resources are cleaned up. The repeatable verifier can cover the API path with:

```bash
SBM_CANCEL_AFTER_CREATE=true \
scripts/verify-deployment-vps.sh
```

10. Repeat one deployment with `SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true`; the verifier confirms `progress_markers.remote_run_dir.path` points to a preserved remote `/tmp/sbm-deploy-*` directory.

```bash
SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true \
scripts/verify-deployment-vps.sh
```

## Known limits

- Normal CI does not require a real VPS, real GitHub download, or real proxy network.
- The first template intentionally supports only pinned official sing-box `1.13.13` Linux `amd64` and `arm64` archives.
- `security-basic` is intentionally limited to inspect-only firewall handling and optional allowlisted base-package installation; lockout-prone SSH policy, firewall mutation, and service restart choices remain out of scope.
