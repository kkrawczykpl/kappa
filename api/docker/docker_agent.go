package docker

import (
	"bytes"
	"context"
	"errors"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
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

func (agent *DockerAgent) ExecCommand(ctx context.Context, id string, opts container.ExecOptions) (output string, err error) {

	execId, err := agent.Client.ContainerExecCreate(ctx, id, opts)

	if err != nil {
		logrus.WithError(err).Errorf("An error occured while creating exec in container: %s.", id)
	}

	// Attach to exec command
	attachResponse, err := agent.Client.ContainerExecAttach(ctx, execId.ID, container.ExecStartOptions{})

	if err != nil {
		logrus.WithError(err).Errorf("An error occured while attaching to executing command in container: %s.", id)
	}

	defer attachResponse.Close()

	var outputBuff, errBuff bytes.Buffer

	_, err = stdcopy.StdCopy(&outputBuff, &errBuff, attachResponse.Reader)

	if err != nil {
		logrus.WithError(err).Error("An error occured while transering exec output to stdout.")
	}

	// Start exec command
	err = agent.Client.ContainerExecStart(ctx, execId.ID, container.ExecStartOptions{})

	if err != nil {
		logrus.WithError(err).Errorf("An error occured while starting executing command in container: %s.", id)
	}

	// Inspect exec command
	inspectResponse, err := agent.Client.ContainerExecInspect(ctx, execId.ID)

	if err != nil {
		logrus.WithError(err).Error("An error occured while checking exec status")
	}

	if inspectResponse.ExitCode != 0 {
		return "", errors.New("exec command failed")
	}

	return outputBuff.String(), nil

}
