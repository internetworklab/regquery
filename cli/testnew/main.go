package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/ini.v1"
)

type BasicGeoIP struct {
	Country     *string
	Region      *string
	City        *string
	Latitude    *float64
	Longitude   *float64
	CountryCode *string
}

func (b *BasicGeoIP) String() string {
	segs := make([]string, 0)
	if b.Country != nil && *b.Country != "" {
		segs = append(segs, fmt.Sprintf("country=%s", *b.Country))
	}
	if b.Region != nil && *b.Region != "" {
		segs = append(segs, fmt.Sprintf("region=%s", *b.Region))
	}
	if b.City != nil && *b.City != "" {
		segs = append(segs, fmt.Sprintf("city=%s", *b.City))
	}
	if b.Latitude != nil {
		segs = append(segs, fmt.Sprintf("latitude=%f", *b.Latitude))
	}
	if b.Longitude != nil {
		segs = append(segs, fmt.Sprintf("longitude=%f", *b.Longitude))
	}
	return strings.Join(segs, ", ")
}

func BasicGeoIPFromINISection(section *ini.Section) *BasicGeoIP {
	if section == nil {
		return nil
	}

	var latitude *float64
	var longitude *float64
	var country *string
	var region *string
	var city *string
	var countryCode *string

	if x, err := section.Key("latitude").Float64(); err == nil {
		latitude = new(float64)
		*latitude = x
	}

	if x, err := section.Key("longitude").Float64(); err == nil {
		longitude = new(float64)
		*longitude = x
	}

	if x := section.Key("country").String(); x != "" {
		country = new(string)
		*country = x
	}

	if x := section.Key("region").String(); x != "" {
		region = new(string)
		*region = x
	}

	if x := section.Key("city").String(); x != "" {
		city = new(string)
		*city = x
	}

	if x := section.Key("country_code").String(); x != "" {
		countryCode = new(string)
		*countryCode = x
	}

	return &BasicGeoIP{
		Country:     country,
		Region:      region,
		City:        city,
		Latitude:    latitude,
		Longitude:   longitude,
		CountryCode: countryCode,
	}
}

type IPGeoDataVersion struct {
	DataVersion       *string    `toml:"data_version,omitempty"`
	CreationTimestamp *time.Time `toml:"create_time,omitempty"`
	UpdateTimestamp   *time.Time `toml:"update_time,omitempty"`
}

type IPGeoData struct {
	Version *IPGeoDataVersion `toml:"Version"`
}

func main() {
	iniPath := "dn42-geoip/data/ipv4/172.20.159.0_28.toml"

	var config IPGeoData
	_, err := toml.DecodeFile(iniPath, &config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("object.Version:\n")
	json.NewEncoder(os.Stdout).Encode(config.Version)
}
