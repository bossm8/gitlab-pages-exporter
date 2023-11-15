package exporter

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/xanzy/go-gitlab"
)

// CheckState defines the current state of the exporter.
type CheckState string

const (
	Failed    CheckState = "failed"
	Succeeded CheckState = "succeeded"
)

const (
	// PrometheusNamespace is the static namespace added to all our metrics.
	PrometheusNamespace string = "gpe"

	// The job name which deploys pages
	// https://docs.gitlab.com/ee/user/project/pages/#how-it-works
	PagesJobName string = "pages"
)

// exporterMetrics holds all metrics the pages exporter provides.
type exporterMetrics struct {
	// Metric describing the custom domains added to pages deployments.
	customDomains *prometheus.GaugeVec
	// Metric describing which pages have pages deployed.
	projectPages *prometheus.GaugeVec

	// Additional metrics to show total numbers.
	// These are required to show timeseries metrics as customDomains and
	// projectPages get reset on each run.
	customDomainsTotal *prometheus.GaugeVec
	projectPagesTotal  *prometheus.GaugeVec

	// Describes the current state of the exporter.
	checkState *prometheus.GaugeVec
	// Describes how long the last check took.
	lastCheckDuration *prometheus.GaugeVec
	// Describes when the last check finished.
	lastCheckTime *prometheus.GaugeVec
	// Describes when the next check is scheduled.
	nextCheckTime *prometheus.GaugeVec

	// Metric holding the number of scrapes since the last restart.
	numberOfScrapes *prometheus.CounterVec

	// Holds the total number of projects which were checked. This metric
	// is added because per default the exporter does not add projects which
	// do not expose pages to the metrics to limit the number of unique metrics
	// exposed to prometheus (cardinality).
	projectsChecked *prometheus.GaugeVec
}

// clearPagesMetrics clears the custom domain and pages metrics on the exporter
// This must be called before each run, as otherwise there will be stale metrics
// when for example a custom domain changes.
func (m *exporterMetrics) clearPagesMetrics() {
	m.customDomains.Reset()
	m.projectPages.Reset()
}

// setCheckStateRunning adjusts the prometheus metrics to show a running state.
func (m *exporterMetrics) setCheckStateRunning() {
	m.checkState.WithLabelValues().Set(1.0)
}

// setCheckStateRunning adjusts the prometheus metrics to show a scheduled state.
func (m *exporterMetrics) setCheckStateFinished() {
	m.checkState.WithLabelValues().Set(0.0)
}

// setNextRun sets the metric showing the next schedule to next.
func (m *exporterMetrics) setNextRun(next int64) {
	m.nextCheckTime.WithLabelValues().Set(float64(next))
}

// setLastCheckMetrics sets the metrics holding information about the last check
// which was run.
func (m *exporterMetrics) setLastCheckMetrics(elapsed *time.Duration) {
	m.lastCheckDuration.WithLabelValues().Set(elapsed.Seconds())
	m.lastCheckTime.WithLabelValues().SetToCurrentTime()
}

// setNumberOfProjects sets the metric holding the number of total projects
// checked to n.
func (m *exporterMetrics) setNumberOfProjects(n *int) {
	m.projectsChecked.WithLabelValues().Set(float64(*n))
}

// setCustomDomainMetrics exposes the domain passed as prometheus metric with
// the value showing the verification status of the domain.
func (m *exporterMetrics) setCustomDomainMetrics(domain *gitlab.PagesDomain) {
	value := 1.0
	if !domain.Verified {
		value = 0.0
	}
	m.customDomains.WithLabelValues(
		fmt.Sprintf("%d", domain.ProjectID),
		domain.URL,
	).Set(value)
}

// increaseNumberOfScrapes increases the scrape runs metric.
func (m *exporterMetrics) increaseNumberOfScrapes() {
	m.numberOfScrapes.WithLabelValues().Inc()
}

// setTotalCustomDomains sets the number of custom domains to total.
func (m *exporterMetrics) setTotalCustomDomains(total *int) {
	m.customDomainsTotal.WithLabelValues().Set(float64(*total))
}

// setTotalProjectPages sets the number of projects with pages enabled to total.
func (m *exporterMetrics) setTotalProjectPages(total *int) {
	m.projectPagesTotal.WithLabelValues().Set(float64(*total))
}

// setProjectPagesMetrics exposes the the project passed as prometheus metric
// the value of the metric will be hasPages (0/1) with the additional label
// check_state set to checkState.
func (m *exporterMetrics) setProjectPagesMetrics(
	project *gitlab.Project,
	hasPages bool,
	checkState CheckState,
) {
	value := 1.0
	if !hasPages {
		value = 0.0
	}
	m.projectPages.WithLabelValues(
		fmt.Sprintf("%d", project.ID),
		project.Name,
		project.WebURL,
		string(project.PagesAccessLevel),
		string(checkState),
	).Set(value)
}

// gitlabPagesExporter holds the actual exporter logic
type gitlabPagesExporter struct {
	gitlabClient *gitlab.Client

	// setMetricsForProjectWithoutPages defines if all projects should be added
	// to prometheus metrics or only the ones actually exposing pages.
	setMetricsForProjectsWithoutPages bool

	metrics *exporterMetrics
}

// NewGitlabPagesExporter creates a new instance of the exporter. Checks can be
// started with .Run().
func NewGitlabPagesExporter(
	apiUrl string,
	adminToken string,
	setMetricsForProjectsWithoutPages bool,
) *gitlabPagesExporter {
	git, err := gitlab.NewClient(adminToken, gitlab.WithBaseURL(apiUrl))
	if err != nil {
		log.Fatalf("ERROR: Failed to create GitLab client: %s", err)
	}

	return &gitlabPagesExporter{
		gitlabClient:                      git,
		setMetricsForProjectsWithoutPages: setMetricsForProjectsWithoutPages,
		metrics: &exporterMetrics{
			projectPages: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "project_pages_enabled",
					Help:      "If GitLab pages are enabled for the project",
				},
				[]string{
					"project_id",
					"project_name",
					"project_web_url",
					"pages_access_level",
					"check_status",
				},
			),
			customDomains: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "custom_domains_verified",
					Help:      "Custom domain verification status",
				},
				[]string{
					"project_id",
					"pages_domain",
				},
			),
			projectPagesTotal: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "project_pages_total",
					Help:      "Shows the total number of projects which have pages deployed",
				},
				[]string{},
			),
			customDomainsTotal: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "custom_domains_total",
					Help:      "Shows the total number of custom domains added",
				},
				[]string{},
			),
			projectsChecked: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "projects_checked_total",
					Help:      "How many projects have been processed",
				},
				[]string{},
			),
			numberOfScrapes: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: PrometheusNamespace,
					Name:      "number_of_scrapes",
					Help:      "How many times the GitLab API was scraped since the last restart",
				},
				[]string{},
			),
			checkState: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "check_running",
					Help:      "Current check state",
				},
				[]string{},
			),
			lastCheckDuration: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "last_check_duration_seconds",
					Help:      "How long the last check was running",
				},
				[]string{},
			),
			lastCheckTime: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "last_check_run_finished_seconds",
					Help:      "When the last check happened",
				},
				[]string{},
			),
			nextCheckTime: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: PrometheusNamespace,
					Name:      "next_check_run_scheduled_seconds",
					Help:      "When the next check is scheduled",
				},
				[]string{},
			),
		},
	}
}

// checkProjectForPagesJob checks the given project if CI/CD is enabled and
// if so if there is a successful job named PagesJobName.
func (g *gitlabPagesExporter) checkProjectForPagesJob(
	project *gitlab.Project,
) (hasPagesJob bool, checkState CheckState) {
	hasPagesJob = false
	checkState = Succeeded

	if string(project.BuildsAccessLevel) != "disabled" && string(project.PagesAccessLevel) != "disabled" {

		jobs, _, err := g.gitlabClient.Jobs.ListProjectJobs(
			project.ID,
			&gitlab.ListJobsOptions{
				ListOptions: gitlab.ListOptions{
					PerPage: 20,
				},
				Scope: &[]gitlab.BuildStateValue{"success"},
			},
		)
		if err != nil {
			log.Printf("ERROR: Failed to get jobs (pages info) for project %s: %s", project.WebURL, err)
			checkState = Failed
		} else {
			for _, job := range jobs {
				if job.Name == PagesJobName {
					hasPagesJob = true
					break
				}
			}
		}
	}

	return
}

// handleProjectPages checks the GitLab API for projects which got pages
// deployments and adds the results to the corresponding prometheus metrics.
// Unfortunately there is no built-in way via the API to gather information
// about pages, thus the information is gathered by checking each project if
// it has CI/CD enabled and if so if there is a job named pages (which is
// mandatory for pages to be deployed).
// https://docs.gitlab.com/ee/user/project/pages/#how-it-works
func (g *gitlabPagesExporter) handleProjectPages() {
	start := time.Now()
	projOpts := &gitlab.ListProjectsOptions{
		OrderBy: gitlab.String("id"),
		Sort:    gitlab.String("asc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    0,
		},
	}

	totalProjects := 0
	totalProjectPages := 0

	for {
		projects, resp, err := g.gitlabClient.Projects.ListProjects(projOpts)
		if err != nil {
			log.Printf("ERROR Failed to get GitLab projects: %s", err)
		}

		var wg sync.WaitGroup
		for _, project := range projects {
			wg.Add(1)
			go func(project *gitlab.Project) {
				defer wg.Done()
				hasPagesJob, checkState := g.checkProjectForPagesJob(project)
				if !hasPagesJob && !g.setMetricsForProjectsWithoutPages {
					return
				} else if hasPagesJob {
					totalProjectPages += 1
				}
				g.metrics.setProjectPagesMetrics(project, hasPagesJob, checkState)
			}(project)
		}
		wg.Wait()

		log.Printf("INFO: Handled %d of %d pages à %d projects",
			resp.CurrentPage,
			resp.TotalPages,
			resp.ItemsPerPage,
		)
		projOpts.Page = resp.NextPage
		totalProjects = totalProjects + len(projects)

		if resp.NextPage == 0 {
			break
		}
	}

	elapsed := time.Since(start)
	log.Printf("INFO: Got %d projects in %s of which %d have deployed pages",
		totalProjects,
		elapsed.Round(time.Second),
		totalProjectPages,
	)

	g.metrics.setNumberOfProjects(&totalProjects)
	g.metrics.setTotalProjectPages(&totalProjectPages)
	g.metrics.setLastCheckMetrics(&elapsed)
}

// handleCustomDomains checks the GitLab API for custom domains and adds the
// results to the corresponding prometheus metrics.
// https://docs.gitlab.com/ee/api/pages_domains.html
func (g *gitlabPagesExporter) handleCustomDomains() {
	start := time.Now()
	customDomains, _, err := g.gitlabClient.PagesDomains.ListAllPagesDomains()
	if err != nil {
		log.Printf("ERROR: Failed to get GitLab custom domains: %s", err)
	}

	for _, domain := range customDomains {
		go g.metrics.setCustomDomainMetrics(domain)
	}

	elapsed := time.Since(start)
	totalCustomDomains := len(customDomains)
	g.metrics.setTotalCustomDomains(&totalCustomDomains)
	log.Printf("INFO: Got %d custom domains in %s",
		totalCustomDomains,
		elapsed.Round(time.Second),
	)
}

// Run runs a new scrape against the GitLab API to gather information about
// each project.
func (g *gitlabPagesExporter) Run(next int64) {
	g.metrics.setNextRun(next)
	g.metrics.setCheckStateRunning()
	g.metrics.clearPagesMetrics()

	log.Printf("INFO: Starting new scrape of GitLab pages on instance %s",
		g.gitlabClient.BaseURL(),
	)
	go g.handleCustomDomains()
	g.handleProjectPages()

	g.metrics.setCheckStateFinished()
	g.metrics.increaseNumberOfScrapes()
}
