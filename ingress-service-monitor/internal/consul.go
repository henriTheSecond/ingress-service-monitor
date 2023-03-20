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
var previousConsulIndex = 0

func constructConsulNodeServicesURL(currentIndex int) string {
	servicesURL := fmt.Sprintf("%v/v1/catalog/node/%v?index=%v&filter=Service!=consul+and+Kind==\"\"", ConsulHTTPURL, getNodeName(), currentIndex)
	log.Debug().Str("servicesURL", servicesURL).Msg("constructConsulNodeServicesURL")
	return servicesURL
}
func constructServiceDefaultsURL(serviceName string, previousConsulIndex int) string {
	servicesURL := fmt.Sprintf("%v/v1/config/service-defaults/%v?index=%v", ConsulHTTPURL, serviceName, previousConsulIndex)
	return servicesURL
}

func getNodeName() string {
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

func getNodeServices(previousConsulIndex int) *http.Response {
	req, err := http.NewRequest("GET", constructConsulNodeServicesURL(previousConsulIndex), nil)
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
func getServicesOnNode(node ConsulNode) map[string]*ConsulService {
	return node.Services
}
func getIngressServices(previousConsulIndex int) (map[string]*ConsulService, int) {
	resp := getNodeServices(previousConsulIndex)
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
		consulIndex := getConsulIndex(resp)
		resp.Body.Close()
		return getServicesOnNode(node), consulIndex
	}
	return nil, 0
}
func registerTaggedService(gateway IngressGateway, tags []string) {
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
func registerTaggedServiceHealthCheck() {
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

func configureGateway(gateway IngressGateway, tags []string) {

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

		registerTaggedService(gateway, tags)
	}

}
func waitForServiceDefaultChanges(serviceName string, previousConsulIndex int, previousValue string) {
	if serviceIsMonitored(serviceName) {
		return
	}
	addToMonitoredServices(serviceName)
	for previousConsulIndex > 0 {
		log.Debug().Int("previousConsulIndex", previousConsulIndex).Msg("waitForDefaultChanges")
		req, err := http.NewRequest("GET", constructServiceDefaultsURL(serviceName, previousConsulIndex), nil)
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
			previousConsulIndex = getConsulIndex(resp)
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
				checkIngressGateway(0)
			}
		}

	}
}
func pollServiceWithoutDefaults(serviceName string) {
	req, err := http.NewRequest("GET", constructServiceDefaultsURL(serviceName, 0), nil)
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
			pollServiceWithoutDefaults(serviceName)
		}
		resp.Body.Close()
		checkIngressGateway(0)
	}

}
func getServiceProtocol(serviceName string) string {
	// return "http"
	req, err := http.NewRequest("GET", constructServiceDefaultsURL(serviceName, 0), nil)
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
			if !serviceWithoutDefaultsIsMonitored(serviceName) {
				addToMonitoredServicesWithoutDefault(serviceName)
				go pollServiceWithoutDefaults(serviceName)
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
		cIndex := getConsulIndex(resp)
		resp.Body.Close()
		go waitForServiceDefaultChanges(serviceName, cIndex, serviceDefaults.Protocol)
		return serviceDefaults.Protocol
	}
	return "tcp"
}
func addIngressGateway(services map[string]*ConsulService) {
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
		createdTags := createGatewayTags(cService)
		for i := 0; i < len(createdTags); i++ {
			ingressTags = append(ingressTags, createdTags[i])
		}
		if len(ingressTags) > 0 && !containsServiceWithName(httpListener.Services, cService.Service) {
			service := IngressGatewayService{}
			service.Name = cService.Service
			serviceProtocol := getServiceProtocol(service.Name)
			if serviceProtocol == "http" {
				hostHeader := getHostHeader(cService.Tags)
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
	if gatewayHasCHanges(oldGateway, gateway) == true {
		log.Debug().Msg("gateway has changed, updating gateway")
		configureGateway(gateway, gwTags)
		oldGateway = gateway
	}

}
