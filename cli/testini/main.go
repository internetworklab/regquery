package main

import (
	"log"
	"path/filepath"

	pkginigeoip "example.com/registryquery/pkg/inigeoip"
)

func main() {
	root := filepath.Join("dn42-geoip", "data")
	geoipmap, err := pkginigeoip.IndexINIGeoIP(root)
	if err != nil {
		log.Fatalf("failed to index INI GeoIP: %v", err)
	}
	for cidr, ipinfo := range geoipmap {
		log.Printf("Found cidr: %s, ipinfo: %s", cidr, ipinfo.String())
	}
}
