package gateway

type UpstreamRoute struct {
	RoutePath     string
	RouteMethod   string
	StripPrefix   bool
	RewriteTarget *string
	ServiceName   string
	Protocol      string
	Host          string
	Port          int
	TimeoutMS     int
}
