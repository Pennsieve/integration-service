package utils

import (
	"fmt"
	"regexp"
	"time"
)

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

func ConvertEpochToUTCLocation(epoch int64) string {
	t := time.Unix(int64(epoch/1000), 0).UTC()
	return fmt.Sprint(t)
}

func ConvertEpochToUTCRFC3339(epoch int64) string {
	t := time.Unix(int64(epoch/1000), 0).UTC()
	return t.UTC().Format(time.RFC3339)
}
