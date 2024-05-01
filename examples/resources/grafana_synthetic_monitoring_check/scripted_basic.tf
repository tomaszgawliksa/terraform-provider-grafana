
data "grafana_synthetic_monitoring_probes" "main" {}

resource "grafana_synthetic_monitoring_check" "scripted" {
  job     = "Scripted defaults"
  target  = "scripted target"
  enabled = false
  probes = [
    data.grafana_synthetic_monitoring_probes.main.probes.Paris,
  ]
  labels = {
    foo = "bar"
  }
  settings {
    scripted {
      script = "console.log('Hello, world!')"
    }
  }
}
