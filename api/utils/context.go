package common

import (
	"context"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
)

func d(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	newCTX, stop := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		for {
			select {
			case <-c:
				logrus.WithContext(ctx).Info("Stopping...")
				stop()
				return
			case <-ctx.Done():
				logrus.WithContext(ctx).Info("Original context canceled! Stopping...")
				stop()
				return
			}
		}
	}()
	return newCTX, stop
}
