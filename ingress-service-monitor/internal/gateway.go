package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

type Gateway struct {
	oldGateway      *IngressGateway
	tags            *Tags
	serviceDefaults *ServiceDefaults
}

func NewGateway(consul *Consul) *Gateway {
	g := new(Gateway)
	g.tags = NewTags()
	g.serviceDefaults = NewServiceDefaults(consul)
	g.oldGateway = new(IngressGateway)
	return g
}
func (g *Gateway) gatewayHasChanges(oldGateway IngressGateway, gateway IngressGateway) bool {
	if oldGateway.Kind != gateway.Kind {
		return true
	}
	if oldGateway.Name != gateway.Name {
		return true
	}
	if len(oldGateway.Listeners) != len(gateway.Listeners) {
		return true
	}
	for i := 0; i < len(oldGateway.Listeners); i++ {
		if oldGateway.Listeners[i].Port != gateway.Listeners[i].Port {
			return true
		}
		if oldGateway.Listeners[i].Protocol != gateway.Listeners[i].Protocol {
			return true
		}
		if len(oldGateway.Listeners[i].Services) != len(gateway.Listeners[i].Services) {
			return true
		}
		sort.Slice(oldGateway.Listeners[i].Services, func(a, b int) bool {
			return oldGateway.Listeners[i].Services[a].Name < oldGateway.Listeners[i].Services[b].Name
		})

		sort.Slice(gateway.Listeners[i].Services, func(a, b int) bool {
			return gateway.Listeners[i].Services[a].Name < gateway.Listeners[i].Services[b].Name
		})
		for j := 0; j < len(oldGateway.Listeners[i].Services); j++ {
			if len(oldGateway.Listeners[i].Services[j].Hosts) != len(gateway.Listeners[i].Services[j].Hosts) {
				return true
			}
			if g.containsService(oldGateway.Listeners[i].Services, gateway.Listeners[i].Services[j]) == false {
				return true
			}

			for k := 0; k < len(oldGateway.Listeners[i].Services[j].Hosts); k++ {
				if g.contains(oldGateway.Listeners[i].Services[j].Hosts, gateway.Listeners[i].Services[j].Hosts[k]) == false {
					return true
				}
				if g.contains(gateway.Listeners[i].Services[j].Hosts, oldGateway.Listeners[i].Services[j].Hosts[k]) == false {
					return true
				}
			}
		}
	}

	return false
}
func (g *Gateway) contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
func (g *Gateway) containsService(s []IngressGatewayService, str IngressGatewayService) bool {
	for _, v := range s {
		if v.Name == str.Name {
			return true
		}
	}

	return false
}

func (g *Gateway) registerTaggedService(gateway IngressGateway, tags []string) {
	registerServiceURL := fmt.Sprintf("%v/v1/agent/service/register", ConsulHTTPURL)
	if TypeGateway == "traefik" {
		tags = append(tags, "traefik.enable=true")
	}
	register := RegisterConsulService{}
	register.Name = IngressServiceName
	register.ID = fmt.Sprintf("%s-service", IngressServiceName)
	register.Port = IngressPort
	register.Kind = ""
	register.Tags = tags
	sJSON, err := json.Marshal(register)
	if err != nil {
		log.Debug().Msg(err.Error())
	}
	log.Debug().Str("register", string(sJSON)).Msg("registerTaggedService")
	payloadBuf := bytes.NewReader(sJSON)
	req, err := http.NewRequest("PUT", registerServiceURL, payloadBuf)
	if err != nil {
		log.Error().Msg(err.Error())
		time.Sleep(5 * time.Second)
	}
	req.Header = http.Header{
		"X-Consul-Token": {ConsulToken},
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Msg(err.Error())
	}
	if resp != nil {
		if resp.StatusCode != 200 {
			errbody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Debug().Msg("problem reading consul response")
			}
			log.Debug().Str("received body", string(errbody)).Msg("registerTaggedService")
			time.Sleep(5 * time.Second)
		}
		resp.Body.Close()
	}

}

func (g *Gateway) configureGateway(gateway IngressGateway, tags []string) {

	var configURL = fmt.Sprintf("%v/v1/config", ConsulHTTPURL)
	gwJSON, err := json.Marshal(gateway)
	if err != nil {
		log.Debug().Msg(err.Error())
	}
	log.Debug().Msg("HTTP PUT ingress gateway")
	log.Debug().Msg(string(gwJSON))
	payloadBuf := bytes.NewReader(gwJSON)
	req, err := http.NewRequest("PUT", configURL, payloadBuf)
	if err != nil {
		log.Debug().Msg(err.Error())
	}
	req.Header = http.Header{
		"X-Consul-Token": {ConsulToken},
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug().Msg(err.Error())
	}
	if resp != nil {
		if resp.StatusCode != 200 {
			errbody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Debug().Msg("problem reading consul response")
			}
			log.Debug().Str("received body", string(errbody)).Msg("configureGateway")
			time.Sleep(5 * time.Second)
		}
		resp.Body.Close()

		g.registerTaggedService(gateway, tags)
	}
}
func (g *Gateway) addIngressGateway(services map[string]*ConsulService) {
	gateway := IngressGateway{}
	gateway.Name = ServiceNameIngressGateway
	gateway.Kind = "ingress-gateway"
	httpListener := IngressListener{}
	httpListener.Protocol = "http"
	httpListener.Port = IngressPort
	tcpListener := IngressListener{}
	tcpListener.Protocol = "tcp"
	tcpListener.Port = IngressPort
	gwTags := []string{}
	for _, cService := range services {
		ingressTags := []string{}
		createdTags := g.tags.createGatewayTags(cService)
		for i := 0; i < len(createdTags); i++ {
			ingressTags = append(ingressTags, createdTags[i])
		}
		if len(ingressTags) > 0 && !g.containsServiceWithName(httpListener.Services, cService.Service) {
			service := IngressGatewayService{}
			service.Name = cService.Service
			serviceProtocol := g.serviceDefaults.GetServiceProtocol(service.Name)
			if serviceProtocol == "http" {
				hostHeader := g.tags.getHostHeader(cService.Tags)
				if hostHeader != "" {
					service.Hosts = append(service.Hosts, hostHeader)
				}

				// if len(gwTags) > 0 {
				httpListener.Services = append(httpListener.Services, service)
				// }
			}
			gwTags = append(gwTags, ingressTags...)
		}

	}
	if httpListener.Services != nil && len(httpListener.Services) > 0 {
		gateway.Listeners = append(gateway.Listeners, httpListener)
	}
	if tcpListener.Services != nil && len(tcpListener.Services) > 0 {
		gateway.Listeners = append(gateway.Listeners, tcpListener)
	}
	if g.gatewayHasChanges(*g.oldGateway, gateway) == true {
		log.Debug().Msg("gateway has changed, updating gateway")
		g.configureGateway(gateway, gwTags)
		*g.oldGateway = gateway
	}
}
func (g *Gateway) containsServiceWithName(s []IngressGatewayService, serviceName string) bool {
	for _, v := range s {
		if v.Name == serviceName {
			return true
		}
	}

	return false
}
