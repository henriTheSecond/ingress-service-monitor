package backend

import (
	"fmt"
	"placlet/ingress-service-monitor/configuration"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/syncmap"
)

type servicedefaultentry struct {
	consulIndex uint64
	protocol    string
}
type servicedefaultsmanager struct {
	defaults                *syncmap.Map
	client                  *api.Client
	gatewayServicePortHttp  int
	gatewayServicePortHttp2 int
	gatewayServicePortTcp   int
	gatewayServicePortGrpc  int
	gatewayServiceName      string
	typeGateway             string
	services                *syncmap.Map
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
	sdManager.defaults = &sync.Map{}
	sdManager.services = &sync.Map{}
	return *sdManager
}
func (sd *servicedefaultsmanager) getServiceProtocol(serviceName string) (string, error) {
	value, _ := sd.defaults.Load(serviceName)
	val := value.(*servicedefaultentry)
	return val.protocol, nil
}
func (sd *servicedefaultsmanager) pollServiceDefaults(serviceName string) {
	value, _ := sd.defaults.Load(serviceName)
	val := value.(*servicedefaultentry)
	configEntry, opts, configerr := sd.client.ConfigEntries().Get(api.ServiceDefaults, serviceName, &api.QueryOptions{WaitIndex: val.consulIndex})
	if configerr != nil {
		log.Err(configerr).Msg("Error occured")
		if val.protocol != "" {
			sd.defaults.Store(serviceName, &servicedefaultentry{consulIndex: 0, protocol: ""})
		}
		time.Sleep(5 * time.Second)
	} else {
		log.Info().Str("serviceName", serviceName).Str("consulindex", fmt.Sprintf("%v", opts.LastIndex)).Msg("check service defaults")
		cc, _ := configEntry.(*api.ServiceConfigEntry)
		sd.defaults.Store(serviceName, &servicedefaultentry{consulIndex: opts.LastIndex, protocol: cc.Protocol})
	}
	sd.configureIngressGateway()
	sd.pollServiceDefaults(serviceName)
}
func (sd *servicedefaultsmanager) StartPolling(services map[string][]string) {
	sd.services = new(sync.Map)
	for key, value := range services {
		sd.services.Store(key, value)
	}
	sd.services.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		val, _ := sd.defaults.Load(k)
		if val == nil {
			log.Info().Str("key", k).Msg("starting polling service defaults")
			sd.defaults.Store(k, &servicedefaultentry{consulIndex: 0, protocol: ""})
			go sd.pollServiceDefaults(k)
		}
		return true
	})
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

	sd.services.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		element := value.([]string)
		for _, tag := range element {
			host := sd.getHostFromTag(tag)
			if host != "" {
				var ingressService api.IngressService
				ingressService.Name = k
				ingressService.Hosts = append(ingressService.Hosts, host)
				log.Info().Str("host", host).Msg("Host found")
				protocol, protocolerr := sd.getServiceProtocol(k)
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
		return true
	})
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
	ce, _, _ := sd.client.ConfigEntries().Get(api.IngressGateway, sd.gatewayServiceName, &api.QueryOptions{})
	equalListeners := false
	if ce != nil {
		ingressgatewayConfigEntry := ce.(*api.IngressGatewayConfigEntry)
		equalListeners = sd.equalListeners(ingressgatewayConfigEntry.Listeners, listeners)
	}
	if !equalListeners {
		log.Info().Msg("Adding new ingressgatewayconfigentry")
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
	}
	return nil
}
func (sd *servicedefaultsmanager) equalListeners(l1 []api.IngressListener, l2 []api.IngressListener) bool {
	if l1 == nil && l2 != nil {
		return false
	}
	if l2 == nil && l1 != nil {
		return false
	}
	if len(l1) != len(l2) {
		return false
	}
	for i := 0; i < len(l1); i++ {
		if l1[i].Port != l2[i].Port {
			return false
		}
		if l1[i].Protocol != l2[i].Protocol {
			return false
		}
		if len(l1[i].Services) != len(l2[i].Services) {
			return false
		}
		sort.Slice(l1[i].Services, func(a, b int) bool {
			return l1[i].Services[a].Name < l1[i].Services[b].Name
		})
		sort.Slice(l2[i].Services, func(a, b int) bool {
			return l2[i].Services[a].Name < l2[i].Services[b].Name
		})
		for j := 0; j < len(l1[i].Services); j++ {
			if len(l1[i].Services[j].Hosts) != len(l2[i].Services[j].Hosts) {
				return false
			}
			for k := 0; k < len(l1[i].Services[j].Hosts); k++ {
				if l1[i].Services[j].Hosts[k] != l2[i].Services[j].Hosts[k] {
					return false
				}
			}
		}
	}
	return true
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
		index := strings.Index(tag, "urlprefix-")
		if index > -1 {
			splitted := strings.Split(tag, "urlprefix-")
			if len(splitted) >= 2 {
				splitted2 := strings.Split(splitted[1], "/")
				if len(splitted2) > 0 {
					return splitted2[0]
				}
			}
		}
	}
	return ""
}
