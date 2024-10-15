package server

import (
	"context"
	"os"

	"github.com/docker/docker/api/types/container"
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

	go server.startEventLoop(ctx, &tasksChan)

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
