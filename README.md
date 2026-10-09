# terraform-provider-flare

Terraform / OpenTofu provider for [Flare](https://github.com/aminparsa18/Flare.Net), the self-hosted OpenTelemetry observability platform. Design: ADR-0146 in the Flare repo.

**Status: early.** Implemented: `flare_notification_channel` (resource and data source), `flare_alert_rule`, `flare_slo`, `flare_maintenance_window`, `flare_alert_template`, `flare_pipeline_rule`, `flare_ingest_key`, `flare_service_account`, `flare_dashboard`, `flare_metric_attribute_rule`, `flare_forwarding_target`, `flare_archive_settings`, `flare_status_page`, `flare_status_incident`.

```hcl
provider "flare" {
  endpoint = "https://flare.example.com" # or FLARE_ENDPOINT
  token    = var.flare_token             # or FLARE_TOKEN; a service-account access token (flr_pat_...)
}

resource "flare_notification_channel" "oncall" {
  name        = "oncall-slack"
  type        = "Webhook"
  webhook_url = var.slack_webhook_url
}
```

## Conventions

- **Names are the stable key.** Flare keeps channel, alert-rule and SLO names unique (case-insensitive), so other resources reference them by name and the provider resolves ids. Renaming updates in place.
- **Credentials are write-only.** Flare never returns them, so they are `sensitive`, kept from your configuration, and changes made outside Terraform are not detected. Importing a channel (`terraform import flare_notification_channel.x <id-or-name>`) therefore leaves them unset until the next apply writes them.
- **Compatibility.** At configure time the provider reads `GET /api/version` and refuses a server older than the minimum it supports (Flare 0.6.0). Development builds are never refused.

## Develop

```bash
go build ./... && go vet ./... && go test ./...
```

Tests use `httptest` fakes and need no Flare server. The Flare API changes live in the Flare repo; this provider needs a build with unique names and service-account get/delete (ADR-0146 phase 1).

## Acceptance tests

They run against a real Flare and a `terraform`/`tofu` binary, and are skipped unless `TF_ACC=1`:

```bash
TF_ACC=1 FLARE_ENDPOINT=http://localhost:8080 FLARE_TOKEN=flr_pat_... \
  TF_ACC_TERRAFORM_PATH=$(which tofu) TF_ACC_PROVIDER_NAMESPACE=aminparsa18 \
  TF_ACC_PROVIDER_HOST=registry.opentofu.org \
  go test ./internal/provider -run TestAcc -v
```

Use a throwaway stack (`docker compose -p tfe2e up -d clickhouse redis ingest api` from the Flare repo, then bootstrap an admin and mint a service-account token); the tests create and destroy uniquely named objects.

## Release

Push a `v*` tag; `.github/workflows/release.yml` runs GoReleaser, which builds the platform zips, a GPG-signed `SHA256SUMS` and the registry manifest, and creates the GitHub release. One-time setup:

1. Generate a GPG key (RSA or ECC, not expiring) and add the private key and passphrase as repository secrets `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`.
2. Terraform Registry: sign in with GitHub, *Publish > Provider*, select this repo, and add the **public** key under *Settings > GPG Keys*.
3. OpenTofu Registry: open an issue/PR at `opentofu/registry` to add the provider and the same public key.
