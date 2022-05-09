package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"programmingpercy/cadence-tavern/customer"
	localprom "programmingpercy/cadence-tavern/prometheus"
	"programmingpercy/cadence-tavern/workflows/orders"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/client"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	cadenceClientName = "cadence-client"
	cadenceService    = "cadence-frontend"
)

const (
	// The names of the Workflows we will be using
	OrderWorkflow     = "programmingpercy/cadence-tavern/workflows/orders.WorkflowOrder"
	GreetingsWorkflow = "programmingpercy/cadence-tavern/workflows/greetings.workflowGreetings"
)

type CadenceClient struct {
	//dispatcher used to communicate
	dispatcher *yarpc.Dispatcher
	// wfClient is the workflow Client
	wfClient workflowserviceclient.Interface
	// client is the client used for cadence
	client client.Client
	// orderWorkflowID is used to remember the workflow id
	orderWorkflowID string
	// orderWorkflowRunID is the run id of the order workflow
	orderWorkflowRunID string

	logger *zap.Logger
}

// SetupCadenceClient is used to create the client we can use
func SetupCadenceClient() (*CadenceClient, error) {
	// Create a dispatcher used to communicate with server
	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: cadenceClientName,
		Outbounds: yarpc.Outbounds{
			// This shouldnt be hard coded in real app
			// This is a map, so we store this communication channel on "cadence-frontend"
			cadenceService: {Unary: grpc.NewTransport().NewSingleOutbound("localhost:7833")},
		},
	})
	// Start dispatcher
	if err := dispatcher.Start(); err != nil {
		return nil, err
	}
	// Grab the Configurations from the Dispatcher based on cadenceService name
	yarpConfig := dispatcher.ClientConfig(cadenceService)
	// Build the workflowserviceClient that handles the workflows
	wfClient := workflowserviceclient.New(yarpConfig)
	// clientoptions used to control metrics etc

	config := zap.NewDevelopmentConfig()

	config.Level.SetLevel(zapcore.InfoLevel)

	var err error
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %v", err)
	}

	// Start prom scope
	reporter, err := localprom.NewPrometheusReporter("127.0.0.1:9099", logger)
	if err != nil {
		return nil, err
	}
	// use WorkerScope
	metricsScope := localprom.NewWorkerScope(reporter)

	// Init Jeager
	tracer, _ := initJaeger("tavern-api")

	opts := &client.Options{
		MetricsScope: metricsScope,
		Tracer:       tracer,
	}

	// Build the Cadence Client
	cadenceClient := client.NewClient(wfClient, "tavern", opts)

	return &CadenceClient{
		dispatcher: dispatcher,
		wfClient:   wfClient,
		client:     cadenceClient,
		logger:     logger,
	}, nil

}

// SetOrderWorkflowIds is used to store workflows IDS in Memory
func (cc *CadenceClient) SetOrderWorkflowIds(id, runID string) {
	cc.orderWorkflowID = id
	cc.orderWorkflowRunID = runID
}

// GreetUser is used to Welcome a new User into the tavern
func (cc *CadenceClient) GreetUser(w http.ResponseWriter, r *http.Request) {
	// Grab user info from body
	var visitor customer.Customer

	err := json.NewDecoder(r.Body).Decode(&visitor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Trigger Workflow here

	// Create workflow options, this is the same as the CLI, a task list, a timeout timer
	opts := client.StartWorkflowOptions{
		TaskList:                     "greetings",
		ExecutionStartToCloseTimeout: time.Second * 10,
	}

	cc.logger.Info("Starting workflow")
	// This is how you Execute a Workflow and wait for it to finish
	// This is useful if you have synchronous workflows that you want to leverage as functions
	future, err := cc.client.ExecuteWorkflow(r.Context(), opts, GreetingsWorkflow, visitor)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Fetch result once done and marshal into
	if err := future.Get(r.Context(), &visitor); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cc.logger.Info("Finished executing greetings")

	data, _ := json.Marshal(visitor)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// Order is used to send a signal to the worker
func (cc *CadenceClient) Order(w http.ResponseWriter, r *http.Request) {
	// Grab order info from body
	var orderInfo orders.Order

	err := json.NewDecoder(r.Body).Decode(&orderInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cc.logger.Info("Sending signal about order", zap.String("by", orderInfo.By))
	// Send a signal to the Workflow
	// We need to provide a Workflow ID, the RUN ID of the workflow, and the Signal type
	err = cc.client.SignalWorkflow(r.Context(), cc.orderWorkflowID, cc.orderWorkflowRunID, "order", orderInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

}

// initJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func initJaeger(service string) (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		ServiceName: service,
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
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
