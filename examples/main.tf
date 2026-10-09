terraform {
  required_providers { flare = { source = "aminparsa18/flare" } }
}
provider "flare" {}

resource "flare_notification_channel" "hook" {
  name        = "tf-hook"
  type        = "Webhook"
  webhook_url = "https://example.com/hook"
}

resource "flare_slo" "api" {
  name           = "tf-api-availability"
  kind           = "Availability"
  service_name   = "api"
  target_percent = 99.9
}

resource "flare_alert_rule" "errors" {
  name           = "tf-errors"
  threshold      = 10
  window_seconds = 300
  severity       = "Error"
  channels       = [flare_notification_channel.hook.name]
  labels         = { team = "core" }
  log_condition = {
    services = ["api"]
  }
}

resource "flare_alert_rule" "burn" {
  name           = "tf-burn"
  condition_kind = "SloBurnRate"
  window_seconds = 3600
  channels       = [flare_notification_channel.hook.name]
  slo_condition = {
    slo_id               = flare_slo.api.id
    long_window_seconds  = 3600
    short_window_seconds = 300
    burn_rate_threshold  = 14
  }
}

resource "flare_pipeline_rule" "redact_cards" {
  name = "Redact card numbers"
  condition = {
    services = ["checkout"]
  }
  actions = [{ kind = "RedactRegex", pattern = "\\d{16}", replacement = "[card]" }]
}

resource "flare_maintenance_window" "weekly_patching" {
  name         = "Weekly patching"
  starts_at    = "2030-01-06T02:00:00Z"
  ends_at      = "2030-01-06T04:00:00Z"
  recurrence   = "Weekly"
  days_of_week = ["Sunday"]
}

resource "flare_forwarding_target" "grafana" {
  name     = "Grafana Cloud"
  endpoint = "https://otlp-gateway.example.com/otlp"
  headers  = { Authorization = "Basic ${var.grafana_token}" }
  signals  = ["Logs", "Traces"]
}

# One per Flare instance. Destroying it resets the archive to the worker's configuration.
resource "flare_archive_settings" "archive" {
  endpoint   = "https://s3.eu-west-1.amazonaws.com/flare-archive"
  access_key = var.archive_access_key
  secret_key = var.archive_secret_key
  format     = "Parquet"
}

variable "grafana_token" {
  type      = string
  sensitive = true
}

variable "archive_access_key" {
  type      = string
  sensitive = true
}

variable "archive_secret_key" {
  type      = string
  sensitive = true
}
