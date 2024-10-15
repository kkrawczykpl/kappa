package docker

import (
	"context"

	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

func CreateVolume(client *client.Client, name string) (volume.Volume, error) {
	return client.VolumeCreate(
		context.Background(),
		volume.CreateOptions{Name: name},
	)
}

func (d *dockerWrap) CreateVolume(ctx context.Context, options *volume.CreateOptions) (vol volume.Volume, err error) {
	_, closer := makeTracker(ctx, "docker_create_volume")
	defer func() { closer(err) }()

	vol, err = d.docker.VolumeCreate(ctx, *options)

	return vol, err
}
