package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

	payload := &CreateData{
		Image: image,
		Name:  containerName,
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
