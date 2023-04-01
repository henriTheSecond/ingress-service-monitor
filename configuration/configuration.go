package configuration

type IngressServiceMonitorConfiguration struct {
	ConsulScheme                  string
	ConsulAddress                 string
	ConsulToken                   string
	TypeGateway                   string
	IngressTagPrefix              string
	GatewayIngressServiceName     string
	GatewayServicePortHTTP        int
	GatewayServicePortHTTP2       int
	GatewayServicePortTCP         int
	GatewayServicePortGRPC        int
	GatewayServiceName            string
	GatewayServiceHealthCheckPort int
	HealthCheckPort               int
}
