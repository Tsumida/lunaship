package infra

import (
	"os"
)

const (
	ENV_LIVE = "live"
	ENV_TEST = "test"
	ENV_DEV  = "dev"
)

type ServerConfig struct {
	ServiceId         string
	ServiceVersion    string
	ServiceOpenapiDoc string
	DeployEnv         string
}

func (sc *ServerConfig) LoadFromEnv() error {
	sc.ServiceId = os.Getenv("SERVICE_ID")
	sc.ServiceVersion = os.Getenv("SERVICE_VERSION")
	sc.ServiceOpenapiDoc = os.Getenv("SERVICE_DOC")

	return nil
}
