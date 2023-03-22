package main

import (
	"fmt"
	"net/http"
	"os"
	"placlet/consul-ingress-service-monitor/internal"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func setVariables() {
	newTypeGateway := os.Getenv(strings.ToUpper("typeGateway")) //possible values fabio|traefik
	if newTypeGateway != "" && (newTypeGateway == "fabio" || newTypeGateway == "traefik") {
		internal.TypeGateway = newTypeGateway
	}
	log.Info().Str("typeGateway", internal.TypeGateway).Msg("Setup")
	newTagIngressService := os.Getenv(strings.ToUpper("tagIngressService"))
	if newTagIngressService != "" {
		internal.TagIngressService = newTagIngressService
	}
	log.Info().Str("tagIngressService", internal.TagIngressService).Msg("Setup")
	newingressPort := os.Getenv(strings.ToUpper("ingressPort"))
	if newingressPort != "" {
		parsedPort, parseError := strconv.Atoi(newingressPort)
		if parseError == nil {
			internal.IngressPort = parsedPort
		} else {
			log.Error().Msg("ingressport must be an integer")
		}
	}
	log.Info().Int("ingressPort", internal.IngressPort).Msg("Setup")
	newconsulHTTPURL := os.Getenv(strings.ToUpper("consulHTTPURL"))
	if newconsulHTTPURL != "" {
		internal.ConsulHTTPURL = newconsulHTTPURL
	}
	log.Info().Str("consulHTTPURL", internal.ConsulHTTPURL).Msg("Setup")
	newServiceNameIngressGateway := os.Getenv(strings.ToUpper("ServiceNameIngressGateway"))
	if newServiceNameIngressGateway != "" {
		internal.ServiceNameIngressGateway = newServiceNameIngressGateway
	}
	log.Info().Str("ServiceNameIngressGateway", internal.ServiceNameIngressGateway).Msg("Setup")
	newIngressServiceName := os.Getenv(strings.ToUpper("IngressServiceName"))
	if newIngressServiceName != "" {
		internal.IngressServiceName = newIngressServiceName
	}
	log.Info().Str("IngressServiceName", internal.IngressServiceName).Msg("Setup")

	newConsulToken := os.Getenv(strings.ToUpper("ConsulToken"))
	if newConsulToken != "" {
		internal.ConsulToken = newConsulToken
	}
	log.Info().Str("ConsulToken", internal.ConsulToken).Msg("Setup")

	newIngressHealthCheckPort := os.Getenv(strings.ToUpper("IngressHealthCheckPort"))
	if newIngressHealthCheckPort != "" {
		parsedPort, parseError := strconv.Atoi(newIngressHealthCheckPort)
		if parseError == nil {
			internal.IngressHealthCheckPort = parsedPort
		} else {
			log.Error().Msg("IngressHealthCheckPort must be an integer")
		}
	}
	log.Info().Int("IngressHealthCheckPort", internal.IngressHealthCheckPort).Msg("Setup")
}
func setTestCase1() {

	internal.TypeGateway = "traefik"
	internal.TagIngressService = "gw-us-east-"
	internal.ConsulHTTPURL = "https://localhost:8499"
	internal.ServiceNameIngressGateway = "gateway-us-east"
	internal.IngressServiceName = "gateway-us-east-ingress"
	internal.ConsulToken = "5d81b656-e6de-0c6e-505d-899a21bd2963"
}
func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	setVariables()
	// setTestCase1()
	time.Sleep(2 * time.Second)
	consul := internal.NewConsul()
	go consul.PollIngressServices()
	port := internal.PortNumber
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthCheck(w, r)
	})
	log.Fatal().Err(http.ListenAndServe(fmt.Sprint(":", port), nil))
}
func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Healthy")
}
