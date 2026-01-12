package handler

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	pkgdn42 "example.com/registryquery/pkg/dn42"
	pkgiso3166 "example.com/registryquery/pkg/iso3166"
	pkgutils "example.com/registryquery/pkg/utils"
)

type IPInfoLikeResponse struct {
	IP            string  `json:"ip"`
	ASN           *string `json:"asn,omitempty"`
	ASName        *string `json:"as_name,omitempty"`
	ASDomain      *string `json:"as_domain,omitempty"`
	CountryCode   *string `json:"country_code,omitempty"`
	Country       *string `json:"country,omitempty"`
	ContinentCode *string `json:"continent_code,omitempty"`
	Continent     *string `json:"continent,omitempty"`
}

func (r *IPInfoLikeResponse) String() string {
	j, _ := json.Marshal(r)
	return string(j)
}

type IPInfoLiteHandler struct {
	DN42Indexer  *pkgdn42.DN42ResIndexer
	ISOAlpha2Map map[string][]pkgiso3166.ISOCountryCodeRecord
}

func (handler *IPInfoLiteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	isoAlpha2Map := handler.ISOAlpha2Map
	dn42Indexer := handler.DN42Indexer

	remoteAddr := pkgutils.GetRemoteAddr(r)
	log.Printf("Started to serve lite query for %s with raw query: %s", remoteAddr, r.URL.RawQuery)
	defer log.Printf("Served lite query for %s with raw query: %s", remoteAddr, r.URL.RawQuery)

	var err error = nil
	var ip net.IP = nil
	var family pkgutils.INetFamily = pkgutils.INetFamilyUnknown
	if ipToQuery := r.URL.Query().Get("ip"); ipToQuery != "" {
		ip, family, _, err = pkgutils.ParseIP(ipToQuery)
		if err != nil {
			encResp(r, w, ErrResponse{Err: err.Error()})
			return
		}

		inetres, routeRes, inetNumProfile, err := dn42Indexer.GetINetInfo(ip, family)
		if err != nil {
			encResp(r, w, ErrResponse{Err: err.Error()})
			return
		}

		if inetres == nil {
			encResp(r, w, ErrResponse{Err: "No inetnum resource found for " + ipToQuery})
			return
		}

		ipinfoResponse := &IPInfoLikeResponse{
			IP:          ipToQuery,
			CountryCode: inetNumProfile.GetFirst("country"),
		}
		if ipinfoResponse.CountryCode != nil && *ipinfoResponse.CountryCode != "" {
			records, ok := isoAlpha2Map[*ipinfoResponse.CountryCode]
			if ok && len(records) > 0 {
				ipinfoResponse.Country = records[0].Name
			}
		}

		asNumber, asName := dn42Indexer.GetAS(routeRes)
		ipinfoResponse.ASN = asNumber
		ipinfoResponse.ASName = asName

		encResp(r, w, ipinfoResponse)
		return
	}
}
