# ingress-service-monitor
## The problem
The Consul Connect service mesh can be used to discover and secure services.
The difficulty is to safely direct traffic into the cluster. You probably have something like traefik or fabio already taking care of this in some way. But how can we dynamically find services in consul that can be accessed through fabio or traefik? And how do we make this work with an ingress gateway?
## The solution
The ingress-service-monitor monitors the cluster for services that can be accessed through the gateway. It does this based on a tag-prefix (see below).
### How does it work?
1. You have a service you want to expose to the outside world.
2. You tag it with the tag prefix: e.g.: gw-us-east-traefik.http.routers.service6000.tls=true, et al
2. ISM starts a consul service on the port that the ingress gateway will listen on (this port is configurable). We call this the ingress-service.
3. ISM searches all the services in the consul cluster that have a certain tag prefix (configurable).
4. For every tag, ISM trims off the prefix and places this tag on the ingress-service. It also registers the service with the ingress gateway.
5. Traefik/fabio detects that there is a service with tags they can work with, and start serving traffic through the ingress gateway.
### Configuration
Environment-variables:
- TYPEGATEWAY: traefik|fabio
- TAGINGRESSSERVICE: e.g.: gw-us-east (the tag that can be placed on services to be monitored by traefik|fabio)
- SERVICENAMEINGRESSGATEWAY: e.g.: gateway-us-east (the name of the service for the ingress gateway)
- INGRESSPORT: default= 9997 (the port that the envoy ingress gateway needs to expose in its listeners)
- INGRESSHEALTHCHECKPORT: default=8443 (the healthcheck port that the envoy ingress gateway needs to expose)
- INGRESSSERVICENAME: gateway-us-east-ingress (the name of the service that will be monitored by traefik|fabio)
- CONSULHTTPURL: default= http://127.0.0.1:8500
- CONSULTOKEN: (need write persmissions on SERVICENAMEINGRESSGATEWAY, write permissions on /agent/service/register, read permissions on /config and /config/service-defaults/serviceName)
