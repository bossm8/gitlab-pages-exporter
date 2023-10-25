package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.ti.bfh.ch/pages-api/exporter"
)

func main() {

	token := os.Getenv("GPE_GITLAB_ADMIN_READ_TOKEN")
	if token == "" {
		log.Fatal("GPE_GITLAB_ADMIN_READ_TOKEN needs to be set")
	}

	apiUrl := os.Getenv("GPE_GITLAB_API_URL")
	if apiUrl == "" {
		log.Fatal("GPE_GITLAB_API_URL needs to be set")
	}

	exp := exporter.NewGitlabPagesExporter(apiUrl, token)
	go exp.Run()

	log.Println("INFO: Starting metrics server at :2112")
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
