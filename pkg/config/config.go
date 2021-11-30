package config

import (
	"flag"
	"os"
	"time"
)

const (
	defaultCheckPeriod = 2 * time.Minute
)

var config = Type{
	LogLevel:       flag.String("log.level", "INFO", ""),
	LogPretty:      flag.Bool("log.pretty", false, ""),
	KubeConfigPath: flag.String("kubeconfig", "", ""),
	HetznerToken:   flag.String("token", os.Getenv("HCLOUD_TOKEN"), ""),
	CheckPeriod:    flag.Duration("period", defaultCheckPeriod, ""),
}

type Type struct {
	LogLevel       *string
	LogPretty      *bool
	KubeConfigPath *string
	HetznerToken   *string
	CheckPeriod    *time.Duration
}

func Get() *Type {
	return &config
}

var gitVersion = "dev"

func GetVersion() string {
	return gitVersion
}
