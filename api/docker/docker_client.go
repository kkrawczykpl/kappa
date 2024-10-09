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
)

var (
	LatencyView = &view.View{
		Name:        latencyMeasure.Name(),
		Measure:     latencyMeasure,
		Description: latencyMeasure.Description(),

		// Latency in buckets:
		// [>=0ms, >=25ms, >=50ms, >=75ms, >=100ms, >=200ms, >=400ms, >=600ms, >=800ms, >=1s, >=2s, >=4s, >=6s]
		Aggregation: view.Distribution(0, 25, 50, 75, 100, 200, 400, 600, 800, 1000, 2000, 4000, 6000),
		TagKeys:     []tag.Key{}}
)

func NewClient(ctx context.Context) DockerClient {
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
	view.Register(LatencyView)

	if err := view.Register(LatencyView); err != nil {
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

	events, errors := d.docker.Events(ctx, types.EventsOptions{
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
			logrus.Printf("Got an event: %s", <-events)
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

func (d *dockerWrap) CreateContainer(ctx context.Context, opts *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (response container.CreateResponse, err error) {
	_, closer := makeTracker(ctx, "docker_create_container")
	defer func() { closer(err) }()

	response, err = d.docker.ContainerCreate(ctx, opts, hostConfig, networkingConfig, platform, containerName)

	return response, err
}

func (d *dockerWrap) PullImage(ctx context.Context, imageName string, opts image.PullOptions) (reader io.ReadCloser, err error) {
	_, closer := makeTracker(ctx, "docker_pull_image")
	defer func() { closer(err) }()
	reader, err = d.docker.ImagePull(ctx, imageName, opts)
	return reader, err
}

func (d *dockerWrap) StartContainer(ctx context.Context, id string, opts container.StartOptions) (err error) {
	_, closer := makeTracker(ctx, "docker_start_container")
	defer func() { closer(err) }()
	err = d.docker.ContainerStart(ctx, id, opts)
	return err
}

func (d *dockerWrap) WaitForContainerState(ctx context.Context, id string, condition container.WaitCondition) (err error) {
	_, closer := makeTracker(ctx, "docker_wait_for_container_state")
	defer func() { closer(err) }()
	containerStatus, containerErr := d.docker.ContainerWait(ctx, id, container.WaitConditionNotRunning)

	select {
	case <-containerStatus:
	case err := <-containerErr:
		return err
	}

	return err
}

func (d *dockerWrap) GetContainerLogs(ctx context.Context, id string, opts container.LogsOptions) (reader io.ReadCloser, err error) {
	_, closer := makeTracker(ctx, "docker_get_container_logs")
	defer func() { closer(err) }()

	reader, err = d.docker.ContainerLogs(ctx, id, opts)

	return reader, err

}

func (d *dockerWrap) Version(ctx context.Context) (types.Version, error) {

	version, err := d.docker.ServerVersion(ctx)

	if err != nil {
		logrus.WithError(err).Fatal("An error occured while fetching Docker Version")
	}

	return version, nil

}
