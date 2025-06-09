package config

type Config struct {
	Name  string
	Debug bool
}

const TrackerPort = 21027
const TrackerAPIPort = TrackerPort + 1
