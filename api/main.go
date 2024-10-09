package main

import (
	"context"

	"github.com/kkrawczykpl/kappa/server"
)

func main() {
	ctx := context.Background()
	server := server.NewServer(ctx)
	server.Start(ctx)
}
