package backend

import (
	"fmt"
	"placlet/ingress-service-monitor/configuration"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
)

type servicedefaultentry struct {
	consulIndex uint64
	protocol    string
}
type servicedefaultsmanager struct {
	defaults                map[string]*servicedefaultentry
	client                  *api.Client
	gatewayServicePortHttp  int
	gatewayServicePortHttp2 int
	gatewayServicePortTcp   int
	gatewayServicePortGrpc  int
	gatewayServiceName      string
	typeGateway             string
	services                map[string][]string
}

func NewServiceDefaultsManager(c *api.Client, config *configuration.IngressServiceMonitorConfiguration) servicedefaultsmanager {
	sdManager := new(servicedefaultsmanager)
	sdManager.gatewayServiceName = config.GatewayServiceName
	sdManager.typeGateway = config.TypeGateway
	sdManager.gatewayServicePortHttp = config.GatewayServicePortHTTP
	sdManager.gatewayServicePortHttp2 = config.GatewayServicePortHTTP2
	sdManager.gatewayServicePortGrpc = config.GatewayServicePortGRPC
	sdManager.gatewayServicePortTcp = config.GatewayServicePortTCP
	sdManager.client = c
	sdManager.defaults = make(map[string]*servicedefaultentry)
	sdManager.services = make(map[string][]string)
	return *sdManager
}
func (sd *servicedefaultsmanager) getServiceProtocol(serviceName string) (string, error) {
	if sd.defaults[serviceName] == nil {
		return "", nil
	}

	return sd.defaults[serviceName].protocol, nil
}
func (sd *servicedefaultsmanager) pollServiceDefaults(serviceName string) {
	configEntry, opts, configerr := sd.client.ConfigEntries().Get(api.ServiceDefaults, serviceName, &api.QueryOptions{WaitIndex: sd.defaults[serviceName].consulIndex})
	if configerr != nil {
		log.Err(configerr).Msg("Error occured")
		if sd.defaults[serviceName].protocol != "" {
			sd.defaults[serviceName] = &servicedefaultentry{consulIndex: 0, protocol: ""}
		}
		time.Sleep(5 * time.Second)
	} else {
		log.Info().Str("serviceName", serviceName).Str("consulindex", fmt.Sprintf("%v", opts.LastIndex)).Msg("check service defaults")
		cc, _ := configEntry.(*api.ServiceConfigEntry)
		sd.defaults[serviceName] = &servicedefaultentry{consulIndex: opts.LastIndex, protocol: cc.Protocol}
	}
	sd.configureIngressGateway()
	sd.pollServiceDefaults(serviceName)
}
func (sd *servicedefaultsmanager) StartPolling(services map[string][]string) {
	sd.services = services
	for key := range sd.services {
		if sd.defaults[key] == nil {
			log.Info().Str("key", key).Msg("starting polling service defaults")
			sd.defaults[key] = &servicedefaultentry{consulIndex: 0, protocol: ""}
			go sd.pollServiceDefaults(key)
		}
	}
	sd.configureIngressGateway()
}
func (sd *servicedefaultsmanager) configureIngressGateway() error {
	var httplistener api.IngressListener
	httplistener.Port = sd.gatewayServicePortHttp
	httplistener.Protocol = "http"
	var http2listener api.IngressListener
	http2listener.Port = sd.gatewayServicePortHttp2
	http2listener.Protocol = "http2"
	var tcplistener api.IngressListener
	tcplistener.Port = sd.gatewayServicePortTcp
	tcplistener.Protocol = "tcp"
	var grpclistener api.IngressListener
	grpclistener.Port = sd.gatewayServicePortGrpc
	grpclistener.Protocol = "grpc"

	for key, element := range sd.services {
		for _, tag := range element {
			host := sd.getHostFromTag(tag)
			if host != "" {
				var ingressService api.IngressService
				ingressService.Name = key
				ingressService.Hosts = append(ingressService.Hosts, host)
				protocol, protocolerr := sd.getServiceProtocol(key)
				if protocolerr != nil {
					break
				}
				switch protocol {
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
	success, _, err := sd.client.ConfigEntries().Set(&api.IngressGatewayConfigEntry{
		Name:      sd.gatewayServiceName,
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
func (sd *servicedefaultsmanager) getHostFromTag(tag string) string {
	if sd.typeGateway == "traefik" {
		var cleanTag = strings.ReplaceAll(tag, "'", "`")
		index := strings.Index(cleanTag, "`")
		if index > -1 {
			splitted := strings.Split(cleanTag, "`")
			if len(splitted) > 2 {
				return splitted[1]
			}
		}
	}
	if sd.typeGateway == "fabio" {
		return tag
	}
	return ""
}
