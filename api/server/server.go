package server

import (
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/gin-gonic/gin"
	"github.com/kkrawczykpl/kappa/docker"
	"github.com/sirupsen/logrus"

	"go.opencensus.io/trace"
)

const (
	DefaultPort = 8080
)

type Server struct {
	Router *gin.Engine
	Agent  *docker.DockerAgent
}

type TaskType string
type TaskData interface{}

const (
	Create TaskType = "create"
	Start  TaskType = "start"
	Exec   TaskType = "exec"
)

type Task struct {
	Type            TaskType
	Data            TaskData
	ResponseChannel chan string
}

type CreateData struct {
	Config     *container.Config     `json:"config"`
	HostConfig *container.HostConfig `json:"host_config"`
	Name       string                `json:"name"`
}

type StartData struct {
	Id string `json:"id"`
}

type ExecData struct {
	Id      string   `json:"id"`
	Command []string `json:"command"`
}

func NewServer(ctx context.Context) *Server {

	hostname, err := os.Hostname()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't resolve hostname")
	}

	_, span := trace.StartSpan(ctx, "server_init")
	defer span.End()
	engine := gin.New()

	agent := docker.NewDockerAgent(&docker.DockerAgentProps{
		Hostname: hostname,
	})

	server := &Server{
		Router: engine,
		Agent:  agent,
	}

	tasksChan := make(chan Task)

	server.Router.Use(TasksWrapper(&tasksChan))

	server.registerHandlers()

	go server.eventLoop(ctx, &tasksChan)

	return server
}

func (server *Server) Start(ctx context.Context) {

	go NewPrometheusServer(8888)

	docker.RegisterViews()

	logrus.Info("Starting Server...")
	server.Router.Run()

}

func (server *Server) registerHandlers() {
	engine := server.Router

	engine.GET("/", handlePing)
	engine.GET("/ping", handlePing)
	engine.POST("/start_container", handleStartContainer)
	engine.POST("/create_container", handleCreateContainer)
	engine.POST("/exec_container_command", handleExecCommand)
}

func (server *Server) eventLoop(ctx context.Context, tasksChan *chan Task) {

	for {
		select {
		case task := <-*tasksChan:
			switch task.Type {
			case Create:
				data := task.Data.(*CreateData)

				reader, err := server.Agent.Client.PullImage(ctx, data.Config.Image, image.PullOptions{})

				if err != nil {
					logrus.WithError(err).Error("An error occured while pulling Docker Image!")
					task.ResponseChannel <- "error"
				}

				io.Copy(os.Stdout, reader)

				response, err := server.Agent.Client.CreateContainer(ctx, data.Config, data.HostConfig, nil, nil, data.Name)

				if err != nil {
					logrus.WithError(err).Errorf("An error occured while creating container! Payload: %s", data)
					task.ResponseChannel <- "error"
				} else {
					task.ResponseChannel <- response.ID
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
