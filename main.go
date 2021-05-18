package main

import (
	"net/http"
	"pema/pkg/types"
	"time"

	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	const interval = 10
	var err error

	e := types.Exporter{}
	_, err = e.ReadSettings()
	if err != nil {
		panic(err)
	}

	a := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pema",
		Subsystem: "mongodb_atlas",
		Name:      "active_clusters",
		Help:      "Number of clusters active with similar name/label",
	},
		e.Settings.GetTagNames(),
	)
	e.Metrics = append(e.Metrics, a)
	prometheus.MustRegister(a)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		ticker := time.NewTicker(interval * time.Second)

		// register metrics in the background
		for range ticker.C {
			c, err := e.GetClusters()
			if err != nil {
				log.Fatal(err)
			}
			err = e.SetMetrics(c)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	log.Println("serving at 5000")
	log.Fatal(http.ListenAndServe(":5000", nil))

}
