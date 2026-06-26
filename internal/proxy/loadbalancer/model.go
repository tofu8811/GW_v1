package loadbalancer

type Instance struct {
	ID        string
	ServiceID string
	Host      string
	Port      int
	Weight    int
}

type LoadBalancer interface {
    Pick(serviceID string, instances []Instance) (Instance, error)
}