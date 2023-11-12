package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bossm8/gitlab-pages-exporter/exporter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron"

	_ "go.uber.org/automaxprocs"
)

var version string = "dev"

func main() {

	v := flag.Bool("v", false, "Print version info")
	flag.Parse()

	if *v {
		println(version)
		os.Exit(0)
	}

	token := os.Getenv("GPE_GITLAB_ADMIN_READ_TOKEN")
	if token == "" {
		log.Fatal("ERROR: GPE_GITLAB_ADMIN_READ_TOKEN needs to be set")
	}

	apiUrl := os.Getenv("GPE_GITLAB_API_URL")
	if apiUrl == "" {
		log.Fatal("ERROR: GPE_GITLAB_API_URL needs to be set")
	}

	schedule := os.Getenv("GPE_CRON_SCHEDULE")
	if schedule == "" {
		log.Println("INFO: Setting GPE_CRON_SCHEDULE to default (0 0 2 * * *)")
		schedule = "0 0 2 * * *"
	}

	setAllMetrics := false
	setAllMetricsStr := os.Getenv("GPE_SET_ALL_PROJECT_METRICS")
	if setAllMetricsStr != "" {
		var err error
		if setAllMetrics, err = strconv.ParseBool(setAllMetricsStr); err != nil {
			log.Fatalf(
				"ERROR: GPE_SET_ALL_PROJECT_METRICS must be valid boolean value, %s",
				err,
			)
		}
	}

	sched, err := cron.Parse(schedule)
	if err != nil {
		log.Fatalf("ERROR: Could not parse cron schedule: %s", err)
	}

	exp := exporter.NewGitlabPagesExporter(apiUrl, token, setAllMetrics)
	runScrape := func() {
		next := sched.Next(time.Now())
		exp.Run(next.Unix())
		log.Printf("INFO: Next run scheduled in %.0f hours (%s)",
			next.Sub(time.Now()).Hours(),
			next.Local().Format("January 2, 2006 15:04:05"),
		)
	}

	log.Println("INFO: Running initial scrape of GitLab pages information")
	go runScrape()

	c := cron.New()
	if err = c.AddFunc(schedule, runScrape); err != nil {
		log.Fatalf("ERROR: Could not start cron schedule: %s", err)
	}
	go c.Run()

	log.Println("INFO: Starting metrics server at :2112")
	log.Println("INFO: Metrics will be served under /metrics")
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
