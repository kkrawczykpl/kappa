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
)

type Task struct {
	Type            TaskType
	Data            TaskData
	ResponseChannel chan string
}

type CreateData struct {
	Image string `json:"image"`
	Name  string `json:"name"`
}

type StartData struct {
	Id string `json:"id"`
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
}

func (server *Server) eventLoop(ctx context.Context, tasksChan *chan Task) {
	for {
		select {
		case task := <-*tasksChan:
			switch task.Type {
			case Create:
				data := task.Data.(*CreateData)

				reader, err := server.Agent.Client.PullImage(ctx, data.Image, image.PullOptions{})

				if err != nil {
					logrus.WithError(err).Error("An error occured while pulling Docker Image!")
					task.ResponseChannel <- "error"
					return
				}

				io.Copy(os.Stdout, reader)

				response, err := server.Agent.Client.CreateContainer(ctx, &container.Config{
					Image: data.Image,
				}, nil, nil, nil, data.Name)

				if err != nil {
					logrus.WithError(err).Errorf("An error occured while creating container! Payload: %s", data)
					task.ResponseChannel <- "error"
					return
				}

				task.ResponseChannel <- response.ID
			case Start:
				data := task.Data.(*StartData)
				if err := server.Agent.StartContainer(ctx, data.Id); err != nil {
					logrus.Errorf("An error occured while starting container! Container ID: %s", data.Id)
					task.ResponseChannel <- "error"
					return
				}

				task.ResponseChannel <- "started"
			}
		case <-ctx.Done():
			return
		}
	}
}
