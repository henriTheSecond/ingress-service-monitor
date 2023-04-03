package backend

import (
	"fmt"
	"placlet/ingress-service-monitor/configuration"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
)

type backend struct {
	client                        *api.Client
	config                        *api.Config
	typeGateway                   string
	ingressTagPrefix              string
	gatewayIngresServiceName      string
	gatewayServicePortHttp        int
	gatewayServiceHealthCheckPort int
	sdManager                     *servicedefaultsmanager
}

func NewBackend(config *configuration.IngressServiceMonitorConfiguration) (*backend, error) {
	b := new(backend)
	b.typeGateway = config.TypeGateway
	b.ingressTagPrefix = config.IngressTagPrefix
	b.gatewayIngresServiceName = config.GatewayIngressServiceName
	b.gatewayServicePortHttp = config.GatewayServicePortHTTP
	b.gatewayServiceHealthCheckPort = config.GatewayServiceHealthCheckPort
	b.config = &api.Config{Address: config.ConsulAddress, Scheme: config.ConsulScheme, Token: config.ConsulToken}
	c, err := api.NewClient(b.config)
	if err != nil {
		return nil, err
	}
	b.client = c
	sdManager := NewServiceDefaultsManager(c, config)
	b.sdManager = &sdManager
	return b, nil
}
func (b *backend) getIngressServices(consulIndex uint64) (map[string][]string, uint64, error) {
	services, meta, err := b.client.Catalog().Services(&api.QueryOptions{
		WaitIndex: consulIndex,
		Filter:    "ServiceKind==\"\" and ServiceName!=\"consul\"",
	})
	if err != nil {
		return nil, consulIndex, err
	}
	filteredMap := make(map[string][]string)
	for key, element := range services {
		for _, val := range element {
			if strings.HasPrefix(val, b.ingressTagPrefix) {
				filteredMap[key] = element
				break
			}
		}
	}
	return filteredMap, meta.LastIndex, nil
}
func (b *backend) registerIngressService(services map[string][]string) error {
	tags := b.getDefaultTags()
	for _, element := range services {
		for _, e := range element {
			if strings.HasPrefix(e, b.ingressTagPrefix) {
				tags = append(tags, b.stripPrefixFromTag(e))
			}
		}
	}
	err := b.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: b.gatewayIngresServiceName,
		Port: b.gatewayServicePortHttp,
		Tags: tags,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://localhost:%d/ready", b.gatewayServiceHealthCheckPort),
			Interval: "30s",
		},
	})
	if err != nil {
		return err
	}
	return nil
}
func (b *backend) stripPrefixFromTag(tag string) string {
	t := strings.TrimPrefix(tag, b.ingressTagPrefix)
	if b.typeGateway == "traefik" {
		return strings.ReplaceAll(t, "'", "`")
	}
	return t
}
func (b *backend) getDefaultTags() []string {
	var tags []string
	if b.typeGateway == "traefik" {
		tags = append(tags, "traefik.enable=true")
	}
	return tags
}
func (b *backend) StartMonitoring(cIndex uint64) {
	log.Info().Str("consulindex", fmt.Sprintf("%v", cIndex)).Msg("getingresservices")
	services, consulIndex, err := b.getIngressServices(cIndex)
	if err != nil {
		log.Err(err).Msg("Error occured")
		time.Sleep(2 * time.Second)
	}
	b.sdManager.StartPolling(services)
	log.Info().Str("consulindex", fmt.Sprintf("%v", cIndex)).Msg("registerIngressService")
	err2 := b.registerIngressService(services)
	if err2 != nil {
		log.Err(err2).Msg("Error occured")
		time.Sleep(2 * time.Second)
	}

	b.StartMonitoring(consulIndex)
}

func (b *backend) pollServicesWithoutDefaults(servicesWhithoutDefaults []string) {
	for _, service := range servicesWhithoutDefaults {
		b.pollServiceWithoutDefaults(service)
	}
}
func (b *backend) pollServiceWithoutDefaults(serviceWithoutDefaults string) {
	b.client.ConfigEntries().Get(api.ServiceDefaults, serviceWithoutDefaults, &api.QueryOptions{})
}
