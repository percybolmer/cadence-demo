package customer

import (
	"fmt"
	"time"
)

var (
	// Bad Solution for in mem during tutorial
	Database = NewMemoryCustomers()
)

// Customer is representation of a client in the Tavern
type Customer struct {
	Name string `json:"name"`
	// LastVisit is a timestamp of the last time this visitor came by the tavern
	LastVisit time.Time `json:"lastVisit"`
	// TimesVisited is how many times a user has visited
	TimesVisited int `json:"timesVisited"`
	// Age is the customer age
	Age int `json:"age"`
}

// Repository is the needed methods to be a customer repo
type Repository interface {
	Get(string) (Customer, error)
	Update(Customer) error
}

// MemoryCustomers is used to store information in Memory
type MemoryCustomers struct {
	Customers map[string]Customer
}

// NewMemoryCustomers will init a new in memory storage for customers
func NewMemoryCustomers() MemoryCustomers {
	customers := MemoryCustomers{
		Customers: make(map[string]Customer),
	}

	return customers
}

// Get is used to fetch a customer by Name
func (mc *MemoryCustomers) Get(name string) (Customer, error) {
	// if err := mc.LoadDataFile(); err != nil {
	// 	return Customer{}, err
	// }
	if mc.Customers == nil {
		mc.Customers = make(map[string]Customer)
	}
	if cust, ok := mc.Customers[name]; ok {
		return cust, nil
	}
	return Customer{}, fmt.Errorf("no such customer: %s", name)
}

// Update will override the information about a customer in storage
func (mc *MemoryCustomers) Update(customer Customer) error {
	if mc.Customers == nil {
		mc.Customers = make(map[string]Customer)
	}

	mc.Customers[customer.Name] = customer

	return nil

}
