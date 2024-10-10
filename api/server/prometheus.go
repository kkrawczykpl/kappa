package server

import (
	"fmt"
	"net/http"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/sirupsen/logrus"
)

func NewPrometheusServer(port int) {
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "kappa",
	})

	if err != nil {
		logrus.WithError(err).Fatalf("Failed to create the Prometheus stats exporter: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", pe)

	address := fmt.Sprintf(":%d", port)

	logrus.Infof("Starting Prometheus Server on %s", address)

	if err := http.ListenAndServe(address, mux); err != nil {
		logrus.WithError(err).Fatalf("Failed to run Prometheus scrape endpoint: %v", err)
	}
}
