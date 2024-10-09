package server

import (
	"context"
	"os"

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
}

func NewServer(ctx context.Context) *Server {
	_, span := trace.StartSpan(ctx, "server_init")
	defer span.End()
	engine := gin.New()

	server := &Server{
		Router: engine,
	}

	server.registerHandlers()

	return server
}

func (server *Server) Start(ctx context.Context) {

	hostname, err := os.Hostname()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't resolve hostname")
	}

	go NewPrometheusServer(8888)

	docker.RegisterViews()

	ctx, cancel := context.WithCancel(context.Background())

	agent := &docker.DockerAgent{
		Cancel:   cancel,
		Client:   docker.NewClient(ctx),
		Hostname: hostname,
	}

	defer cancel()

	logrus.Info("Starting Docker Agent...")
	version, _ := agent.Client.Version(ctx)

	logrus.WithFields(logrus.Fields{
		"Platform":              version.Platform.Name,
		"OS":                    version.Os,
		"Arch":                  version.Arch,
		"Experimental Features": version.Experimental,
		"Version":               version.Version,
		"API Version":           version.APIVersion,
	}).Info("Docker Information:")

	logrus.Info("Starting Server...")
	server.Router.Run()

}

func (server *Server) registerHandlers() {
	engine := server.Router

	engine.GET("/", handlePing)
	engine.GET("/ping", handlePing)
}
