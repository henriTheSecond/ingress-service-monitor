package internal

type ConsulName struct {
	ServiceName string
}
type IngressService struct {
	Name  string
	Hosts []string
}
type ConsulSelf struct {
	Config *ConsulSelfConfig
}
type ConsulSelfConfig struct {
	NodeName string
}
type ConsulNode struct {
	Services map[string]*ConsulService
}
type ConsulService struct {
	ID      string
	Service string
	Tags    []string
}
type RegisterConsulService struct {
	ID      string
	Name    string
	Address string
	Tags    []string
	Port    int
	Kind    string
}
type IngressGateway struct {
	Kind      string
	Name      string
	TLS       IsEnabled
	Listeners []IngressListener
}
type IsEnabled struct {
	Enabled bool
}
type IngressListener struct {
	Port     int
	Protocol string
	Services []IngressGatewayService
}
type IngressGatewayService struct {
	Name  string
	Hosts []string
}
type ServiceDefault struct {
	Protocol string
}
type FreeIngressHealthCheck struct {
	HTTP      string
	TCP       string
	Interval  string
	ServiceID string
	Name      string
}
