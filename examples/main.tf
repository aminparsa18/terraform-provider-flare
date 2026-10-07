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
