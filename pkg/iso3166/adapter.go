package iso3166

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
