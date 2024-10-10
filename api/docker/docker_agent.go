package docker

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

type DockerAgent struct {
	Cancel   func()
	Client   DockerClient
	Hostname string
}

type DockerAgentProps struct {
	Hostname string
}

func NewDockerAgent(config *DockerAgentProps) *DockerAgent {

	ctx, cancel := context.WithCancel(context.Background())

	agent := &DockerAgent{
		Cancel:   cancel,
		Client:   NewClient(ctx),
		Hostname: config.Hostname,
	}

	err := checkDockerVersion(ctx, agent)

	if err != nil {
		logrus.WithError(err).Fatal("Docker version check failed! Make sure that your Docker Deamon is working...")
	}

	return agent
}

func checkDockerVersion(ctx context.Context, agent *DockerAgent) error {
	version, err := agent.Client.Version(ctx)

	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"Platform":              version.Platform.Name,
		"OS":                    version.Os,
		"Arch":                  version.Arch,
		"Experimental Features": version.Experimental,
		"Version":               version.Version,
		"API Version":           version.APIVersion,
	}).Info("Docker Information:")

	return nil
}

func (agent *DockerAgent) StartContainer(ctx context.Context, id string) error {
	return agent.Client.StartContainer(ctx, id, container.StartOptions{})

}
