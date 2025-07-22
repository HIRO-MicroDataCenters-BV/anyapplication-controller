package types

import v1 "hiro.io/anyapplication/api/v1"

type Chart struct {
}

type Charts interface {
	RunSynchronization()
	GetChart(application *v1.AnyApplication) (*Chart, error)
}
