package utils

import "regexp"

func ExtractRoute(requestRouteKey string) string {
	r := regexp.MustCompile(`(?P<method>) (?P<pathKey>.*)`)
	routeKeyParts := r.FindStringSubmatch(requestRouteKey)
	return routeKeyParts[r.SubexpIndex("pathKey")]
}

func ExtractParam(routeKey string) string {
	r := regexp.MustCompile(`/integrations/(?P<integrationId>.*)`)
	tokenParts := r.FindStringSubmatch(routeKey)
	return tokenParts[r.SubexpIndex("integrationId")]
}
