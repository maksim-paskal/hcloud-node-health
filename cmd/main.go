package main

import (
	"flag"
	"time"

	"github.com/maksim-paskal/hcloud-node-health/pkg/api"
	"github.com/maksim-paskal/hcloud-node-health/pkg/config"
	log "github.com/sirupsen/logrus"
)

func main() {
	flag.Parse()

	log.Infof("Staring hcloud-node-health %s...", config.GetVersion())

	logLevel, err := log.ParseLevel(*config.Get().LogLevel)
	if err != nil {
		log.WithError(err).Fatal()
	}

	log.SetLevel(logLevel)
	log.SetReportCaller(true)

	if !*config.Get().LogPretty {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if err := api.Init(); err != nil {
		log.WithError(err).Fatal()
	}

	scheduleNodeCheck()
}

func scheduleNodeCheck() {
	for {
		if err := api.NodesCheck(); err != nil {
			log.WithError(err).Fatal()
		}

		time.Sleep(*config.Get().CheckPeriod)
	}
}
