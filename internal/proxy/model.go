package proxy

type UpstreamRoute struct {
	RouteID       string
	RoutePath     string
	RouteMethod   string
	StripPrefix   bool
	RewriteTarget *string

	ServiceID   string
	ServiceName string
	Protocol    string
	LBStrategy  string
	InstanceID  string
	Host        string
	Port        int
	Weight      int
	TimeoutMS   int

	MatchedInstances int
}
