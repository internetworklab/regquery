package tomlgeoip

import (
	"io"
	"time"

	"github.com/BurntSushi/toml"
)

type IPGeoDataVersion struct {
	DataVersion       *string    `toml:"data_version,omitempty"`
	CreationTimestamp *time.Time `toml:"create_time,omitempty"`
	UpdateTimestamp   *time.Time `toml:"update_time,omitempty"`
}

type IPGeoDataCountry struct {
	Name *string `toml:"name,omitempty"`
	Code *string `toml:"code,omitempty"`
}

type IPGeoDataRegion struct {
	Name *string `toml:"name,omitempty"`
	Code *string `toml:"code,omitempty"`
}

type IPGeoDataMaster struct {
	CIDR    *string           `toml:"CIDR,omitempty"`
	Country *IPGeoDataCountry `toml:"country,omitempty"`
	Source  *string           `toml:"source,omitempty"`
}

type IPGeoDataEntry struct {
	CIDR           *string           `toml:"CIDR,omitempty"`
	Anycast        *bool             `toml:"anycast,omitempty"`
	Country        *IPGeoDataCountry `toml:"country,omitempty"`
	Region         *IPGeoDataRegion  `toml:"region,omitempty"`
	City           *string           `toml:"city,omitempty"`
	Latitude       *float64          `toml:"latitude,omitempty"`
	Longitude      *float64          `toml:"longitude,omitempty"`
	AccuracyRadius *int              `toml:"accuracy_radius,omitempty"`
}

type IPGeoData struct {
	Version *IPGeoDataVersion `toml:"Version"`
	Master  *IPGeoDataMaster  `toml:"Master,omitempty"`
	GeoData []IPGeoDataEntry  `toml:"GeoData,omitempty"`
}

func IPGeoDataFromTOML(r io.Reader) (*toml.MetaData, *IPGeoData, error) {
	var config IPGeoData
	decoder := toml.NewDecoder(r)
	meta, err := decoder.Decode(&config)
	if err != nil {
		return nil, nil, err
	}
	return &meta, &config, nil
}
