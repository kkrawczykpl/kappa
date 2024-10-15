package server

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

func (server *Server) startEventLoop(ctx context.Context, tasksChan *chan Task) {

	for {
		select {
		case task := <-*tasksChan:
			switch task.Type {
			case Create:
				data := task.Data.(*CreateData)

				id, err := server.Agent.PullImageAndCreateContainer(ctx, data.Config, data.HostConfig, nil, nil, data.Name)

				if err != nil {
					logrus.WithError(err).Errorf("An error occured while creating container! Payload: %s", data)
					task.ResponseChannel <- "error"
				} else {
					task.ResponseChannel <- id
				}
			case Start:
				data := task.Data.(*StartData)
				if err := server.Agent.StartContainer(ctx, data.Id); err != nil {
					logrus.Errorf("An error occured while starting container! Container ID: %s", data.Id)
					task.ResponseChannel <- "error"
				} else {
					task.ResponseChannel <- "started"
				}

			case Exec:
				data := task.Data.(*ExecData)
				output, err := server.Agent.ExecCommand(ctx, data.Id, container.ExecOptions{
					Cmd:          data.Command,
					AttachStdout: true,
					AttachStderr: true,
				})

				if err != nil {
					logrus.Errorf("An error occured while executing command in container! Container ID: %s Command: %s", data.Id, data.Command)
					task.ResponseChannel <- "error"
				} else {
					task.ResponseChannel <- output
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
