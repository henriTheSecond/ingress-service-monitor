package internal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

var TypeGateway = "traefik" //possible values fabio|traefik

var TagIngressService = "ingressprefix-"
var monitoredServices []string
var monitoredServicesWithoutDefaults []string
var oldGateway IngressGateway

func checkIngressGateway(previousConsulIndex int) int {
	serviceMap, consulIndex := getIngressServices(previousConsulIndex)
	addIngressGateway(serviceMap)
	return consulIndex
}
func PollIngressServices() {

	log.Debug().Msg("pollIngressServices")
	if previousConsulIndex > -1 {
		previousConsulIndex = checkIngressGateway(previousConsulIndex)
	}
	registerTaggedServiceHealthCheck()
	PollIngressServices()
}

func serviceIsMonitored(serviceName string) bool {
	for i := 0; i < len(monitoredServices); i++ {
		if monitoredServices[i] == serviceName {
			return true
		}
	}
	return false
}
func addToMonitoredServices(serviceName string) {
	if !serviceIsMonitored(serviceName) {
		monitoredServices = append(monitoredServices, serviceName)
	}
}
func serviceWithoutDefaultsIsMonitored(serviceName string) bool {
	for i := 0; i < len(monitoredServicesWithoutDefaults); i++ {
		if monitoredServicesWithoutDefaults[i] == serviceName {
			return true
		}
	}
	return false
}
func addToMonitoredServicesWithoutDefault(serviceName string) {
	if !serviceWithoutDefaultsIsMonitored(serviceName) {
		monitoredServicesWithoutDefaults = append(monitoredServicesWithoutDefaults, serviceName)
	}
}

func gatewayHasCHanges(oldGateway IngressGateway, gateway IngressGateway) bool {
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
			if containsService(oldGateway.Listeners[i].Services, gateway.Listeners[i].Services[j]) == false {
				return true
			}

			for k := 0; k < len(oldGateway.Listeners[i].Services[j].Hosts); k++ {
				if contains(oldGateway.Listeners[i].Services[j].Hosts, gateway.Listeners[i].Services[j].Hosts[k]) == false {
					return true
				}
				if contains(gateway.Listeners[i].Services[j].Hosts, oldGateway.Listeners[i].Services[j].Hosts[k]) == false {
					return true
				}
			}
		}
	}

	return false
}
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
func containsService(s []IngressGatewayService, str IngressGatewayService) bool {
	for _, v := range s {
		if v.Name == str.Name {
			return true
		}
	}

	return false
}
func containsServiceWithName(s []IngressGatewayService, serviceName string) bool {
	for _, v := range s {
		if v.Name == serviceName {
			return true
		}
	}

	return false
}
func createGatewayTags(cService *ConsulService) []string {
	gwTags := []string{}
	for _, tag := range cService.Tags {
		if strings.HasPrefix(tag, TagIngressService) {
			gwTags = append(gwTags, createGatewayTag(tag))
		}
	}
	return gwTags
}
func createGatewayTag(tag string) string {
	if TypeGateway == "traefik" {
		return fmt.Sprintf("%s", strings.Trim(strings.TrimPrefix(strings.ReplaceAll(tag, "'", "`"), TagIngressService), "/"))
	}
	return fmt.Sprintf("urlprefix-%s/", strings.Trim(strings.TrimPrefix(tag, TagIngressService), "/"))
}
func getIngressURL(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, TagIngressService) {
			return strings.TrimPrefix(tag, TagIngressService)
		}
	}
	return ""
}
func getHostHeader(ingressUrls []string) string {
	for i := 0; i < len(ingressUrls); i++ {
		if TypeGateway == "traefik" {
			firstindex := strings.Index(ingressUrls[i], "`")
			if firstindex > -1 {
				lastindex := strings.LastIndex(ingressUrls[i], "`")
				if lastindex > -1 {
					return ingressUrls[i][firstindex+1 : lastindex]

				}
			}
			firstindex = strings.Index(ingressUrls[i], "'")
			if firstindex > -1 {
				lastindex := strings.LastIndex(ingressUrls[i], "'")
				if lastindex > -1 {
					return ingressUrls[i][firstindex+1 : lastindex]

				}
			}
		}
		if TypeGateway == "fabio" {
			if ingressUrls[i] != "" {
				return ingressUrls[i]
			}
		}
	}
	return ""
}
