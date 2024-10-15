package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	common "github.com/kkrawczykpl/kappa/utils"
	"github.com/sirupsen/logrus"
)

func TasksWrapper(tasks *chan Task) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set("tasks", *tasks)
		ctx.Next()
	}
}

func handlePing(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"ping": "pong", "status": http.StatusOK})
}

func handleCreateContainer(ctx *gin.Context) {

	tasksChan, ok := ctx.MustGet("tasks").(chan Task)

	if !ok {
		logrus.Error("An error occured while fetching tasks channel")
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": "false"})
		return
	}

	image := ctx.PostForm("image")
	containerName := ctx.PostForm("name")
	exposedPort := ctx.PostForm("exposed_port")

	// @TODO: Add validation
	// @TODO: Now only exposing same port for host and container is supported. Make support for smth like host_port:container_port
	var containerConfig *container.Config
	var hostConfig *container.HostConfig = nil

	if exposedPort == "" {

		containerConfig = &container.Config{
			Image: image,
		}
	} else {
		containerConfig = &container.Config{
			Image: image,
			ExposedPorts: nat.PortSet{
				nat.Port(fmt.Sprintf("%s/tcp", exposedPort)): struct{}{},
			},
		}

		hostConfig = &container.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port(fmt.Sprintf("%s/tcp", exposedPort)): []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: exposedPort},
				},
			},
		}
	}

	payload := &CreateData{
		Config:     containerConfig,
		HostConfig: hostConfig,
		Name:       containerName,
	}

	responseChannel := make(chan string)

	tasksChan <- Task{Type: Create, Data: payload, ResponseChannel: responseChannel}

	select {
	case response := <-responseChannel:
		if response == "error" {
			ctx.JSON(http.StatusInternalServerError, gin.H{"success": "false"})
			return
		} else {
			ctx.JSON(http.StatusOK, gin.H{"success": "true", "message": response})
			return
		}
	case <-time.After(60 * time.Second):
		ctx.JSON(http.StatusGatewayTimeout, gin.H{"success": "false", "error": "Timeout waiting for container status"})
		return
	}
}

func handleStartContainer(ctx *gin.Context) {

	tasksChan, ok := ctx.MustGet("tasks").(chan Task)

	if !ok {
		logrus.Error("An error occured while fetching tasks channel")
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": "false"})
		return
	}

	id := ctx.PostForm("id")

	responseChannel := make(chan string)

	tasksChan <- Task{Type: Start, Data: &StartData{Id: id}, ResponseChannel: responseChannel}

	select {
	case response := <-responseChannel:
		if response == "error" {
			ctx.JSON(http.StatusInternalServerError, gin.H{"success": "false"})
			return
		} else {
			ctx.JSON(http.StatusOK, gin.H{"success": "true"})
			return
		}
	case <-time.After(60 * time.Second):
		ctx.JSON(http.StatusGatewayTimeout, gin.H{"success": "false", "error": "Timeout waiting for container status"})
		return
	}
}

func handleExecCommand(ctx *gin.Context) {

	tasksChan, ok := ctx.MustGet("tasks").(chan Task)

	if !ok {
		logrus.Error("An error occured while fetching tasks channel")
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": "false"})
		return
	}

	id := ctx.PostForm("id")
	cmd := common.SplitCommand(ctx.PostForm("command"))

	responseChannel := make(chan string)

	tasksChan <- Task{Type: Exec, Data: &ExecData{Id: id, Command: cmd}, ResponseChannel: responseChannel}

	select {
	case response := <-responseChannel:
		if response == "error" {
			ctx.JSON(http.StatusOK, gin.H{"success": "false"})
		} else {
			ctx.JSON(http.StatusOK, gin.H{"success": "true", "message": response})
		}
	case <-time.After(60 * time.Second):
		ctx.JSON(http.StatusOK, gin.H{"success": "false", "error": "Timeout waiting for container status"})
	}

}

func handleLogs(ctx *gin.Context) {

}
