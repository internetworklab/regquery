package main

import (
	"encoding/json"
	"log"
	"os"

	pkgipgeodata "example.com/registryquery/pkg/tomlgeoip"
)

func main() {
	iniPath := "dn42-geoip/data/ipv4/172.20.159.0_28.toml"
	f, err := os.Open(iniPath)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	_, ipGeo, err := pkgipgeodata.IPGeoDataFromTOML(f)
	if err != nil {
		log.Panic(err)
	}

	json.NewEncoder(os.Stdout).Encode(ipGeo)
}
