package orders

import (
	"context"
	"errors"
	"programmingpercy/cadence-tavern/customer"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// Order is a simple type to represent orders made
type Order struct {
	Item  string  `json:"item"`
	Price float32 `json:"price"`
	By    string  `json:"by"`
}

func init() {
	workflow.Register(WorkflowOrder)
	workflow.Register(workflowProcessOrder)

	activity.Register(activityIsCustomerLegal)
	activity.Register(activitiyFindCustomerByName)
}

// MaxSignalsAmount is how many signals we accept before restart
// Cadence recommends a production workflow to have <1000
const MaxSignalsAmount = 3

// WorkflowOrder will handle incomming Orders
// This is exposed so we can use it in api
func WorkflowOrder(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute * 60,
		StartToCloseTimeout:    time.Minute * 60,
		HeartbeatTimeout:       time.Hour * 20,
		// Here we will Add Retry policies etc later
	}
	// Add the Options to Context to apply configurations
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("Waiting for Orders")

	// restartWorkflow
	var restartWorkflow bool
	// signalCounter
	signalCount := 0

	// Preconfigure ChildWorkflow Options
	orderWaiterCfg := workflow.ChildWorkflowOptions{
		ExecutionStartToCloseTimeout: time.Minute * 2, // Each Order can tops take 2 min
	}

	// Grab the Selector from the workflow Context,
	selector := workflow.NewSelector(ctx)
	// For ever running loop
	for {
		// Get the Signal used to identify an Event, we named our Order event into order
		signalChan := workflow.GetSignalChannel(ctx, "order")

		// We add a "Receiver" to the Selector, The receiver is a function that will trigger once a new Signal is recieved
		selector.AddReceive(signalChan, func(c workflow.Channel, more bool) {
			// Create the Order to marshal the Input into
			var order Order
			// Receive will read input data into the struct
			c.Receive(ctx, &order)

			// increment signal counter
			signalCount++
			// Create ctx for Child flow
			orderCtx := workflow.WithChildOptions(ctx, orderWaiterCfg)
			// Trigger the child workflow
			waiter := workflow.ExecuteChildWorkflow(orderCtx, workflowProcessOrder, order)
			if err := waiter.Get(ctx, nil); err != nil {
				workflow.GetLogger(ctx).Error("Order has failed.", zap.Error(err))
			}

		})

		if signalCount >= MaxSignalsAmount {
			// We should restart
			// Add a Default to the selector, which will make sure that this is triggered once all jobs in queue are done
			selector.AddDefault(func() {
				restartWorkflow = true
			})
		}

		selector.Select(ctx)

		// If its time to restart, return the ContinueAsNew
		if restartWorkflow {
			return workflow.NewContinueAsNewError(ctx, WorkflowOrder)
		}

	}
}

// workflowProcessOrder is used to handle orders and will be ran as a CHILD
func workflowProcessOrder(ctx workflow.Context, order Order) error {

	logger := workflow.GetLogger(ctx)
	logger.Info("process order workflow started")
	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
		HeartbeatTimeout:       time.Second * 20,
		// Here we will Add Retry policies etc later
	}
	// Add the Options to Context to apply configurations
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Find Customer from Repo
	var cust customer.Customer
	err := workflow.ExecuteActivity(ctx, activitiyFindCustomerByName, order.By).Get(ctx, &cust)

	if err != nil {
		logger.Error("Customer is not in the Tavern", zap.Error(err))
		return err
	}

	var allowed bool
	err = workflow.ExecuteActivity(ctx, activityIsCustomerLegal, cust).Get(ctx, &allowed)
	if err != nil {
		logger.Error("Customer is not of age", zap.Error(err))
		return err
	}

	logger.Info("Order made", zap.String("item", order.Item), zap.Float32("price", order.Price))
	return nil

}

// activityFindCustomerByName is used to find the Customer is in the Tavern
func activitiyFindCustomerByName(ctx context.Context, name string) (customer.Customer, error) {
	return customer.Database.Get(name)
}

// activityIsCustomerLegal is used to check the age of the customer
func activityIsCustomerLegal(ctx context.Context, visitor customer.Customer) (bool, error) {

	if visitor.Age < 18 {
		return false, errors.New("customer is not old enough, dont serve him")
	}
	return true, nil
}
