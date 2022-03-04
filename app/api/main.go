package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"go.uber.org/cadence/client"
)

func main() {

	rootCtx := context.Background()
	cc, err := SetupCadenceClient()
	if err != nil {
		panic(err)
	}

	// Start long running workflow
	opts := client.StartWorkflowOptions{
		TaskList:                     "greetings",
		ExecutionStartToCloseTimeout: time.Hour * 1, // Wait 1 hours, make sure you use a high enough time
		// to make sure that the workflow does not timeout before 3 singals are recieved
	}

	// We use Start here since we want to start it but not wait for it to return
	// Execution contains information about the execution such as Workflow ID etc
	// In production, make sure you check if the WOrkflows are already running to avoid  booting up multiple unless wanted
	execution, err := cc.client.StartWorkflow(rootCtx, opts, OrderWorkflow)
	if err != nil {
		panic(err)
	}

	log.Println("Workflow ID: ", execution.ID)
	// Apply Workflows IDs
	cc.SetOrderWorkflowIds(execution.ID, execution.RunID)

	mux := http.NewServeMux()
	mux.HandleFunc("/greetings", cc.GreetUser)
	mux.HandleFunc("/order", cc.Order)

	log.Fatal(http.ListenAndServe("localhost:8080", mux))
}
