package main

import (
	"fmt"
	"io"
	localprom "programmingpercy/cadence-tavern/prometheus"
	_ "programmingpercy/cadence-tavern/workflows/greetings"
	_ "programmingpercy/cadence-tavern/workflows/orders"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	_ "go.uber.org/cadence/.gen/go/cadence"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/worker"

	"go.uber.org/yarpc"
	_ "go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/tchannel"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// cadenceService should always be cadence-frontend
	CadenceService = "cadence-frontend"
	// ClientName is the identifier for the service
	ClientName = "greetings-worker"
	// Domain is the domain you have registered and want to operate in
	Domain = "tavern"
	// Host is the Cadence server IP:Port
	Host = "127.0.0.1:7933"
	// TaskList is the identifier for tasks, activites and workflows
	TaskList = "greetings"
)

func main() {
	// Init Tracer
	tracer, closer := initJaeger("tavern-worker-service")
	defer closer.Close()

	// Create the Worker service
	worker, logger, err := newWorkerServiceClient(tracer)
	if err != nil {
		panic(err)
	}

	// Start worker
	if err := worker.Start(); err != nil {
		panic(fmt.Errorf("failed to start the worker: %v", err))
	}

	logger.Info("Started Worker.", zap.String("worker", TaskList))

	// Block Forever
	select {}

}

// newWorkerServiceClient is used to initialize a new Worker service
// It will handle Connecting and configuration of the client
// Returns a Worker, the logger applied or an error
// TODO expand this function to allow more configurations, will be done later in the article.
func newWorkerServiceClient(tracer opentracing.Tracer) (worker.Worker, *zap.Logger, error) {

	// Create a logger to use for the service
	logger, err := newLogger()
	if err != nil {
		return nil, nil, err
	}

	reporter, err := localprom.NewPrometheusReporter("127.0.0.1:9098", logger)
	if err != nil {
		return nil, nil, err
	}

	metricsScope := localprom.NewServiceScope(reporter)

	// build the most basic Options for now
	workerOptions := worker.Options{
		Logger:       logger,
		MetricsScope: metricsScope,
		Tracer:       tracer,
	}
	// Create the connection that the worker should use
	connection, err := newCadenceConnection(ClientName)
	if err != nil {
		return nil, nil, err
	}
	//  Create the worker and return
	return worker.New(connection, Domain, TaskList, workerOptions), logger, nil
}

// newCadenceConnection is used to create a new YARPC connection to the Cadence server
// @clientName - used to identify the connection on YARPC
func newCadenceConnection(clientName string) (workflowserviceclient.Interface, error) {
	// Create a new Channel to communicate through
	// Set the service name to our Client name so we can Identify the connection
	ch, err := tchannel.NewChannelTransport(tchannel.ServiceName(ClientName))
	if err != nil {
		return nil, fmt.Errorf("failed to set up Transport channel: %v", err)
	}
	// Set up the dispatcher
	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: ClientName,
		Outbounds: yarpc.Outbounds{
			CadenceService: {Unary: ch.NewSingleOutbound(Host)},
		},
	})
	// Start the dispatcher to allow incomming/outgoing messages
	if err := dispatcher.Start(); err != nil {
		return nil, fmt.Errorf("failed to start dispatcher: %v", err)
	}
	// Return a new workflowserviceclient with the connection assigned
	return workflowserviceclient.New(dispatcher.ClientConfig(CadenceService)), nil
}

// newLogger will create a new logger to be used by the Worker Services
// For now use DevelopmentConfig and Info level
func newLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()

	config.Level.SetLevel(zapcore.InfoLevel)

	var err error
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %v", err)
	}

	return logger, nil
}

// initJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func initJaeger(service string) (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		ServiceName: service,
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, closer, err := cfg.NewTracer(config.Logger(jaeger.StdLogger))
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return tracer, closer
}
