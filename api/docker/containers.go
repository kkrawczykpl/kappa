package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func (d *dockerWrap) CreateContainer(ctx context.Context, opts *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (response container.CreateResponse, err error) {
	_, closer := makeTracker(ctx, "docker_create_container")
	defer func() { closer(err) }()

	response, err = d.docker.ContainerCreate(ctx, opts, hostConfig, networkingConfig, platform, containerName)

	return response, err
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

func (d *dockerWrap) AttachContainer(ctx context.Context, id string) (response types.HijackedResponse, err error) {
	_, closer := makeTracker(ctx, "docker_attach_container")
	defer func() { closer(err) }()

	return d.docker.ContainerAttach(ctx, id, container.AttachOptions{
		Stdout: true,
		Stderr: true,
	})
}

func (d *dockerWrap) ContainerExecCreate(ctx context.Context, id string, opts container.ExecOptions) (response types.IDResponse, err error) {
	_, closer := makeTracker(ctx, "docker_container_exec_create")
	defer func() { closer(err) }()
	return d.docker.ContainerExecCreate(ctx, id, opts)
}

func (d *dockerWrap) ContainerExecStart(ctx context.Context, id string, opts container.ExecStartOptions) (err error) {
	_, closer := makeTracker(ctx, "docker_container_exec_start")
	defer func() { closer(err) }()
	return d.docker.ContainerExecStart(ctx, id, opts)
}

func (d *dockerWrap) ContainerExecInspect(ctx context.Context, id string) (response container.ExecInspect, err error) {
	_, closer := makeTracker(ctx, "docker_container_exec_inspect")
	defer func() { closer(err) }()
	return d.docker.ContainerExecInspect(ctx, id)
}

func (d *dockerWrap) ContainerExecAttach(ctx context.Context, id string, opts container.ExecStartOptions) (response types.HijackedResponse, err error) {
	_, closer := makeTracker(ctx, "docker_container_exec_attach")
	defer func() { closer(err) }()
	return d.docker.ContainerExecAttach(ctx, id, opts)
}

func (d *dockerWrap) ContainerRemove(ctx context.Context, id string, opts container.RemoveOptions) (err error) {
	_, closer := makeTracker(ctx, "docker_container_remove")
	defer func() { closer(err) }()
	return d.docker.ContainerRemove(ctx, id, opts)
}

func (d *dockerWrap) ContainerStop(ctx context.Context, id string, opts container.StopOptions) (err error) {
	_, closer := makeTracker(ctx, "docker_container_stop")
	defer func() { closer(err) }()
	return d.docker.ContainerStop(ctx, id, opts)
}

func (d *dockerWrap) ContainerRestart(ctx context.Context, id string, opts container.StopOptions) (err error) {
	_, closer := makeTracker(ctx, "docker_container_restart")
	defer func() { closer(err) }()
	return d.docker.ContainerRestart(ctx, id, opts)
}

func (d *dockerWrap) ContainerKill(ctx context.Context, id string, signal string) (err error) {
	_, closer := makeTracker(ctx, "docker_container_kill")
	defer func() { closer(err) }()
	return d.docker.ContainerKill(ctx, id, signal)
}

func (d *dockerWrap) ContainerPause(ctx context.Context, id string) (err error) {
	_, closer := makeTracker(ctx, "docker_container_pause")
	defer func() { closer(err) }()
	return d.docker.ContainerPause(ctx, id)
}

func (d *dockerWrap) ContainerUnpause(ctx context.Context, id string) (err error) {
	_, closer := makeTracker(ctx, "docker_container_unpause")
	defer func() { closer(err) }()
	return d.docker.ContainerUnpause(ctx, id)
}

func (d *dockerWrap) ContainerRename(ctx context.Context, id string, newName string) (err error) {
	_, closer := makeTracker(ctx, "docker_container_rename")
	defer func() { closer(err) }()
	return d.docker.ContainerRename(ctx, id, newName)
}

func (d *dockerWrap) ContainerUpdate(ctx context.Context, id string, opts container.UpdateConfig) (response container.ContainerUpdateOKBody, err error) {
	_, closer := makeTracker(ctx, "docker_container_update")
	defer func() { closer(err) }()
	return d.docker.ContainerUpdate(ctx, id, opts)
}

func (d *dockerWrap) ContainerStats(ctx context.Context, id string, stream bool) (response container.StatsResponseReader, err error) {
	_, closer := makeTracker(ctx, "docker_container_stats")
	defer func() { closer(err) }()
	return d.docker.ContainerStats(ctx, id, stream)
}

func (d *dockerWrap) ContainerInspect(ctx context.Context, id string) (response types.ContainerJSON, err error) {
	_, closer := makeTracker(ctx, "docker_container_inspect")
	defer func() { closer(err) }()
	return d.docker.ContainerInspect(ctx, id)
}

func (d *dockerWrap) ContainerList(ctx context.Context, opts container.ListOptions) (response []types.Container, err error) {
	_, closer := makeTracker(ctx, "docker_container_list")
	defer func() { closer(err) }()
	return d.docker.ContainerList(ctx, opts)
}
