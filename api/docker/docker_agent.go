package docker

type DockerAgent struct {
	Cancel   func()
	Client   DockerClient
	Hostname string
}
