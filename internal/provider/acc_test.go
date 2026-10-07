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
