package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type ServiceDefaults struct {
	serviceDefaultsMonitor *ServiceDefaultsMonitor
	consul                 *Consul
}

func NewServiceDefaults(consul *Consul) *ServiceDefaults {
	sd := new(ServiceDefaults)
	sd.consul = consul
	sd.serviceDefaultsMonitor = NewServiceDefaultsMonitor()
	return sd
}
func (s *ServiceDefaults) waitForServiceDefaultChanges(serviceName string, previousConsulIndex int, previousValue string) {
	if s.serviceDefaultsMonitor.serviceIsMonitored(serviceName) {
		return
	}
	s.serviceDefaultsMonitor.addToMonitoredServices(serviceName)
	for previousConsulIndex > 0 {
		log.Debug().Int("previousConsulIndex", previousConsulIndex).Msg("waitForDefaultChanges")
		req, err := http.NewRequest("GET", s.constructServiceDefaultsURL(serviceName, previousConsulIndex), nil)
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
				s.consul.ResetServices()
			}
		}
	}
}

func (s *ServiceDefaults) pollServiceWithoutDefaults(serviceName string) {
	req, err := http.NewRequest("GET", s.constructServiceDefaultsURL(serviceName, 0), nil)
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
			s.pollServiceWithoutDefaults(serviceName)
		}
		resp.Body.Close()
		s.consul.ResetServices()
	}

}
func (s *ServiceDefaults) constructServiceDefaultsURL(serviceName string, previousConsulIndex int) string {
	servicesURL := fmt.Sprintf("%v/v1/config/service-defaults/%v?index=%v", ConsulHTTPURL, serviceName, previousConsulIndex)
	return servicesURL
}

func (s *ServiceDefaults) GetServiceProtocol(serviceName string) string {
	req, err := http.NewRequest("GET", s.constructServiceDefaultsURL(serviceName, 0), nil)
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
			if !s.serviceDefaultsMonitor.serviceWithoutDefaultsIsMonitored(serviceName) {
				s.serviceDefaultsMonitor.addToMonitoredServicesWithoutDefault(serviceName)
				go s.pollServiceWithoutDefaults(serviceName)
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
		go s.waitForServiceDefaultChanges(serviceName, cIndex, serviceDefaults.Protocol)
		return serviceDefaults.Protocol
	}
	return "tcp"
}
