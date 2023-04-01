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
	gatewayServicePortHttp2       int
	gatewayServicePortTcp         int
	gatewayServicePortGrpc        int
	gatewayServiceName            string
	gatewayServiceHealthCheckPort int
}

func NewBackend(config *configuration.IngressServiceMonitorConfiguration) (*backend, error) {
	b := new(backend)
	b.typeGateway = config.TypeGateway
	b.ingressTagPrefix = config.IngressTagPrefix
	b.gatewayIngresServiceName = config.GatewayIngressServiceName
	b.gatewayServicePortHttp = config.GatewayServicePortHTTP
	b.gatewayServicePortHttp2 = config.GatewayServicePortHTTP2
	b.gatewayServicePortGrpc = config.GatewayServicePortGRPC
	b.gatewayServicePortTcp = config.GatewayServicePortTCP
	b.gatewayServiceHealthCheckPort = config.GatewayServiceHealthCheckPort
	b.gatewayServiceName = config.GatewayServiceName
	b.config = &api.Config{Address: config.ConsulAddress, Scheme: config.ConsulScheme, Token: config.ConsulToken}
	c, err := api.NewClient(b.config)
	if err != nil {
		return nil, err
	}
	b.client = c
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
	log.Info().Str("consulindex", fmt.Sprintf("%v", cIndex)).Msg("registerIngressService")
	err2 := b.registerIngressService(services)
	if err2 != nil {
		log.Err(err2).Msg("Error occured")
		time.Sleep(2 * time.Second)
	}
	log.Info().Str("consulindex", fmt.Sprintf("%v", cIndex)).Msg("configureIngressGateway")
	err3 := b.configureIngressGateway(services)
	if err3 != nil {
		log.Err(err3).Msg("Error occured")
		time.Sleep(2 * time.Second)
	}

	b.StartMonitoring(consulIndex)
}
func (b *backend) configureIngressGateway(services map[string][]string) error {
	var httplistener api.IngressListener
	httplistener.Port = b.gatewayServicePortHttp
	httplistener.Protocol = "http"
	var http2listener api.IngressListener
	http2listener.Port = b.gatewayServicePortHttp2
	http2listener.Protocol = "http2"
	var tcplistener api.IngressListener
	tcplistener.Port = b.gatewayServicePortTcp
	tcplistener.Protocol = "tcp"
	var grpclistener api.IngressListener
	grpclistener.Port = b.gatewayServicePortGrpc
	grpclistener.Protocol = "grpc"
	var unknownServiceDefaults []string

	for key, element := range services {
		for _, tag := range element {
			host := b.getHostFromTag(tag)
			if host != "" {
				var ingressService api.IngressService
				ingressService.Name = key
				ingressService.Hosts = append(ingressService.Hosts, host)
				configEntry, _, configerr := b.client.ConfigEntries().Get(api.ServiceDefaults, key, &api.QueryOptions{})
				if configerr != nil {
					unknownServiceDefaults = append(unknownServiceDefaults, key)
					log.Err(configerr).Msg("Error occured")
					break
				}
				cc, _ := configEntry.(*api.ServiceConfigEntry)
				if cc == nil {
					break
				}
				switch cc.Protocol {
				case "http":
					httplistener.Services = append(httplistener.Services, ingressService)
				case "tcp":
					tcplistener.Services = append(tcplistener.Services, ingressService)
				case "grpc":
					grpclistener.Services = append(grpclistener.Services, ingressService)
				case "http2":
					http2listener.Services = append(http2listener.Services, ingressService)
				default:

				}
			}
		}
	}
	var listeners []api.IngressListener
	if len(httplistener.Services) > 0 {
		listeners = append(listeners, httplistener)
	}
	if len(http2listener.Services) > 0 {
		listeners = append(listeners, http2listener)
	}
	if len(grpclistener.Services) > 0 {
		listeners = append(listeners, grpclistener)
	}
	if len(tcplistener.Services) > 0 {
		listeners = append(listeners, tcplistener)
	}
	success, _, err := b.client.ConfigEntries().Set(&api.IngressGatewayConfigEntry{
		Name:      b.gatewayServiceName,
		Kind:      api.IngressGateway,
		Listeners: listeners,
	}, &api.WriteOptions{})
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("Not able to save the gateway configuration")
	}
	return nil
}
func (b *backend) pollServicesWithoutDefaults(servicesWhithoutDefaults []string) {
	for _, service := range servicesWhithoutDefaults {
		b.pollServiceWithoutDefaults(service)
	}
}
func (b *backend) pollServiceWithoutDefaults(serviceWithoutDefaults string) {
	b.client.ConfigEntries().Get(api.ServiceDefaults, serviceWithoutDefaults, &api.QueryOptions{})
}
func (b *backend) getHostFromTag(tag string) string {
	if b.typeGateway == "traefik" {
		var cleanTag = strings.ReplaceAll(tag, "'", "`")
		index := strings.Index(cleanTag, "`")
		if index > -1 {
			splitted := strings.Split(cleanTag, "`")
			if len(splitted) > 2 {
				return splitted[1]
			}
		}
	}
	if b.typeGateway == "fabio" {
		return tag
	}
	return ""
}
