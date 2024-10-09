package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func handlePing(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"ping": "pong", "status": http.StatusOK})
}

func StartContainer(ctx *gin.Context) {

}
