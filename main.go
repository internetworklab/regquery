package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	pkgdn42 "example.com/registryquery/pkg/dn42"
	pkginigeoip "example.com/registryquery/pkg/inigeoip"
	pkgiso3166 "example.com/registryquery/pkg/iso3166"
	pkgutils "example.com/registryquery/pkg/utils"
	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v3"
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

func (r *IPInfoLikeResponse) String() string {
	j, _ := json.Marshal(r)
	return string(j)
}

type Profile struct {
	data          map[string][]string
	originContent []byte
}

func (p *Profile) String() string {
	return string(p.originContent)
}

func (p *Profile) MarshalYAML() (interface{}, error) {
	stringsBuf := bytes.NewBufferString("")
	dec := yaml.NewEncoder(stringsBuf)
	dec.SetIndent(2)
	dec.Encode(p.Dump())
	contentB, err := io.ReadAll(stringsBuf)
	if err != nil {
		return nil, err
	}
	return string(contentB), nil
}

func (p *Profile) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Dump())
}

func (p *Profile) Dump() map[string][]string {
	clone := make(map[string][]string)
	for k, v := range p.data {
		clone[k] = append([]string{}, v...)
	}
	return clone
}

func (p *Profile) GetFirst(key string) *string {
	if p != nil && p.data != nil {
		if vals, ok := p.data[key]; ok {
			if len(vals) > 0 {
				val := vals[0]
				return &val
			}
		}
	}
	return nil
}

func ParseProfile(path string) (*Profile, error) {
	result := new(Profile)
	result.data = make(map[string][]string)

	var err error = nil
	result.originContent, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		pattern := regexp.MustCompile(`^([\w\d-_]+):\s*(.+)`)
		matches := pattern.FindStringSubmatch(line)
		if len(matches) >= 3 {
			group1 := matches[1]
			group2 := matches[2]

			if _, ok := result.data[group1]; !ok {
				result.data[group1] = make([]string, 0)
			}
			result.data[group1] = append(result.data[group1], group2)
		}
	}

	return result, nil
}

type ServeCmd struct {
	RegistryPath              string `help:"Path to the registry." type:"path" default:"registry"`
	ISOCountryCodeDataPath    string `help:"Path to the ISO country code data." type:"path" default:"ISO-3166-Countries-with-Regional-Codes"`
	ListenAddress             string `help:"Address to listen on." type:"string" default:":18080"`
	AutoReIndexInterval       string `help:"Interval to auto re-index the index." type:"string" default:"12h"`
	GeoIPCacheRefreshIntvSecs int    `help:"Interval to refresh the GeoIP cache." type:"int" default:"86400"`
	INIGeoIPDBPath            string `help:"Path to the INI GeoIP database." type:"path" default:"dn42-geoip/data"`
}

type ErrResponse struct {
	Err string `json:"err"`
}

type Formattable interface {
	String() string
}

// returns (AS Number, AS name)
func getAS(routeRes *pkgdn42.INetNumResource, registryPath string) (*string, *string) {
	var asnPtr *string
	var asNamePtr *string

	if routeRes != nil {
		routeProfile, err := ParseProfile(routeRes.ProfilePath)
		if err == nil && routeProfile != nil {
			asn := routeProfile.GetFirst("origin")
			if asn != nil && *asn != "" {
				asnPtr = new(string)
				*asnPtr = *asn

				asnPattern := regexp.MustCompile(`^AS(\d+)$`)
				if ok := asnPattern.MatchString(*asn); ok {
					asnProfile, err := ParseProfile(filepath.Join(registryPath, "data", "aut-num", *asn))
					if err == nil && asnProfile != nil {
						asnName := asnProfile.GetFirst("as-name")
						if asnName != nil && *asnName != "" {
							*asnName = strings.TrimPrefix(*asnName, "AS-")
							asNamePtr = new(string)
							*asNamePtr = *asnName
						}
					}
				}
			}
		}
	}

	return asnPtr, asNamePtr
}

func encResp(req *http.Request, w http.ResponseWriter, resp interface{}) error {
	accept := req.Header.Get("Accept")
	if accept == "" {
		accept = req.URL.Query().Get("accept")
	}
	if accept == "" {
		accept = "application/json"
	}

	if strings.HasPrefix(accept, "application/json") {
		return json.NewEncoder(w).Encode(resp)
	}
	if strings.HasPrefix(accept, "text/plain") {
		if s, ok := resp.(string); ok {
			fmt.Fprintf(w, "%s", s)
			return nil
		}
		if f, ok := resp.(Formattable); ok {
			fmt.Fprintf(w, "%s", f.String())
			return nil
		}
		return json.NewEncoder(w).Encode(resp)
	}
	if strings.HasPrefix(accept, "application/yaml") {
		return yaml.NewEncoder(w).Encode(resp)
	}
	return json.NewEncoder(w).Encode(resp)
}

func getISO3166CountryCodeRecords(registryPath string) (map[string][]pkgiso3166.ISOCountryCodeRecord, error) {
	var isoCountryCodeRecords []pkgiso3166.ISOCountryCodeRecord = nil
	isoCountryDataF, err := os.Open(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ISO country code data file: %v", err)
	}
	if err := json.NewDecoder(isoCountryDataF).Decode(&isoCountryCodeRecords); err != nil {
		return nil, fmt.Errorf("failed to decode ISO country code data file: %v", err)
	}

	isoAlpha2Map := make(map[string][]pkgiso3166.ISOCountryCodeRecord)
	for _, record := range isoCountryCodeRecords {
		key := record.Alpha2
		if key != nil && *key != "" {
			isoAlpha2Map[*key] = append(isoAlpha2Map[*key], record)
		}
	}
	return isoAlpha2Map, nil
}

type DN42ResIndexer struct {
	INetNumIndexer  *pkgdn42.INetNumIndexer
	INet6NumIndexer *pkgdn42.INetNumIndexer
	RouteIndexer    *pkgdn42.INetNumIndexer
	Route6Indexer   *pkgdn42.INetNumIndexer
}

// returns (inetnumRes, routeRes)
func (dn42Indexer *DN42ResIndexer) GetINetInfo(ip net.IP, family pkgutils.INetFamily) (*pkgdn42.INetNumResource, *pkgdn42.INetNumResource, *Profile, error) {

	if dn42Indexer == nil {
		return nil, nil, nil, fmt.Errorf("dn42 indexer is nil")
	}

	var err error = nil
	var inetres *pkgdn42.INetNumResource = nil
	var routeRes *pkgdn42.INetNumResource = nil
	var inetnumProfile *Profile = nil
	if family == pkgutils.INetFamilyIPv4 {
		inetres, err = dn42Indexer.INetNumIndexer.Query(ip, family)
		if err == nil {
			routeRes, err = dn42Indexer.RouteIndexer.Query(ip, family)
		}
	} else if family == pkgutils.INetFamilyIPv6 {
		inetres, err = dn42Indexer.INet6NumIndexer.Query(ip, family)
		if err == nil {
			routeRes, err = dn42Indexer.Route6Indexer.Query(ip, family)
		}
	}

	if inetres != nil {
		inetnumProfile, err = ParseProfile(inetres.ProfilePath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse inetnum profile")
		}
	}

	return inetres, routeRes, inetnumProfile, err
}

func NewDN42ResIndexer(ctx context.Context, autoReIndexInterval time.Duration, registryPath string) (*DN42ResIndexer, error) {
	dn42Indexer := new(DN42ResIndexer)

	inet6NumIndexer := pkgdn42.NewINetNumIndexer(
		pkgutils.INetFamilyIPv6,
		filepath.Join(registryPath, "data", "inet6num"),
	)
	if err := inet6NumIndexer.Index(ctx); err != nil {
		return nil, fmt.Errorf("failed to index inet6num: %v", err)
	}
	go func() {
		for err := range inet6NumIndexer.AutoReIndex(ctx, autoReIndexInterval) {
			if err != nil {
				log.Printf("Error auto re-indexing inet6num index: %v", err)
			} else {
				log.Printf("Auto re-indexed inet6num index")
			}
		}
	}()

	inetNumIndexer := pkgdn42.NewINetNumIndexer(
		pkgutils.INetFamilyIPv4,
		filepath.Join(registryPath, "data", "inetnum"),
	)

	err := inetNumIndexer.Index(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to index inetnum: %v", err)
	}
	go func() {
		for err := range inetNumIndexer.AutoReIndex(ctx, autoReIndexInterval) {
			if err != nil {
				log.Printf("Error auto re-indexing inetnum index: %v", err)
			} else {
				log.Printf("Auto re-indexed inetnum index")
			}
		}
	}()

	// for ROA4
	routeIndexer := pkgdn42.NewINetNumIndexer(
		pkgutils.INetFamilyIPv4,
		filepath.Join(registryPath, "data", "route"),
	)

	if err := routeIndexer.Index(ctx); err != nil {
		return nil, fmt.Errorf("failed to index route: %v", err)
	}
	go func() {
		for err := range routeIndexer.AutoReIndex(ctx, autoReIndexInterval) {
			if err != nil {
				log.Printf("Error auto re-indexing route index: %v", err)
			} else {
				log.Printf("Auto re-indexed route index")
			}
		}
	}()

	// for ROA6
	route6Indexer := pkgdn42.NewINetNumIndexer(
		pkgutils.INetFamilyIPv6,
		filepath.Join(registryPath, "data", "route6"),
	)

	if err := route6Indexer.Index(ctx); err != nil {
		return nil, fmt.Errorf("failed to index route6: %v", err)
	}
	go func() {
		for err := range route6Indexer.AutoReIndex(ctx, autoReIndexInterval) {
			if err != nil {
				log.Printf("Error auto re-indexing route index: %v", err)
			} else {
				log.Printf("Auto re-indexed route6 index")
			}
		}
	}()

	dn42Indexer.INetNumIndexer = inetNumIndexer
	dn42Indexer.INet6NumIndexer = inet6NumIndexer
	dn42Indexer.RouteIndexer = routeIndexer
	dn42Indexer.Route6Indexer = route6Indexer
	return dn42Indexer, nil
}

func (s *ServeCmd) Run() error {
	ctx := context.Background()

	log.Printf("Serving queries from %s", s.RegistryPath)

	var autoReIndexInterval time.Duration
	var err error = nil
	autoReIndexInterval, err = time.ParseDuration(s.AutoReIndexInterval)
	if err != nil {
		return err
	}

	isoAlpha2Map, err := getISO3166CountryCodeRecords(filepath.Join(s.ISOCountryCodeDataPath, "all", "all.json"))
	if err != nil {
		return err
	}

	dn42Indexer, err := NewDN42ResIndexer(ctx, autoReIndexInterval, s.RegistryPath)
	if err != nil {
		return fmt.Errorf("failed to create DN42 res indexer: %v", err)
	}

	geoipCache := pkginigeoip.NewCachedGeoIPMapWrapper(time.Duration(s.GeoIPCacheRefreshIntvSecs)*time.Second, s.INIGeoIPDBPath)
	geoipCache.Run(ctx)

	serveMux := http.NewServeMux()

	serveMux.HandleFunc("/ip2location/v1/query", func(w http.ResponseWriter, r *http.Request) {

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

			ip2LocResp.ASN, ip2LocResp.AS = getAS(routeRes, s.RegistryPath)

			encResp(r, w, ip2LocResp)
			return
		}
	})

	serveMux.HandleFunc("/ipinfo/lite/query", func(w http.ResponseWriter, r *http.Request) {
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

			asNumber, asName := getAS(routeRes, s.RegistryPath)
			ipinfoResponse.ASN = asNumber
			ipinfoResponse.ASName = asName

			encResp(r, w, ipinfoResponse)
			return
		}
	})

	serveMux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := pkgutils.GetRemoteAddr(r)
		log.Printf("Started to serve query for %s with raw query: %s", remoteAddr, r.URL.RawQuery)
		defer log.Printf("Served query for %s with raw query: %s", remoteAddr, r.URL.RawQuery)

		var err error = nil
		var ip net.IP = nil
		var family pkgutils.INetFamily = pkgutils.INetFamilyUnknown

		if ipToQuery := r.URL.Query().Get("ip"); ipToQuery != "" {
			ip, family, _, err = pkgutils.ParseIP(ipToQuery)
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}

			inetres, _, inetnumProfile, err := dn42Indexer.GetINetInfo(ip, family)
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}

			if inetres == nil {
				encResp(r, w, ErrResponse{Err: "No inetnum resource found for " + ipToQuery})
				return
			}
			if inetnumProfile == nil {
				encResp(r, w, ErrResponse{Err: "No inetnum profile found for " + ipToQuery})
				return
			}
			encResp(r, w, inetnumProfile)
			return
		}
	})

	server := &http.Server{
		Handler: serveMux,
	}
	listener, err := net.Listen("tcp", s.ListenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Printf("Serving queries on %s", s.ListenAddress)

	return server.Serve(listener)
}

var CLI struct {
	Serve ServeCmd `cmd:"" help:"Serve queries."`
}

func main() {
	ctx := kong.Parse(&CLI)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
