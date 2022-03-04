package greetings

import (
	"context"
	"programmingpercy/cadence-tavern/customer"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

var (
	visitorCount = 0
)

func init() {
	// init will be called once the workflow file is imported
	// this will Register the workflow to the Worker service
	workflow.Register(workflowGreetings)
	// Register the activities also
	activity.Register(activityGreetings)
	activity.Register(activityStoreCustomer)
}

// workflowGreetings is the Workflow that is used to handle new Customers in the Tavern.
// our Workflow accepts a customer as Input, and Outputs a Customer, and an Error
func workflowGreetings(ctx workflow.Context, visitor customer.Customer) (customer.Customer, error) {
	// workflow Options for HeartBeat Timeout and other Timeouts.
	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
		HeartbeatTimeout:       time.Second * 20,
		// Here we will Add Retry policies etc later
	}
	// Add the Options to Context to apply configurations
	ctx = workflow.WithActivityOptions(ctx, ao)
	// Grab the Logger that is configured on the Workflow
	logger := workflow.GetLogger(ctx)
	logger.Info("greetings workflow started")

	// Execute the activityGreetings and Wait for the Response with GET
	// GET() will Block until the activitiy is Completed.
	// Get accepts input to marshal result to,
	// ExecuteActivity returns a FUTURE, so if you want async you can simply Skip .Get
	// Get takes in a interface{} as input that we can use to Scan the result into.
	err := workflow.ExecuteActivity(ctx, activityGreetings, visitor).Get(ctx, &visitor)
	if err != nil {
		logger.Error("Greetings Activity failed", zap.Error(err))
		return customer.Customer{}, err
	}

	err = workflow.ExecuteActivity(ctx, activityStoreCustomer, visitor).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to update customer", zap.Error(err))
		return customer.Customer{}, err
	}

	// Let us wait for orders

	// The output of the Workflow is a Visitor with filled information
	return visitor, nil
}

// activityGreetings is used to say Hello to a Customer and change their LastVisit and TimesVisisted
// The returned value will be a Customer struct filled with this information
func activityGreetings(ctx context.Context, visitor customer.Customer) (customer.Customer, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Greetings activity started")
	logger.Info("New Visitor", zap.String("customer", visitor.Name), zap.Int("visitorCount", visitorCount))
	visitorCount++

	oldCustomerInfo, _ := customer.Database.Get(visitor.Name)

	visitor.LastVisit = time.Now()
	visitor.TimesVisited = oldCustomerInfo.TimesVisited + 1
	return visitor, nil
}

// activityStoreCustomer is used to store the Customer in the configured Customer Storage.
func activityStoreCustomer(ctx context.Context, visitor customer.Customer) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Store Customer activity started")
	logger.Info("Updating Customer", zap.String("customer", visitor.Name), zap.Time("lastVisit", visitor.LastVisit),
		zap.Int("timesVisited", visitor.TimesVisited))

	// Store Customer in Database (Memory Cache during this Example)
	err := customer.Database.Update(visitor)
	if err != nil {
		return err
	}
	return nil
}
