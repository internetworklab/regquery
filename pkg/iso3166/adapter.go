package iso3166

import (
	"encoding/json"
	"fmt"
	"os"
)

type ISOCountryCodeRecord struct {
	Name                   *string `json:"name,omitempty"`
	Alpha2                 *string `json:"alpha-2,omitempty"`
	Alpha3                 *string `json:"alpha-3,omitempty"`
	CountryCode            *string `json:"country-code,omitempty"`
	ISO31662               *string `json:"iso_3166-2,omitempty"`
	Region                 *string `json:"region,omitempty"`
	SubRegion              *string `json:"sub-region,omitempty"`
	IntermediateRegion     *string `json:"intermediate-region,omitempty"`
	RegionCode             *string `json:"region-code,omitempty"`
	SubRegionCode          *string `json:"sub-region-code,omitempty"`
	IntermediateRegionCode *string `json:"intermediate-region-code,omitempty"`
}

func NewISO3166CountryCodeRecords(registryPath string) (map[string][]ISOCountryCodeRecord, error) {

	var isoCountryCodeRecords []ISOCountryCodeRecord = nil
	isoCountryDataF, err := os.Open(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ISO country code data file: %v", err)
	}
	if err := json.NewDecoder(isoCountryDataF).Decode(&isoCountryCodeRecords); err != nil {
		return nil, fmt.Errorf("failed to decode ISO country code data file: %v", err)
	}

	isoAlpha2Map := make(map[string][]ISOCountryCodeRecord)
	for _, record := range isoCountryCodeRecords {
		key := record.Alpha2
		if key != nil && *key != "" {
			isoAlpha2Map[*key] = append(isoAlpha2Map[*key], record)
		}
	}
	return isoAlpha2Map, nil
}
