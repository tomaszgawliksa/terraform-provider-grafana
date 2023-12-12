resource "grafana_dashboard" "test" {
  config_json = <<EOD
{
  "title": "Dashboard for report",
  "uid": "report"
}
EOD
  message     = "inital commit."
}

resource "grafana_dashboard" "test2" {
  config_json = <<EOD
{
  "title": "Dashboard for report",
  "uid": "report"
}
EOD
  message     = "inital commit."
}

resource "grafana_report" "test" {
  name          = "multiple dashboards report"
  recipients    = ["some@email.com"]
  schedule {
    frequency         = "monthly"
    last_day_of_month = true
  }
  include_dashboard_link = true
  layout = "grid"
  orientation = "portrait"
  scale_factor = 2
  state       = "scheduled"
  formats = ["pdf"]
  dashboards = [
    {
      uid = grafana_dashboard.test.uid
      time_range = {
        from = "now-1h"
        to   = "now"
      }
    },
    {
      uid = grafana_dashboard.test2.uid
    }
  ]
}


