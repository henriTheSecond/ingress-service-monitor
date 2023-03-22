package internal

import (
	"fmt"
	"strings"
)

var TypeGateway = "traefik" //possible values fabio|traefik
var TagIngressService = "ingressprefix-"

type ServiceMonitor struct {
	monitoredServices                []string
	monitoredServicesWithoutDefaults []string
}

func NewServiceMonitor() *ServiceMonitor {
	p := new(ServiceMonitor)
	return p
}

func (p *ServiceMonitor) serviceIsMonitored(serviceName string) bool {
	for i := 0; i < len(p.monitoredServices); i++ {
		if p.monitoredServices[i] == serviceName {
			return true
		}
	}
	return false
}
func (p *ServiceMonitor) addToMonitoredServices(serviceName string) {
	if !p.serviceIsMonitored(serviceName) {
		p.monitoredServices = append(p.monitoredServices, serviceName)
	}
}
func (p *ServiceMonitor) serviceWithoutDefaultsIsMonitored(serviceName string) bool {
	for i := 0; i < len(p.monitoredServicesWithoutDefaults); i++ {
		if p.monitoredServicesWithoutDefaults[i] == serviceName {
			return true
		}
	}
	return false
}
func (p *ServiceMonitor) addToMonitoredServicesWithoutDefault(serviceName string) {
	if !p.serviceWithoutDefaultsIsMonitored(serviceName) {
		p.monitoredServicesWithoutDefaults = append(p.monitoredServicesWithoutDefaults, serviceName)
	}
}

func (p *ServiceMonitor) createGatewayTags(cService *ConsulService) []string {
	gwTags := []string{}
	for _, tag := range cService.Tags {
		if strings.HasPrefix(tag, TagIngressService) {
			gwTags = append(gwTags, p.createGatewayTag(tag))
		}
	}
	return gwTags
}
func (p *ServiceMonitor) createGatewayTag(tag string) string {
	if TypeGateway == "traefik" {
		return fmt.Sprintf("%s", strings.Trim(strings.TrimPrefix(strings.ReplaceAll(tag, "'", "`"), TagIngressService), "/"))
	}
	return fmt.Sprintf("urlprefix-%s/", strings.Trim(strings.TrimPrefix(tag, TagIngressService), "/"))
}
func (p *ServiceMonitor) getIngressURL(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, TagIngressService) {
			return strings.TrimPrefix(tag, TagIngressService)
		}
	}
	return ""
}
func (p *ServiceMonitor) getHostHeader(ingressUrls []string) string {
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
