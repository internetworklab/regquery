package handler

import (
	"log"
	"net"
	"net/http"

	pkgdn42 "example.com/registryquery/pkg/dn42"
	pkginigeoip "example.com/registryquery/pkg/inigeoip"
	pkgutils "example.com/registryquery/pkg/utils"
)

type IP2LocationHandler struct {
	GeoIPCache  *pkginigeoip.CachedGeoIPMapWrapper
	DN42Indexer *pkgdn42.DN42ResIndexer
}

type IP2LocationLikeResponse struct {
	IP          *string  `json:"ip,omitempty"`
	CountryCode *string  `json:"country_code,omitempty"`
	CountryName *string  `json:"country_name,omitempty"`
	RegionName  *string  `json:"region_name,omitempty"`
	CityName    *string  `json:"city_name,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	ASN         *string  `json:"asn,omitempty"`
	AS          *string  `json:"as,omitempty"`
	IsProxy     *bool    `json:"is_proxy,omitempty"`
}

func (handler *IP2LocationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dn42Indexer := handler.DN42Indexer
	geoipCache := handler.GeoIPCache
	ctx := r.Context()

	remoteAddr := pkgutils.GetRemoteAddr(r)
	log.Printf("Started to serve lite query for %s with raw query: %s", remoteAddr, r.URL.RawQuery)
	defer log.Printf("Served lite query for %s with raw query: %s", remoteAddr, r.URL.RawQuery)

	var err error = nil
	var ip net.IP = nil
	var family pkgutils.INetFamily = pkgutils.INetFamilyUnknown
	var bits int = 0
	if ipToQuery := r.URL.Query().Get("ip"); ipToQuery != "" {
		ip, family, bits, err = pkgutils.ParseIP(ipToQuery)
		if err != nil {
			encResp(r, w, ErrResponse{Err: err.Error()})
			return
		}

		inetres, routeRes, _, err := dn42Indexer.GetINetInfo(ip, family)

		if err != nil {
			encResp(r, w, ErrResponse{Err: err.Error()})
			return
		}
		if inetres == nil {
			encResp(r, w, ErrResponse{Err: "No inetnum resource found for " + ipToQuery})
			return
		}

		geoipMap, err := geoipCache.GetGeoIPMap(ctx)
		if err != nil {
			encResp(r, w, ErrResponse{Err: err.Error()})
			return
		}

		ip2LocResp := &IP2LocationLikeResponse{
			IP: &ipToQuery,
		}

		if geoipMap != nil {
			if route := geoipMap.Get(net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}); route != nil {
				if geoipRec, ok := route.Value.(*pkginigeoip.BasicGeoIP); ok {
					ip2LocResp.CountryCode = geoipRec.CountryCode
					ip2LocResp.CountryName = geoipRec.Country
					ip2LocResp.RegionName = geoipRec.Region
					ip2LocResp.CityName = geoipRec.City
					ip2LocResp.Latitude = geoipRec.Latitude
					ip2LocResp.Longitude = geoipRec.Longitude
					f := false
					ip2LocResp.IsProxy = &f
				}
			}
		}

		ip2LocResp.ASN, ip2LocResp.AS = dn42Indexer.GetAS(routeRes)

		encResp(r, w, ip2LocResp)
		return
	}
}
