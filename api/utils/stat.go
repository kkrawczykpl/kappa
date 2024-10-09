package common

import (
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

func MakeKey(name string) tag.Key {
	key, err := tag.NewKey(name)

	if err != nil {
		logrus.WithError(err).Fatalf("An error occured while creating tag %s", name)
	}

	return key
}

func MakeMeasure(name string, desc string, unit string) *stats.Int64Measure {
	return stats.Int64(name, desc, unit)
}
