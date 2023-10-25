package exporter

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/xanzy/go-gitlab"
)

type GitlabPagesExporter struct {
	gitlabClient *gitlab.Client
	httpClient   *http.Client

	customDomainMetrics *prometheus.GaugeVec
	projectPagesMetrics *prometheus.GaugeVec
}

func NewGitlabPagesExporter(apiUrl string, adminToken string) *GitlabPagesExporter {
	git, err := gitlab.NewClient(adminToken, gitlab.WithBaseURL(apiUrl))
	if err != nil {
		log.Fatalf("ERROR: Failed to create GitLab client: %s", err)
	}

	return &GitlabPagesExporter{
		gitlabClient: git,
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		projectPagesMetrics: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gpe_pages_enabled",
				Help: "GitLab project pages statistics",
			},
			[]string{
				"project_id",
				"project_name",
				"web_url",
				"access_level",
				"check",
			},
		),
		customDomainMetrics: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gpe_custom_domains_verified",
				Help: "GitLab projects with custom domains",
			},
			[]string{
				"project_id",
				"url",
			},
		),
	}
}

func (g *GitlabPagesExporter) fetchProjectPagesMetrics() {

	start := time.Now()
	projOpts := &gitlab.ListProjectsOptions{
		OrderBy: gitlab.String("id"),
		Sort:    gitlab.String("asc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	totalProjects := 0

	for {
		projects, resp, err := g.gitlabClient.Projects.ListProjects(projOpts)
		if err != nil {
			log.Printf("ERROR Failed to get GitLab projects: %s", err)
		}

		if resp.NextPage == 0 {
			break
		}

		for _, project := range projects {
			go g.setProjectPagesMetrics(project)
		}

		log.Printf("INFO: Handled %d of %d pages à %d projects", resp.CurrentPage, resp.TotalPages, resp.ItemsPerPage)
		projOpts.Page = resp.NextPage
		totalProjects = totalProjects + len(projects)
	}
	elapsed := time.Since(start)
	log.Printf("INFO: Got %d projects in %s", totalProjects, elapsed)
}

func (g *GitlabPagesExporter) fetchCustomDomainMetrics() {
	start := time.Now()
	customDomains, _, err := g.gitlabClient.PagesDomains.ListAllPagesDomains()
	if err != nil {
		log.Printf("ERROR: Failed to get GitLab custom domains: %s", err)
	}

	for _, domain := range customDomains {
		go g.setCustomDomainMetrics(domain)
	}

	elapsed := time.Since(start)
	log.Printf("INFO: Got %d custom domains in %s", len(customDomains), elapsed)
}

func (g *GitlabPagesExporter) setCustomDomainMetrics(domain *gitlab.PagesDomain) {
	value := 1.0
	if !domain.Verified {
		value = 0.0
	}
	g.customDomainMetrics.WithLabelValues(
		fmt.Sprintf("%d", domain.ProjectID),
		domain.URL,
	).Set(value)
}

func (g *GitlabPagesExporter) setProjectPagesMetrics(project *gitlab.Project) {
	url := fmt.Sprintf("%s/pages", project.WebURL)
	check := "succeeded"
	value := 0.0
	resp, err := g.httpClient.Get(url)
	if err != nil {
		log.Printf("ERROR: Failed to get /pages for project %s: %s", project.WebURL, err)
		check = "failed"
	}
	if resp.StatusCode == http.StatusOK {
		value = 1
	} else if resp.StatusCode != http.StatusFound {
		check = "failed"
	}
	g.projectPagesMetrics.WithLabelValues(
		fmt.Sprintf("%d", project.ID),
		project.Name,
		project.WebURL,
		string(project.PagesAccessLevel),
		check,
	).Set(value)
}

func (g *GitlabPagesExporter) Run() {
	for {
		log.Println("INFO: Starting new metrics collection")
		go g.fetchCustomDomainMetrics()
		go g.fetchProjectPagesMetrics()
		time.Sleep(24 * time.Hour)
	}
}
