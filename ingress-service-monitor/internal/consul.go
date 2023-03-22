package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

var IngressPort = 9997

var ConsulHTTPURL = "http://127.0.0.1:8500"
var PortNumber = 10000
var ServiceNameIngressGateway = "ingress-gateway"
var IngressServiceName = "ingress"
var IngressHealthCheckPort = 8443
var ConsulToken = ""

type Consul struct {
	serviceMonitor      *ServiceMonitor
	previousConsulIndex int
	oldGateway          *IngressGateway
}

func NewConsul() *Consul {
	c := new(Consul)
	c.serviceMonitor = NewServiceMonitor()
	c.previousConsulIndex = -1
	return c
}
func (c *Consul) constructConsulNodeServicesURL(currentIndex int) string {
	servicesURL := fmt.Sprintf("%v/v1/catalog/node/%v?index=%v&filter=Service!=consul+and+Kind==\"\"", ConsulHTTPURL, c.getNodeName(), currentIndex)
	log.Debug().Str("servicesURL", servicesURL).Msg("constructConsulNodeServicesURL")
	return servicesURL
}
func (c *Consul) constructServiceDefaultsURL(serviceName string, previousConsulIndex int) string {
	servicesURL := fmt.Sprintf("%v/v1/config/service-defaults/%v?index=%v", ConsulHTTPURL, serviceName, previousConsulIndex)
	return servicesURL
}

func (c *Consul) getNodeName() string {
	nodeNameURL := fmt.Sprintf("%v/v1/agent/self", ConsulHTTPURL)
	log.Debug().Str("nodenameurl", nodeNameURL).Msg("getNodeName")
	req, err := http.NewRequest("GET", nodeNameURL, nil)
	req.Header = http.Header{
		"X-Consul-Token": {ConsulToken},
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Msg("Problem calling consul api")
		log.Error().Msg(err.Error())
		time.Sleep(5 * time.Second)
	}
	if resp != nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error().Msg("problem reading consul response")
		}
		// log.Debug().Str("received body", string(body)).Msg("getNodeName")
		self := ConsulSelf{}
		parseErr := json.Unmarshal(body, &self)
		if parseErr != nil {
			log.Error().Msg("Error parsing json")
		}
		resp.Body.Close()
		if self.Config != nil {
			return self.Config.NodeName
		}
	}
	return ""
}

func (c *Consul) getNodeServices(previousConsulIndex int) *http.Response {
	req, err := http.NewRequest("GET", c.constructConsulNodeServicesURL(previousConsulIndex), nil)
	req.Header = http.Header{
		"X-Consul-Token": {ConsulToken},
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Msg("Problem calling consul api")
		log.Error().Msg(err.Error())
		time.Sleep(5 * time.Second)
	}
	return resp
}
func (c *Consul) getConsulIndex(consulResponse *http.Response) int {
	if consulResponse != nil && consulResponse.Header != nil && consulResponse.Header["X-Consul-Index"] != nil && len(consulResponse.Header) > 0 && len(consulResponse.Header["X-Consul-Index"]) > 0 {
		consulIndex, err := strconv.Atoi(consulResponse.Header["X-Consul-Index"][0])
		if err != nil {
			log.Error().Msg("no valid consul index")
		}
		return consulIndex
	}
	return 0
}
func (c *Consul) getServicesOnNode(node ConsulNode) map[string]*ConsulService {
	return node.Services
}
func (c *Consul) getIngressServices(previousConsulIndex int) (map[string]*ConsulService, int) {
	resp := c.getNodeServices(previousConsulIndex)
	if resp != nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error().Msg("problem reading consul response")
		}
		// log.Debug().Str("received body", string(body)).Msg("getIngressServices")
		node := ConsulNode{}
		parseErr := json.Unmarshal(body, &node)
		if parseErr != nil {
			log.Error().Msg("Error parsing json")
		}
		consulIndex := c.getConsulIndex(resp)
		resp.Body.Close()
		return c.getServicesOnNode(node), consulIndex
	}
	return nil, 0
}
func (c *Consul) registerTaggedService(gateway IngressGateway, tags []string) {
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
		}
		resp.Body.Close()
	}

}
func (c *Consul) registerTaggedServiceHealthCheck() {
	var registerCheckURL = fmt.Sprintf("%v/v1/agent/check/register", ConsulHTTPURL)
	register := FreeIngressHealthCheck{}
	register.ServiceID = fmt.Sprintf("%s-service", IngressServiceName)
	register.Name = "ingressServiceHealthCheck"
	register.HTTP = fmt.Sprintf("http://127.0.0.1:%d/ready", IngressHealthCheckPort)
	register.Interval = "10s"
	sJSON, err := json.Marshal(register)
	if err != nil {
		log.Error().Msg(err.Error())
	}

	log.Debug().Str("registring ingress healthcheck:", string(sJSON))
	payloadBuf := bytes.NewReader(sJSON)
	req, err := http.NewRequest("PUT", registerCheckURL, payloadBuf)
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
			log.Debug().Str("received body", string(errbody)).Msg("RegisterTaggedServiceHealthCheck")
		}
		resp.Body.Close()
	}
}

func (c *Consul) configureGateway(gateway IngressGateway, tags []string) {

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
		}
		resp.Body.Close()

		c.registerTaggedService(gateway, tags)
	}
}
func (c *Consul) waitForServiceDefaultChanges(serviceName string, previousConsulIndex int, previousValue string) {
	if c.serviceMonitor.serviceIsMonitored(serviceName) {
		return
	}
	c.serviceMonitor.addToMonitoredServices(serviceName)
	for previousConsulIndex > 0 {
		log.Debug().Int("previousConsulIndex", previousConsulIndex).Msg("waitForDefaultChanges")
		req, err := http.NewRequest("GET", c.constructServiceDefaultsURL(serviceName, previousConsulIndex), nil)
		req.Header = http.Header{
			"X-Consul-Token": {ConsulToken},
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Debug().Msg("Problem calling consul api")
			log.Debug().Msg(err.Error())
			time.Sleep(5 * time.Second)
		}
		if resp != nil {
			previousConsulIndex = c.getConsulIndex(resp)
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Debug().Msg("Problem calling consul api")
				log.Debug().Msg(err.Error())
				time.Sleep(5 * time.Second)
			}
			serviceDefaults := ServiceDefault{}
			parseErr := json.Unmarshal(body, &serviceDefaults)
			if parseErr != nil {
				log.Debug().Msg("Problem calling consul api")
				log.Debug().Msg(parseErr.Error())
				time.Sleep(5 * time.Second)
			}
			resp.Body.Close()
			if serviceDefaults.Protocol != previousValue {
				log.Debug().Str("forcing refresh for", serviceName).Msg("waitForServiceDefaultChanges")
				c.checkIngressGateway(0)
			}
		}

	}
}
func (c *Consul) pollServiceWithoutDefaults(serviceName string) {
	req, err := http.NewRequest("GET", c.constructServiceDefaultsURL(serviceName, 0), nil)
	req.Header = http.Header{
		"X-Consul-Token": {ConsulToken},
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug().Msg("Problem calling consul api")
		log.Debug().Msg(err.Error())
		time.Sleep(5 * time.Second)
	}
	if resp != nil {
		if resp.StatusCode != 200 {
			resp.Body.Close()
			log.Debug().Str(serviceName, "no service defaults found for").Msg("sleeping 5s")
			time.Sleep(5 * time.Second)
			c.pollServiceWithoutDefaults(serviceName)
		}
		resp.Body.Close()
		c.checkIngressGateway(0)
	}

}
func (c *Consul) checkIngressGateway(previousConsulIndex int) int {
	serviceMap, consulIndex := c.getIngressServices(previousConsulIndex)
	c.addIngressGateway(serviceMap)
	return consulIndex
}
func (c *Consul) getServiceProtocol(serviceName string) string {
	// return "http"
	req, err := http.NewRequest("GET", c.constructServiceDefaultsURL(serviceName, 0), nil)
	req.Header = http.Header{
		"X-Consul-Token": {ConsulToken},
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug().Msg("Problem calling consul api")
		log.Debug().Msg(err.Error())
		time.Sleep(5 * time.Second)
	}
	if resp != nil {
		if resp.StatusCode != 200 {
			if !c.serviceMonitor.serviceWithoutDefaultsIsMonitored(serviceName) {
				c.serviceMonitor.addToMonitoredServicesWithoutDefault(serviceName)
				go c.pollServiceWithoutDefaults(serviceName)
			}
			return "tcp"
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Debug().Msg("Problem calling consul api")
			log.Debug().Msg(err.Error())
			time.Sleep(5 * time.Second)
		}
		serviceDefaults := ServiceDefault{}
		parseErr := json.Unmarshal(body, &serviceDefaults)
		if parseErr != nil {
			log.Debug().Msg("Problem calling consul api")
			log.Debug().Msg(err.Error())
			time.Sleep(5 * time.Second)
		}
		cIndex := c.getConsulIndex(resp)
		resp.Body.Close()
		go c.waitForServiceDefaultChanges(serviceName, cIndex, serviceDefaults.Protocol)
		return serviceDefaults.Protocol
	}
	return "tcp"
}
func (c *Consul) PollIngressServices() {

	log.Debug().Msg("pollIngressServices")
	if c.previousConsulIndex > -1 {
		c.previousConsulIndex = c.checkIngressGateway(c.previousConsulIndex)
	}
	c.registerTaggedServiceHealthCheck()
	c.PollIngressServices()
}
func (c *Consul) addIngressGateway(services map[string]*ConsulService) {
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
		createdTags := c.serviceMonitor.createGatewayTags(cService)
		for i := 0; i < len(createdTags); i++ {
			ingressTags = append(ingressTags, createdTags[i])
		}
		if len(ingressTags) > 0 && !c.containsServiceWithName(httpListener.Services, cService.Service) {
			service := IngressGatewayService{}
			service.Name = cService.Service
			serviceProtocol := c.getServiceProtocol(service.Name)
			if serviceProtocol == "http" {
				hostHeader := c.serviceMonitor.getHostHeader(cService.Tags)
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
	if c.gatewayHasChanges(*c.oldGateway, gateway) == true {
		log.Debug().Msg("gateway has changed, updating gateway")
		c.configureGateway(gateway, gwTags)
		*c.oldGateway = gateway
	}
}
func (c *Consul) gatewayHasChanges(oldGateway IngressGateway, gateway IngressGateway) bool {
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
			if c.containsService(oldGateway.Listeners[i].Services, gateway.Listeners[i].Services[j]) == false {
				return true
			}

			for k := 0; k < len(oldGateway.Listeners[i].Services[j].Hosts); k++ {
				if c.contains(oldGateway.Listeners[i].Services[j].Hosts, gateway.Listeners[i].Services[j].Hosts[k]) == false {
					return true
				}
				if c.contains(gateway.Listeners[i].Services[j].Hosts, oldGateway.Listeners[i].Services[j].Hosts[k]) == false {
					return true
				}
			}
		}
	}

	return false
}
func (c *Consul) contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
func (c *Consul) containsService(s []IngressGatewayService, str IngressGatewayService) bool {
	for _, v := range s {
		if v.Name == str.Name {
			return true
		}
	}

	return false
}
func (c *Consul) containsServiceWithName(s []IngressGatewayService, serviceName string) bool {
	for _, v := range s {
		if v.Name == serviceName {
			return true
		}
	}

	return false
}
