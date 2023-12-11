package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/grafana/grafana-openapi-client-go/client/reports"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/grafana/terraform-provider-grafana/internal/common"
)

const (
	reportFrequencyHourly  = "hourly"
	reportFrequencyDaily   = "daily"
	reportFrequencyWeekly  = "weekly"
	reportFrequencyMonthly = "monthly"
	reportFrequencyCustom  = "custom"
	reportFrequencyOnce    = "once"
	reportFrequencyNever   = "never"

	reportOrientationPortrait  = "portrait"
	reportOrientationLandscape = "landscape"

	reportLayoutGrid   = "grid"
	reportLayoutSimple = "simple"

	reportFormatPDF   = "pdf"
	reportFormatCSV   = "csv"
	reportFormatImage = "image"

	reportStateDraft     = "draft"
	reportStateScheduled = "scheduled"
	reportStatePaused    = "paused"
)

var (
	reportLayouts      = []string{reportLayoutSimple, reportLayoutGrid}
	reportOrientations = []string{reportOrientationLandscape, reportOrientationPortrait}
	reportFrequencies  = []string{reportFrequencyNever, reportFrequencyOnce, reportFrequencyHourly, reportFrequencyDaily, reportFrequencyWeekly, reportFrequencyMonthly, reportFrequencyCustom}
	reportFormats      = []string{reportFormatPDF, reportFormatCSV, reportFormatImage}
	states             = []string{reportStateDraft, reportStateScheduled, reportStatePaused}
)

func ResourceReport() *schema.Resource {
	return &schema.Resource{
		Description: `
**Note:** This resource is available only with Grafana Enterprise 7.+.

* [Official documentation](https://grafana.com/docs/grafana/latest/dashboards/create-reports/)
* [HTTP API](https://grafana.com/docs/grafana/latest/developers/http_api/reporting/)
`,
		CreateContext: CreateReport,
		UpdateContext: UpdateReport,
		ReadContext:   ReadReport,
		DeleteContext: DeleteReport,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"org_id": orgIDAttribute(),
			"id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Generated identifier of the report.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the report.",
			},
			"dashboard_id": {
				Type:         schema.TypeInt,
				ExactlyOneOf: []string{"dashboard_id", "dashboard_uid"},
				Computed:     true,
				Optional:     true,
				Deprecated:   "Use dashboard_uid instead",
				Description:  "Dashboard to be sent in the report. This field is deprecated, use `dashboard_uid` instead.",
			},
			"dashboard_uid": {
				Type:         schema.TypeString,
				ExactlyOneOf: []string{"dashboard_id", "dashboard_uid"},
				Computed:     true,
				Optional:     true,
				Deprecated:   "Use dashboards instead",
				Description:  "Dashboard to be sent in the report.",
			},
			"recipients": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "List of recipients of the report.",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringMatch(common.EmailRegexp, "must be an email address"),
				},
				MinItems: 1,
			},
			"reply_to": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Reply-to email address of the report.",
				ValidateFunc: validation.StringMatch(common.EmailRegexp, "must be an email address"),
			},
			"message": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Message to be sent in the report.",
			},
			"include_dashboard_link": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether to include a link to the dashboard in the report.",
			},
			"include_table_csv": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Deprecated:  "Include csv in formats",
				Description: "Whether to include a CSV file of table panel data.",
			},
			"layout": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  common.AllowedValuesDescription("Layout of the report", reportLayouts),
				Default:      reportLayoutGrid,
				ValidateFunc: validation.StringInSlice(reportLayouts, false),
			},
			"orientation": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  common.AllowedValuesDescription("Orientation of the report", reportOrientations),
				Default:      reportOrientationLandscape,
				ValidateFunc: validation.StringInSlice(reportOrientations, false),
			},
			"formats": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: common.AllowedValuesDescription("Specifies what kind of attachment to generate for the report", reportFormats),
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringInSlice(reportFormats, false),
				},
			},
			"state": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  common.AllowedValuesDescription("State of the report", states),
				Default:      reportStateScheduled,
				ValidateFunc: validation.StringInSlice(states, false),
			},
			"scale_factor": {
				Type:         schema.TypeInt,
				Optional:     true,
				Description:  "Zoom to enlarge the text or zoom out to see more data (like table columns)",
				Default:      2,
				ValidateFunc: validation.IntBetween(1, 3),
			},
			"time_range": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Time range of the report.",
				MaxItems:    1,
				Deprecated:  "Set time range in dashboards for each dashboard",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"from": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Start of the time range.",
							RequiredWith: []string{"time_range.0.to"},
						},
						"to": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "End of the time range.",
							RequiredWith: []string{"time_range.0.from"},
						},
					},
				},
			},
			"schedule": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "Schedule of the report.",
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"frequency": {
							Type:         schema.TypeString,
							Required:     true,
							Description:  common.AllowedValuesDescription("Frequency of the report", reportFrequencies),
							ValidateFunc: validation.StringInSlice(reportFrequencies, false),
						},
						"start_time": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Start time of the report. If empty, the start date will be set to the creation time. Note that times will be saved as UTC in Grafana.",
							ValidateFunc: validation.IsRFC3339Time,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								oldParsed, _ := time.Parse(time.RFC3339, old)
								newParsed, _ := time.Parse(time.RFC3339, new)

								// If empty, the start date will be set to the current time (at the time of creation)
								if new == "" && oldParsed.Before(time.Now()) {
									return true
								}

								return oldParsed.Equal(newParsed)
							},
						},
						"end_time": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "End time of the report. If empty, the report will be sent indefinitely (according to frequency). Note that times will be saved as UTC in Grafana.",
							ValidateFunc: validation.IsRFC3339Time,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								oldParsed, _ := time.Parse(time.RFC3339, old)
								newParsed, _ := time.Parse(time.RFC3339, new)
								return oldParsed.Equal(newParsed)
							},
						},
						"workdays_only": {
							Type:        schema.TypeBool,
							Optional:    true,
							Description: "Whether to send the report only on work days.",
							Default:     false,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return !reportWorkdaysOnlyConfigAllowed(d.Get("schedule.0.frequency").(string))
							},
						},
						"custom_interval": {
							Type:     schema.TypeString,
							Optional: true,
							Description: "Custom interval of the report.\n" +
								"**Note:** This field is only available when frequency is set to `custom`.",
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								_, _, err := parseCustomReportInterval(i)
								return diag.FromErr(err)
							},
						},
						"last_day_of_month": {
							Type:        schema.TypeBool,
							Optional:    true,
							Description: "Send the report on the last day of the month",
							Default:     false,
						},
						"timezone": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Schedule timezone",
							Default:     "GMT",
						},
					},
				},
			},
			"dashboards": {
				Type:        schema.TypeList,
				Description: "List of dashboards to be sent in the report",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"dashboard": {
							Type:        schema.TypeList,
							Description: "Dashboard information",
							MinItems:    1,
							MaxItems:    1,
							Required:    true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"uid": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Dashboard UID",
									},
								},
							},
						},
						"time_range": {
							Type:        schema.TypeList,
							Description: "Dashboard time range",
							Optional:    true,
							MinItems:    1,
							MaxItems:    1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"from": {
										Type:         schema.TypeString,
										Optional:     true,
										Description:  "Start of the time range.",
										RequiredWith: []string{"time_range.0.to"},
									},
									"to": {
										Type:         schema.TypeString,
										Optional:     true,
										Description:  "End of the time range.",
										RequiredWith: []string{"time_range.0.from"},
									},
								},
							},
						},
						"report_variables": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Dashboard report variables",
						},
					},
				},
			},
		},
	}
}

func CreateReport(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, orgID := OAPIClientFromNewOrgResource(meta, d)

	req, err := schemaToReportParams(d)
	if err != nil {
		diag.FromErr(err)
	}

	params := reports.NewCreateReportParams().WithBody(req)

	res, err := client.Reports.CreateReport(params)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(MakeOrgResourceID(orgID, res.Payload.ID))
	return ReadReport(ctx, d, meta)
}

func ReadReport(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, _, idStr := OAPIClientFromExistingOrgResource(meta, d.Id())
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return diag.FromErr(err)
	}

	params := reports.NewGetReportParams().WithID(id)
	r, err := client.Reports.GetReport(params)
	if err, shouldReturn := common.CheckReadError("report", d, err); shouldReturn {
		return err
	}

	d.SetId(MakeOrgResourceID(r.Payload.OrgID, id))
	d.Set("dashboard_id", r.Payload.Dashboards[0].Dashboard.ID)
	d.Set("dashboard_uid", r.Payload.Dashboards[0].Dashboard.UID)
	d.Set("name", r.Payload.Name)
	d.Set("recipients", strings.Split(r.Payload.Recipients, ","))
	d.Set("reply_to", r.Payload.ReplyTo)
	d.Set("message", r.Payload.Message)
	d.Set("include_dashboard_link", r.Payload.EnableDashboardURL)
	d.Set("include_table_csv", r.Payload.EnableCSV)
	d.Set("layout", r.Payload.Options.Layout)
	d.Set("orientation", r.Payload.Options.Orientation)
	d.Set("org_id", strconv.FormatInt(r.Payload.OrgID, 10))
	d.Set("state", r.Payload.State)
	d.Set("scale_factor", r.Payload.ScaleFactor)

	if _, ok := d.GetOk("formats"); ok {
		formats := make([]string, len(r.Payload.Formats))
		for i, f := range r.Payload.Formats {
			formats[i] = string(f)
		}
		d.Set("formats", common.StringSliceToSet(formats))
	}

	timeRange := r.Payload.Dashboards[0].TimeRange
	if timeRange.From != "" {
		d.Set("time_range", []interface{}{
			map[string]interface{}{
				"from": timeRange.From,
				"to":   timeRange.To,
			},
		})
	}

	schedule := map[string]interface{}{
		"timezone":      r.Payload.Schedule.TimeZone,
		"frequency":     r.Payload.Schedule.Frequency,
		"workdays_only": r.Payload.Schedule.WorkdaysOnly,
	}
	if r.Payload.Schedule.IntervalAmount != 0 && r.Payload.Schedule.IntervalFrequency != "" {
		schedule["custom_interval"] = fmt.Sprintf("%d %s", r.Payload.Schedule.IntervalAmount, r.Payload.Schedule.IntervalFrequency)
	}
	if r.Payload.Schedule.StartDate.String() != "" {
		t, err := time.Parse(time.RFC3339, r.Payload.Schedule.StartDate.String())
		if err != nil {
			return diag.FromErr(err)
		}
		schedule["start_time"] = t.UTC()
	}
	if r.Payload.Schedule.EndDate.String() != "" {
		t, err := time.Parse(time.RFC3339, r.Payload.Schedule.EndDate.String())
		if err != nil {
			return diag.FromErr(err)
		}
		schedule["end_time"] = t.UTC()
	}
	if r.Payload.Schedule.DayOfMonth == "last" {
		schedule["last_day_of_month"] = true
	}

	d.Set("schedule", []interface{}{schedule})

	return nil
}

func UpdateReport(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, _, idStr := OAPIClientFromExistingOrgResource(meta, d.Id())
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return diag.FromErr(err)
	}

	report, err := schemaToReportParams(d)
	if err != nil {
		return diag.FromErr(err)
	}

	params := reports.NewUpdateReportParams().WithID(id).WithBody(report)
	_, err = client.Reports.UpdateReport(params)
	if err != nil {
		data, _ := json.Marshal(report)
		return diag.Errorf("error updating the following report:\n%s\n%v", string(data), err)
	}

	return ReadReport(ctx, d, meta)
}

func DeleteReport(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, _, idStr := OAPIClientFromExistingOrgResource(meta, d.Id())
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return diag.FromErr(err)
	}

	params := reports.NewDeleteReportParams().WithID(id)
	_, err = client.Reports.DeleteReport(params)
	diag, _ := common.CheckReadError("report", d, err)
	return diag
}

func schemaToReportParams(d *schema.ResourceData) (*models.CreateOrUpdateConfigCmd, error) {
	report := createReportSchema(d)

	dashboards := d.Get("dashboards").([]interface{})
	if len(dashboards) > 0 {
		//report.Dashboards = dashboards
	} else {
		if err := setDeprecatedDashboardValues(report, d); err != nil {
			return nil, err
		}
	}

	formats := common.SetToStringSlice(d.Get("formats").(*schema.Set))
	if len(formats) == 0 {
		report.Formats = []models.Type{reportFormatPDF}
	}

	if err := setReportFrequency(report, d); err != nil {
		return nil, err
	}

	return report, nil
}

func createReportSchema(d *schema.ResourceData) *models.CreateOrUpdateConfigCmd {
	return &models.CreateOrUpdateConfigCmd{
		Name:               d.Get("name").(string),
		EnableDashboardURL: d.Get("include_dashboard_link").(bool),
		EnableCSV:          d.Get("include_table_csv").(bool),
		Message:            d.Get("message").(string),
		Options: &models.ReportOptionsDTO{
			Layout:      d.Get("layout").(string),
			Orientation: d.Get("orientation").(string),
		},
		Recipients:  strings.Join(common.ListToStringSlice(d.Get("recipients").([]interface{})), ","),
		ReplyTo:     d.Get("reply_to").(string),
		ScaleFactor: int64(d.Get("scale_factor").(int)),
		State:       models.State(d.Get("state").(string)),
		Schedule: &models.ScheduleDTO{
			Frequency: d.Get("schedule.0.frequency").(string),
			TimeZone:  d.Get("schedule.0.timezone").(string),
		},
	}
}

func setDeprecatedDashboardValues(report *models.CreateOrUpdateConfigCmd, d *schema.ResourceData) error {
	id := int64(d.Get("dashboard_id").(int))
	uid := d.Get("dashboard_uid").(string)

	timeRange := d.Get("time_range").([]interface{})
	timeRangeDTO := &models.TimeRangeDTO{}
	if len(timeRange) > 0 {
		tr := timeRange[0].(map[string]interface{})
		timeRangeDTO = &models.TimeRangeDTO{
			From: tr["from"].(string),
			To:   tr["to"].(string),
		}
	}

	if id == 0 && uid == "" {
		return fmt.Errorf("dashboard_id or dashboard_uid should be set")
	}

	if uid == "" {
		// We cannot retrieve dashboard by ID, so we set the old values into the deprecated fields.
		report.DashboardID = id
		report.Options.TimeRange = timeRangeDTO
		report.TemplateVars = d.Get("template_vars")
		return nil
	}

	report.Dashboards = []*models.DashboardDTO{
		{
			Dashboard:       &models.DashboardReportDTO{UID: uid},
			ReportVariables: d.Get("template_vars"),
			TimeRange:       timeRangeDTO,
		},
	}

	return nil
}

func setReportFrequency(report *models.CreateOrUpdateConfigCmd, d *schema.ResourceData) error {
	// Set schedule start time
	if report.Schedule.Frequency != reportFrequencyNever {
		if startTimeStr := d.Get("schedule.0.start_time").(string); startTimeStr != "" {
			startDate, err := time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				return err
			}
			startDate = startDate.UTC()
			report.Schedule.StartDate = strfmt.DateTime(startDate)
		}
	}

	// Set schedule end time
	if report.Schedule.Frequency != reportFrequencyOnce && report.Schedule.Frequency != reportFrequencyNever {
		if endTimeStr := d.Get("schedule.0.end_time").(string); endTimeStr != "" {
			endDate, err := time.Parse(time.RFC3339, endTimeStr)
			if err != nil {
				return err
			}
			endDate = endDate.UTC()
			report.Schedule.EndDate = strfmt.DateTime(endDate)
		}
	}

	if report.Schedule.Frequency == reportFrequencyMonthly {
		if lastDayOfMonth := d.Get("schedule.0.last_day_of_month").(bool); lastDayOfMonth {
			report.Schedule.DayOfMonth = "last"
		}
	}

	if reportWorkdaysOnlyConfigAllowed(report.Schedule.Frequency) {
		report.Schedule.WorkdaysOnly = d.Get("schedule.0.workdays_only").(bool)
	}

	if report.Schedule.Frequency == reportFrequencyCustom {
		customInterval := d.Get("schedule.0.custom_interval").(string)
		amount, unit, err := parseCustomReportInterval(customInterval)
		if err != nil {
			return err
		}
		report.Schedule.IntervalAmount = int64(amount)
		report.Schedule.IntervalFrequency = unit
	}

	return nil
}

func reportWorkdaysOnlyConfigAllowed(frequency string) bool {
	return frequency == reportFrequencyHourly || frequency == reportFrequencyDaily || frequency == reportFrequencyCustom
}

func parseCustomReportInterval(i interface{}) (int, string, error) {
	parseErr := errors.New("custom_interval must be in format `<number> <unit>` where unit is one of `hours`, `days`, `weeks`, `months`")

	v := i.(string)
	split := strings.Split(v, " ")
	if len(split) != 2 {
		return 0, "", parseErr
	}

	number, err := strconv.Atoi(split[0])
	if err != nil {
		return 0, "", parseErr
	}

	unit := split[1]
	if unit != "hours" && unit != "days" && unit != "weeks" && unit != "months" {
		return 0, "", parseErr
	}

	return number, unit, nil
}
