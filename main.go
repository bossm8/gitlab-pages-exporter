package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron"
	"gitlab.ti.bfh.ch/pages-api/exporter"
)

func main() {

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

	sched, err := cron.Parse(schedule)
	if err != nil {
		log.Fatalf("ERROR: Could not parse cron schedule: %s", err)
	}
	next := sched.Next(time.Now()).UnixMilli()

	exp := exporter.NewGitlabPagesExporter(apiUrl, token)
	log.Println("INFO: Running initial gathering of GitLab pages information")
	go exp.Run(next)

	c := cron.New()
	if err = c.AddFunc(schedule, func() {
		next = sched.Next(time.Now()).UnixMilli()
		exp.Run(next)
	}); err != nil {
		log.Fatalf("ERROR: Could not start cron schedule: %s", err)
	}
	go c.Run()

	log.Println("INFO: Starting metrics server at :2112")
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
