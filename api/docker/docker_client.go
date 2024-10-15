package docker

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	common "github.com/kkrawczykpl/kappa/utils"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"golang.org/x/time/rate"
)

type DockerClient interface {
	CreateContainer(ctx context.Context, opts *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	PullImage(ctx context.Context, imageName string, opts image.PullOptions) (io.ReadCloser, error)
	StartContainer(ctx context.Context, id string, opts container.StartOptions) error
	WaitForContainerState(ctx context.Context, id string, condition container.WaitCondition) error
	AddEventListener(ctx context.Context) (listener chan *events.Message, err error)
	RemoveEventListener(ctx context.Context, listener chan *events.Message) (err error)
	GetContainerLogs(ctx context.Context, id string, opts container.LogsOptions) (reader io.ReadCloser, err error)
	Version(ctx context.Context) (types.Version, error)
	AttachContainer(ctx context.Context, id string) (types.HijackedResponse, error)
	ContainerExecCreate(ctx context.Context, id string, opts container.ExecOptions) (response types.IDResponse, err error)
	ContainerExecStart(ctx context.Context, id string, opts container.ExecStartOptions) (err error)
	ContainerExecInspect(ctx context.Context, id string) (response container.ExecInspect, err error)
	ContainerExecAttach(ctx context.Context, id string, opts container.ExecStartOptions) (response types.HijackedResponse, err error)
	ContainerRemove(ctx context.Context, id string, opts container.RemoveOptions) (err error)
	ContainerStop(ctx context.Context, id string, opts container.StopOptions) (err error)
	ContainerRestart(ctx context.Context, id string, opts container.StopOptions) (err error)
	ContainerKill(ctx context.Context, id string, signal string) (err error)
	ContainerPause(ctx context.Context, id string) (err error)
	ContainerUnpause(ctx context.Context, id string) (err error)
	ContainerRename(ctx context.Context, id string, newName string) (err error)
	ContainerUpdate(ctx context.Context, id string, opts container.UpdateConfig) (response container.ContainerUpdateOKBody, err error)
	ContainerStats(ctx context.Context, id string, stream bool) (response container.StatsResponseReader, err error)
	ContainerInspect(ctx context.Context, id string) (response types.ContainerJSON, err error)
	ContainerList(ctx context.Context, opts container.ListOptions) (response []types.Container, err error)
}

type dockerWrap struct {
	docker *client.Client
}

var (
	tagName        = common.MakeKey("tag_name")
	tagStatus      = common.MakeKey("tag_status")
	latencyMeasure = common.MakeMeasure("docker_latency_measure", "Docker latency", "msecs")
	eventsMeasure  = common.MakeMeasure("docker_events", "Docker events", "")
	eventAction    = common.MakeKey("event_action")
	eventType      = common.MakeKey("event_type")
	containerName  = common.MakeKey("container_name")
	containerImage = common.MakeKey("container_image")
)

var (
	LatencyView = &view.View{
		Name:        latencyMeasure.Name(),
		Measure:     latencyMeasure,
		Description: latencyMeasure.Description(),

		// Latency in buckets:
		// [>=0ms, >=25ms, >=50ms, >=75ms, >=100ms, >=200ms, >=400ms, >=600ms, >=800ms, >=1s, >=2s, >=4s, >=6s]
		Aggregation: view.Distribution(0, 25, 50, 75, 100, 200, 400, 600, 800, 1000, 2000, 4000, 6000),
		TagKeys:     []tag.Key{tagName, tagStatus}}

	EventsView = &view.View{
		Name:        eventsMeasure.Name(),
		Measure:     eventsMeasure,
		Description: eventsMeasure.Description(),

		Aggregation: view.Count(),
		TagKeys:     []tag.Key{eventAction, eventType, containerName, containerImage},
	}
)

func NewClient(ctx context.Context) DockerClient {
	logrus.Info("Starting Docker Agent...")
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logrus.WithError(err).Fatal("An error occured while creating new Docker client.")
	}
	defer client.Close()

	_, pingErr := client.Ping(ctx)

	if pingErr != nil {
		logrus.WithError(err).Fatal("An error occured while connecting to Docker deamon.")
	}

	wrap := &dockerWrap{docker: client}
	go wrap.listenEventLoop(ctx)

	return wrap
}

func (dockerWrap *dockerWrap) listenEventLoop(ctx context.Context) {
	logrus.Info("Starting Docker Agent Event Loop")
	limiter := rate.NewLimiter(2.0, 1)
	for limiter.Wait(ctx) == nil {
		err := dockerWrap.listenEvents(ctx)
		if err != nil {
			logrus.WithError(err).Error("An error occured while listening to events! Retrying...")
		}
	}
}

func RegisterViews() {

	if err := view.Register(LatencyView); err != nil {
		logrus.WithError(err).Fatalf("Failed to register views: %v", err)
	}

	if err := view.Register(EventsView); err != nil {
		logrus.WithError(err).Fatalf("Failed to register views: %v", err)
	}
}

func makeTracker(ctx context.Context, name string) (context.Context, func(error)) {

	ctx, err := tag.New(ctx, tag.Insert(tagName, name))

	if err != nil {
		logrus.WithError(err).Fatalf("cannot add tag %v=%v", tagName, name)
	}

	ctx, span := trace.StartSpan(ctx, name)
	start := time.Now()

	return ctx, func(err error) {

		status := "ok"
		if err != nil {
			if err == context.Canceled {
				status = "canceled"
			} else if err == context.DeadlineExceeded {
				status = "timeout"
			} else {
				status = "error"
			}
		}

		ctx, err := tag.New(ctx, tag.Upsert(tagStatus, status))

		if err != nil {
			logrus.WithError(err).Fatalf("cannot add tag %v=%v", tagStatus, status)
		}

		stats.Record(ctx, latencyMeasure.M(int64(time.Since(start)/time.Millisecond)))
		span.End()
	}
}

func (d *dockerWrap) makeListener(ctx context.Context, listener chan *events.Message) {

	events, errors := d.docker.Events(context.Background(), types.EventsOptions{
		Since: strconv.FormatInt(time.Now().Unix(), 10),
		Filters: filters.NewArgs(
			filters.Arg("type", string(events.ContainerEventType)),
		),
	})

	for {
		select {
		case err := <-errors:
			logrus.WithError(err).Fatal("Events listener error!")

		case event := <-events:
			listener <- &event

		case <-ctx.Done():
			return
		}
	}
}

func (d *dockerWrap) listenEvents(ctx context.Context) error {
	listener, err := d.AddEventListener(ctx)

	if err != nil {
		return err
	}

	defer d.RemoveEventListener(ctx, listener)

	for {
		select {
		case ev := <-listener:
			if ev == nil {
				return errors.New("Event listener closed")
			}

			ctx, err := tag.New(context.Background(),
				tag.Upsert(eventAction, string(ev.Action)),
				tag.Upsert(eventType, string(ev.Type)),
				tag.Upsert(containerName, string(ev.Actor.Attributes["name"])),
				tag.Upsert(containerImage, string(ev.Actor.Attributes["image"])),
			)

			if err != nil {
				logrus.WithError(err).Fatalf("An error occured while adding event tags %v=%v %v=%v",
					eventAction, ev.Action,
					eventType, ev.Type,
				)
			}

			stats.Record(ctx, eventsMeasure.M(0))

		case <-ctx.Done():
			return nil
		}
	}
}

func (d *dockerWrap) AddEventListener(ctx context.Context) (listener chan *events.Message, err error) {
	ctx, closer := makeTracker(ctx, "docker_add_event_listener")
	defer func() { closer(err) }()

	logrus.Info("Adding Event Listener")
	listener = make(chan *events.Message)

	go d.makeListener(ctx, listener)

	return listener, nil
}

func (d *dockerWrap) RemoveEventListener(ctx context.Context, listener chan *events.Message) (err error) {
	_, closer := makeTracker(ctx, "docker_remove_event_listener")
	defer func() { closer(err) }()

	logrus.Info("Removing Event Listener")
	cancelFunc := context.WithValue(ctx, "listener_cancel", struct{}{})

	close(listener)

	<-cancelFunc.Done()

	return nil
}

func (d *dockerWrap) PullImage(ctx context.Context, imageName string, opts image.PullOptions) (reader io.ReadCloser, err error) {
	_, closer := makeTracker(ctx, "docker_pull_image")
	defer func() { closer(err) }()
	reader, err = d.docker.ImagePull(ctx, imageName, opts)
	return reader, err
}

func (d *dockerWrap) Version(ctx context.Context) (types.Version, error) {

	version, err := d.docker.ServerVersion(ctx)

	if err != nil {
		logrus.WithError(err).Fatal("An error occured while fetching Docker Version")
	}

	return version, nil

}
