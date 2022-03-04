package prometheus

import (
	"time"

	prom "github.com/m3db/prometheus_client_golang/prometheus"
	"github.com/uber-go/tally"
	"github.com/uber-go/tally/prometheus"
	"go.uber.org/zap"
)

var (
	safeCharacters = []rune{'_'}

	sanitizeOptions = tally.SanitizeOptions{
		NameCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		KeyCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		ValueCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		ReplacementCharacter: tally.DefaultReplacementCharacter,
	}
)

// NewPrometheusReporter is used to create a new reporter that can send info to prom
// we need a zap logger inputted to make sure we get logs on error
// addr should be the IP:PORT to send metrics
func NewPrometheusReporter(addr string, logger *zap.Logger) (prometheus.Reporter, error) {
	promCfg := prometheus.Configuration{
		ListenAddress: addr,
	}

	reporter, err := promCfg.NewReporter(
		prometheus.ConfigurationOptions{
			Registry: prom.NewRegistry(),
			OnError: func(err error) {
				logger.Warn("error in prometheus reporter", zap.Error(err))
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return reporter, nil
}

// NewServiceScope is used by services and prefixed Service_
func NewServiceScope(reporter prometheus.Reporter) tally.Scope {
	serviceScope, _ := tally.NewRootScope(tally.ScopeOptions{
		Prefix:          "Service_",
		Tags:            map[string]string{},
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &sanitizeOptions,
	}, 1*time.Second)

	return serviceScope
}

// NewWorkerScope is used by Workers and prefixed Worker_
func NewWorkerScope(reporter prometheus.Reporter) tally.Scope {
	serviceScope, _ := tally.NewRootScope(tally.ScopeOptions{
		Prefix:          "Worker_",
		Tags:            map[string]string{},
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &sanitizeOptions,
	}, 1*time.Second)

	return serviceScope
}
