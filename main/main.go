package main

import (
	"fmt"
	"net/http"
	"os"
	"placlet/ingress-service-monitor/backend"
	"placlet/ingress-service-monitor/configuration"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rs/zerolog"
)

func main() {
	config := &configuration.IngressServiceMonitorConfiguration{
		ConsulScheme:                  "https",
		ConsulAddress:                 "localhost:8500",
		ConsulToken:                   "$",
		TypeGateway:                   "traefik",
		IngressTagPrefix:              "gw-us-east-",
		GatewayIngressServiceName:     "gateway-us-east-ingress",
		GatewayServiceName:            "gateway-us-east",
		GatewayServicePortHTTP:        9997,
		GatewayServicePortHTTP2:       9998,
		GatewayServicePortTCP:         9999,
		GatewayServicePortGRPC:        9996,
		GatewayServiceHealthCheckPort: 19000,
		HealthCheckPort:               10000,
	}
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Info().Msg("Starting ingress-service-monitor")
	newConsulScheme := os.Getenv(strings.ToUpper("consulScheme"))
	if newConsulScheme != "" && (newConsulScheme == "http" || newConsulScheme == "https") {
		config.ConsulScheme = newConsulScheme
	}
	newConsulAddress := os.Getenv(strings.ToUpper("consulAddress"))
	if newConsulAddress != "" {
		config.ConsulAddress = newConsulAddress
	}
	newConsulToken := os.Getenv(strings.ToUpper("consulToken"))
	if newConsulToken != "" {
		config.ConsulToken = newConsulToken
	}
	newIngressTagPrefix := os.Getenv(strings.ToUpper("IngressTagPrefix"))
	if newIngressTagPrefix != "" {
		config.IngressTagPrefix = newIngressTagPrefix
	}
	newGatewayIngresServiceName := os.Getenv(strings.ToUpper("GatewayIngressServiceName"))
	if newGatewayIngresServiceName != "" {
		config.GatewayIngressServiceName = newGatewayIngresServiceName
	}
	newGatewayServiceName := os.Getenv(strings.ToUpper("GatewayServiceName"))
	if newGatewayServiceName != "" {
		config.GatewayServiceName = newGatewayServiceName
	}
	newGatewayServicePortHTTP := os.Getenv(strings.ToUpper("GatewayServicePortHTTP"))
	if newGatewayServicePortHTTP != "" {
		parsedPort, parseError := strconv.Atoi(newGatewayServicePortHTTP)
		if parseError == nil {
			config.GatewayServicePortHTTP = parsedPort
		} else {
			log.Error().Msg("GatewayServicePortHTTP must be an integer")
		}
	}
	newGatewayServicePortHTTP2 := os.Getenv(strings.ToUpper("GatewayServicePortHTTP2"))
	if newGatewayServicePortHTTP != "" {
		parsedPort, parseError := strconv.Atoi(newGatewayServicePortHTTP2)
		if parseError == nil {
			config.GatewayServicePortHTTP2 = parsedPort
		} else {
			log.Error().Msg("GatewayServicePortHTTP2 must be an integer")
		}
	}
	newGatewayServicePortTCP := os.Getenv(strings.ToUpper("GatewayServicePortTCP"))
	if newGatewayServicePortHTTP != "" {
		parsedPort, parseError := strconv.Atoi(newGatewayServicePortTCP)
		if parseError == nil {
			config.GatewayServicePortTCP = parsedPort
		} else {
			log.Error().Msg("GatewayServicePortTCP must be an integer")
		}
	}
	newGatewayServicePortGRPC := os.Getenv(strings.ToUpper("GatewayServicePortGRPC"))
	if newGatewayServicePortHTTP != "" {
		parsedPort, parseError := strconv.Atoi(newGatewayServicePortGRPC)
		if parseError == nil {
			config.GatewayServicePortGRPC = parsedPort
		} else {
			log.Error().Msg("GatewayServicePortGRPC must be an integer")
		}
	}
	newGatewayServiceHealthCheckPort := os.Getenv(strings.ToUpper("GatewayServiceHealthCheckPort"))
	if newGatewayServiceHealthCheckPort != "" {
		parsedPort, parseError := strconv.Atoi(newGatewayServiceHealthCheckPort)
		if parseError == nil {
			config.GatewayServiceHealthCheckPort = parsedPort
		} else {
			log.Error().Msg("GatewayServiceHealthCheckPort must be an integer")
		}
	}
	newHealthCheckPort := os.Getenv(strings.ToUpper("HealthCheckPort"))
	if newHealthCheckPort != "" {
		parsedPort, parseError := strconv.Atoi(newHealthCheckPort)
		if parseError == nil {
			config.HealthCheckPort = parsedPort
		} else {
			log.Error().Msg("HealthCheckPort must be an integer")
		}
	}
	newTypeGateway := os.Getenv(strings.ToUpper("typeGateway")) //possible values fabio|traefik
	if newTypeGateway != "" && (newTypeGateway == "fabio" || newTypeGateway == "traefik") {
		config.TypeGateway = newTypeGateway
	}

	b, err := backend.NewBackend(config)
	if err != nil {
		fmt.Println(err)
	}

	log.Info().Msg("Starting monitoring")
	go b.StartMonitoring(0)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthCheck(w, r)
	})

	log.Info().Msg("Starting healthcheck")
	log.Fatal().Err(http.ListenAndServe(fmt.Sprint(":", config.HealthCheckPort), nil))
}
func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Healthy")
}
