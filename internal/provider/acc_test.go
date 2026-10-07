package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Acceptance tests run against a real Flare and a terraform/tofu binary. They are skipped unless TF_ACC=1:
//
//	TF_ACC=1 FLARE_ENDPOINT=http://localhost:8080 FLARE_TOKEN=flr_pat_... \
//	  TF_ACC_TERRAFORM_PATH=$(which tofu) TF_ACC_PROVIDER_NAMESPACE=aminparsa18 \
//	  TF_ACC_PROVIDER_HOST=registry.opentofu.org go test ./internal/provider -run TestAcc
//
// Each test uses uniquely named objects and cleans up through the framework's destroy step.
var accFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"flare": providerserver.NewProtocol6WithError(New("test")()),
}

func accPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("FLARE_ENDPOINT") == "" || os.Getenv("FLARE_TOKEN") == "" {
		t.Fatal("FLARE_ENDPOINT and FLARE_TOKEN must be set for acceptance tests")
	}
}

const accChannel = `
resource "flare_notification_channel" "c" {
  name        = "acc-channel"
  type        = "Webhook"
  webhook_url = "https://example.com/hook"
}
`

func TestAccNotificationChannelAndDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_0_0)},
		Steps: []resource.TestStep{
			{
				Config: accChannel + `data "flare_notification_channel" "d" { name = flare_notification_channel.c.name }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flare_notification_channel.c", "send_resolved", "true"),
					resource.TestCheckResourceAttrPair("data.flare_notification_channel.d", "id", "flare_notification_channel.c", "id"),
				),
			},
			{ // rename is an in-place update, not a replacement
				Config: `
resource "flare_notification_channel" "c" {
  name        = "acc-channel-renamed"
  type        = "Webhook"
  webhook_url = "https://example.com/hook"
}`,
				Check: resource.TestCheckResourceAttr("flare_notification_channel.c", "name", "acc-channel-renamed"),
			},
			{
				ResourceName: "flare_notification_channel.c", ImportState: true, ImportStateId: "acc-channel-renamed",
				ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id",
				ImportStateVerifyIgnore: []string{"webhook_url"}, // write-only: Flare never returns it
			},
		},
	})
}

func TestAccSLO(t *testing.T) {
	cfg := func(target string) string {
		return `
resource "flare_slo" "s" {
  name           = "acc-slo"
  kind           = "Availability"
  service_name   = "acc-api"
  target_percent = ` + target + `
}`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{Config: cfg("99.9"), Check: resource.TestCheckResourceAttr("flare_slo.s", "window_days", "30")},
			{Config: cfg("99.5"), Check: resource.TestCheckResourceAttr("flare_slo.s", "target_percent", "99.5")},
			{ResourceName: "flare_slo.s", ImportState: true, ImportStateId: "acc-slo", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
		},
	})
}

// Every condition kind in one rule each, so a schema/API mismatch in any block fails here.
func TestAccAlertRuleKinds(t *testing.T) {
	const cfg = accChannel + `
resource "flare_slo" "s" {
  name           = "acc-rule-slo"
  kind           = "Availability"
  service_name   = "acc-api"
  target_percent = 99.9
}

resource "flare_alert_rule" "logs" {
  name           = "acc-logs"
  threshold      = 10
  window_seconds = 300
  severity       = "Error"
  labels         = { team = "core" }
  channels       = [flare_notification_channel.c.name]
  log_condition = {
    services = ["acc-api"]
    attributes = [{ key = "k", value = "v" }]
  }
}

resource "flare_alert_rule" "metric" {
  name           = "acc-metric"
  condition_kind = "MetricThreshold"
  comparator     = "LessThan"
  threshold      = 0.5
  window_seconds = 600
  channels       = [flare_notification_channel.c.name]
  metric_condition = {
    metric_name = "acc.cpu"
    type        = "Gauge"
    aggregation = "Max"
    attributes  = { host = "a" }
  }
}

resource "flare_alert_rule" "exceptions" {
  name           = "acc-exceptions"
  condition_kind = "ExceptionCount"
  threshold      = 1
  window_seconds = 300
  channels       = [flare_notification_channel.c.name]
  exception_condition = { exception_type = "System.Exception" }
}

resource "flare_alert_rule" "anomaly" {
  name           = "acc-anomaly"
  condition_kind = "Anomaly"
  window_seconds = 300
  channels       = [flare_notification_channel.c.name]
  anomaly_condition = {
    baseline_periods  = 4
    z_score_threshold = 3
  }
}

resource "flare_alert_rule" "burn" {
  name           = "acc-burn"
  condition_kind = "SloBurnRate"
  window_seconds = 3600
  channels       = [flare_notification_channel.c.name]
  slo_condition = {
    slo_id               = flare_slo.s.id
    long_window_seconds  = 3600
    short_window_seconds = 300
    burn_rate_threshold  = 14
  }
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("flare_alert_rule.logs", "channels.#", "1"),
				resource.TestCheckResourceAttr("flare_alert_rule.metric", "comparator", "LessThan"),
				resource.TestCheckResourceAttrPair("flare_alert_rule.burn", "slo_condition.slo_id", "flare_slo.s", "id"),
			)},
			{ // re-applying the same config must plan clean (no drift from server defaults)
				Config: cfg, PlanOnly: true,
			},
			{ResourceName: "flare_alert_rule.metric", ImportState: true, ImportStateId: "acc-metric", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
		},
	})
}

func TestAccPipelineRule(t *testing.T) {
	cfg := func(name, pattern string) string {
		return `
resource "flare_pipeline_rule" "p" {
  name = "` + name + `"
  condition = {
    services = ["acc-api"]
  }
  actions = [
    { kind = "RedactRegex", pattern = "` + pattern + `", replacement = "[card]" },
    { kind = "ExtractRegex", pattern = "user=(?<user>\\w+)", source_attribute_key = "raw" },
    { kind = "ParseJson", key_prefix = "json.", max_depth = 3 },
  ]
}

resource "flare_pipeline_rule" "all" {
  name    = "acc-pipeline-all"
  enabled = false
  actions = [{ kind = "RedactRegex", pattern = "secret" }]
}`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{Config: cfg("acc-pipeline", `\\d{16}`), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("flare_pipeline_rule.p", "enabled", "true"),
				resource.TestCheckResourceAttr("flare_pipeline_rule.p", "actions.#", "3"),
				resource.TestCheckResourceAttr("flare_pipeline_rule.all", "enabled", "false"),
			)},
			{Config: cfg("acc-pipeline", `\\d{16}`), PlanOnly: true}, // omitted condition/replacement/limits replan clean
			{Config: cfg("acc-pipeline-renamed", `\\d{12}`), Check: resource.TestCheckResourceAttr("flare_pipeline_rule.p", "name", "acc-pipeline-renamed")},
			{ResourceName: "flare_pipeline_rule.p", ImportState: true, ImportStateId: "acc-pipeline-renamed", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
			{ResourceName: "flare_pipeline_rule.all", ImportState: true, ImportStateId: "acc-pipeline-all", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
		},
	})
}

func TestAccMaintenanceWindow(t *testing.T) {
	cfg := func(name, end string) string {
		return accChannel + `
resource "flare_alert_rule" "r" {
  name           = "acc-mw-rule"
  threshold      = 10
  window_seconds = 300
  channels       = [flare_notification_channel.c.name]
}

resource "flare_maintenance_window" "oneoff" {
  name       = "` + name + `"
  starts_at  = "2030-01-01T02:00:00Z"
  ends_at    = "` + end + `"
  rule_names = [flare_alert_rule.r.name]
}

resource "flare_maintenance_window" "weekly" {
  name           = "acc-mw-weekly"
  starts_at      = "2030-01-01T02:00:00Z"
  ends_at        = "2030-01-01T04:00:00Z"
  recurrence     = "Weekly"
  days_of_week   = ["Sunday", "Saturday"]
  time_zone      = "Europe/Berlin"
  repeat_until   = "2031-01-01T00:00:00Z"
  label_matchers = { team = "core" }
}`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{Config: cfg("acc-mw", "2030-01-01T04:00:00Z"), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("flare_maintenance_window.oneoff", "recurrence", "None"),
				resource.TestCheckResourceAttr("flare_maintenance_window.oneoff", "time_zone", "UTC"),
				resource.TestCheckResourceAttr("flare_maintenance_window.oneoff", "rule_names.#", "1"),
				resource.TestCheckResourceAttr("flare_maintenance_window.weekly", "days_of_week.#", "2"),
			)},
			{Config: cfg("acc-mw", "2030-01-01T04:00:00Z"), PlanOnly: true},
			{Config: cfg("acc-mw-renamed", "2030-01-01T05:00:00Z"), Check: resource.TestCheckResourceAttr("flare_maintenance_window.oneoff", "name", "acc-mw-renamed")},
			{ResourceName: "flare_maintenance_window.oneoff", ImportState: true, ImportStateId: "acc-mw-renamed", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
			{ResourceName: "flare_maintenance_window.weekly", ImportState: true, ImportStateId: "acc-mw-weekly", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
		},
	})
}

func TestAccIngestKeyAndServiceAccount(t *testing.T) {
	cfg := func(name string, perMinute string) string {
		return `
resource "flare_service_account" "sa" {
  name = "acc-sa"
  role = "Member"
}

resource "flare_ingest_key" "k" {
  name                  = "` + name + `"
  limits_enabled        = true
  max_events_per_minute = ` + perMinute + `
}

resource "flare_ingest_key" "plain" {
  name = "acc-key-plain"
}`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{Config: cfg("acc-key", "1000"), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("flare_ingest_key.k", "raw_key"),
				resource.TestCheckResourceAttr("flare_ingest_key.k", "max_events_per_minute", "1000"),
				resource.TestCheckResourceAttr("flare_ingest_key.plain", "limits_enabled", "false"),
				resource.TestCheckResourceAttr("flare_service_account.sa", "role", "Member"),
			)},
			{Config: cfg("acc-key", "1000"), PlanOnly: true},
			{Config: cfg("acc-key-renamed", "2000"), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("flare_ingest_key.k", "name", "acc-key-renamed"),
				resource.TestCheckResourceAttr("flare_ingest_key.k", "max_events_per_minute", "2000"),
				resource.TestCheckResourceAttrSet("flare_ingest_key.k", "raw_key"), // kept across an in-place update
			)},
			{ResourceName: "flare_ingest_key.k", ImportState: true, ImportStateId: "acc-key-renamed", ImportStateVerify: true,
				ImportStateVerifyIdentifierAttribute: "id", ImportStateVerifyIgnore: []string{"raw_key"}},
			{ResourceName: "flare_service_account.sa", ImportState: true, ImportStateId: "acc-sa", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "id"},
		},
	})
}

func TestAccDashboard(t *testing.T) {
	cfg := func(name, tags string) string {
		return `
resource "flare_dashboard" "d" {
  name        = "` + name + `"
  description = "managed by terraform"
  tags        = ` + tags + `
  layout_json = jsonencode({
    panels = [{
      id        = "p1"
      panelType = "text"
      title     = "Notes"
      layout    = { x = 0, y = 0, w = 6, h = 4 }
      query     = {}
    }]
    variables = []
  })
}

resource "flare_dashboard" "bare" {
  name        = "acc-dash-bare"
  layout_json = <<-EOT
    { "panels": [] }
  EOT
}`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{Config: cfg("acc-dash", `["prod", "core"]`), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("flare_dashboard.d", "tags.#", "2"),
				resource.TestCheckResourceAttrSet("flare_dashboard.d", "id"),
				resource.TestCheckNoResourceAttr("flare_dashboard.bare", "tags.#"),
			)},
			{Config: cfg("acc-dash", `["prod", "core"]`), PlanOnly: true},
			{Config: cfg("acc-dash-renamed", `["prod"]`), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("flare_dashboard.d", "name", "acc-dash-renamed"),
				resource.TestCheckResourceAttr("flare_dashboard.d", "tags.#", "1"),
			)},
			{ResourceName: "flare_dashboard.d", ImportState: true, ImportStateId: "acc-dash-renamed", ImportStateVerify: true,
				ImportStateVerifyIdentifierAttribute: "id", ImportStateVerifyIgnore: []string{"layout_json"}}, // text differs from the API's compact form
		},
	})
}
