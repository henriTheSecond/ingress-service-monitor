package internal

import (
	"fmt"
	"strings"
)

type Tags struct {
}

func NewTags() *Tags {
	return new(Tags)
}
func (t *Tags) createGatewayTags(cService *ConsulService) []string {
	gwTags := []string{}
	for _, tag := range cService.Tags {
		if strings.HasPrefix(tag, TagIngressService) {
			gwTags = append(gwTags, t.createGatewayTag(tag))
		}
	}
	return gwTags
}
func (t *Tags) createGatewayTag(tag string) string {
	if TypeGateway == "traefik" {
		return fmt.Sprintf("%s", strings.Trim(strings.TrimPrefix(strings.ReplaceAll(tag, "'", "`"), TagIngressService), "/"))
	}
	return fmt.Sprintf("urlprefix-%s/", strings.Trim(strings.TrimPrefix(tag, TagIngressService), "/"))
}
func (t *Tags) getHostHeader(tags []string) string {
	for i := 0; i < len(tags); i++ {
		if TypeGateway == "traefik" {
			firstindex := strings.Index(tags[i], "`")
			if firstindex > -1 {
				lastindex := strings.LastIndex(tags[i], "`")
				if lastindex > -1 {
					return tags[i][firstindex+1 : lastindex]

				}
			}
			firstindex = strings.Index(tags[i], "'")
			if firstindex > -1 {
				lastindex := strings.LastIndex(tags[i], "'")
				if lastindex > -1 {
					return tags[i][firstindex+1 : lastindex]

				}
			}
		}
		if TypeGateway == "fabio" {
			if tags[i] != "" {
				return tags[i]
			}
		}
	}
	return ""
}
