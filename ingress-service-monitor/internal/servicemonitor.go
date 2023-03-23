package internal

import (
	"strings"
)

var TypeGateway = "traefik" //possible values fabio|traefik
var TagIngressService = "ingressprefix-"

type ServiceDefaultsMonitor struct {
	monitoredServices                []string
	monitoredServicesWithoutDefaults []string
}

func NewServiceDefaultsMonitor() *ServiceDefaultsMonitor {
	p := new(ServiceDefaultsMonitor)
	return p
}

func (p *ServiceDefaultsMonitor) serviceIsMonitored(serviceName string) bool {
	for i := 0; i < len(p.monitoredServices); i++ {
		if p.monitoredServices[i] == serviceName {
			return true
		}
	}
	return false
}
func (p *ServiceDefaultsMonitor) addToMonitoredServices(serviceName string) {
	if !p.serviceIsMonitored(serviceName) {
		p.monitoredServices = append(p.monitoredServices, serviceName)
	}
}
func (p *ServiceDefaultsMonitor) serviceWithoutDefaultsIsMonitored(serviceName string) bool {
	for i := 0; i < len(p.monitoredServicesWithoutDefaults); i++ {
		if p.monitoredServicesWithoutDefaults[i] == serviceName {
			return true
		}
	}
	return false
}
func (p *ServiceDefaultsMonitor) addToMonitoredServicesWithoutDefault(serviceName string) {
	if !p.serviceWithoutDefaultsIsMonitored(serviceName) {
		p.monitoredServicesWithoutDefaults = append(p.monitoredServicesWithoutDefaults, serviceName)
	}
}

func (p *ServiceDefaultsMonitor) getIngressURL(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, TagIngressService) {
			return strings.TrimPrefix(tag, TagIngressService)
		}
	}
	return ""
}
