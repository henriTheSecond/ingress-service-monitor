package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	previousConsulIndex int
	gateway             *Gateway
}

func NewConsul() *Consul {
	c := new(Consul)
	c.previousConsulIndex = 0
	c.gateway = NewGateway(c)
	return c
}
func (c *Consul) constructConsulServicesURL(currentIndex int) string {
	servicesURL := fmt.Sprintf("%v/v1/catalog/services?index=%v&filter=ServiceKind==\"\" and ServiceName!=\"consul\"", ConsulHTTPURL, currentIndex)
	log.Debug().Str("servicesURL", servicesURL).Msg("constructConsulNodeServicesURL")
	return servicesURL
}
func (c *Consul) ResetServices() {
	c.checkIngressGateway(0)
}
func (c *Consul) getConsulServices(previousConsulIndex int) *http.Response {
	req, err := http.NewRequest("GET", c.constructConsulServicesURL(previousConsulIndex), nil)
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

func (c *Consul) getIngressServices(previousConsulIndex int) (map[string]*ConsulService, int) {
	resp := c.getConsulServices(previousConsulIndex)
	if resp != nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error().Msg("problem reading consul response")
		}
		node := new(ConsulNode)
		node.Services = make(map[string]*ConsulService)
		serviceMap := map[string][]string{}
		parseErr := json.Unmarshal(body, &serviceMap)
		if parseErr != nil {
			log.Error().Msg("Error parsing json")
		}
		for service, tags := range serviceMap {
			consulService := new(ConsulService)
			consulService.Service = service
			consulService.Tags = tags
			node.Services[service] = consulService
		}
		consulIndex := getConsulIndex(resp)
		resp.Body.Close()
		return node.Services, consulIndex
	}
	return nil, 0
}

func (c *Consul) checkIngressGateway(previousConsulIndex int) int {
	serviceMap, consulIndex := c.getIngressServices(previousConsulIndex)
	c.gateway.addIngressGateway(serviceMap)
	return consulIndex
}

func (c *Consul) PollIngressServices() {

	log.Debug().Msg("pollIngressServices")
	if c.previousConsulIndex > -1 {
		c.previousConsulIndex = c.checkIngressGateway(c.previousConsulIndex)
	}
	c.registerTaggedServiceHealthCheck()
	c.PollIngressServices()
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
			time.Sleep(5 * time.Second)
		}
		resp.Body.Close()
	}
}
func getConsulIndex(consulResponse *http.Response) int {
	if consulResponse != nil && consulResponse.Header != nil && consulResponse.Header["X-Consul-Index"] != nil && len(consulResponse.Header) > 0 && len(consulResponse.Header["X-Consul-Index"]) > 0 {
		consulIndex, err := strconv.Atoi(consulResponse.Header["X-Consul-Index"][0])
		if err != nil {
			log.Error().Msg("no valid consul index")
		}
		return consulIndex
	}
	return 0
}
