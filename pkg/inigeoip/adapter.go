package inigeoip

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkgtomlgeoip "example.com/registryquery/pkg/tomlgeoip"
	pkgutils "example.com/registryquery/pkg/utils"
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

func BasicGeoIPFromMaster(master pkgtomlgeoip.IPGeoDataMaster) (*BasicGeoIP, error) {
	result := &BasicGeoIP{}
	if country := master.Country; country != nil {
		result.Country = country.Name
		result.CountryCode = country.Code
	}

	return result, nil
}

func BasicGeoIPFromEntry(record pkgtomlgeoip.IPGeoDataEntry) (*BasicGeoIP, error) {
	result := &BasicGeoIP{}
	if country := record.Country; country != nil {
		result.Country = country.Name
		result.CountryCode = country.Code
	}

	if region := record.Region; region != nil {
		if regionName := region.Name; regionName != nil {
			result.Region = regionName
		} else if regionCode := region.Code; regionCode != nil {
			result.Region = regionCode
		}
	}
	result.City = result.Region
	if city := record.City; city != nil && *city != "" {
		result.City = city
	}

	if lat := record.Latitude; lat != nil {
		result.Latitude = lat
	}
	if lon := record.Longitude; lon != nil {
		result.Longitude = lon
	}

	return result, nil
}

// Returning a map, where key is CIDR (as returned by `net.IPNet.String()`), value is `*BasicGeoIP`
func WalkTOMLFile(tomlPath string) (map[string]*BasicGeoIP, error) {
	f, err := os.Open(tomlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open TOML file: %w", err)
	}
	defer f.Close()

	_, ipGeoData, err := pkgtomlgeoip.IPGeoDataFromTOML(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	result := make(map[string]*BasicGeoIP)
	if masterRecord := ipGeoData.Master; masterRecord != nil {
		if cidr := masterRecord.CIDR; cidr != nil && *cidr != "" {
			if v, err := BasicGeoIPFromMaster(*masterRecord); err == nil {
				result[*cidr] = v
			}
		}
	}

	if geoipRecords := ipGeoData.GeoData; geoipRecords != nil {
		for _, geoipRecord := range geoipRecords {
			if cidr := geoipRecord.CIDR; cidr != nil && *cidr != "" {
				if v, err := BasicGeoIPFromEntry(geoipRecord); err == nil {
					result[*cidr] = v
				}
			}
		}
	}

	return result, nil
}

func IndexTOMLGeoIP(root string) (map[string]*BasicGeoIP, error) {
	type Collector struct {
		geoipMap map[string]*BasicGeoIP
	}
	collector := new(Collector)
	collector.geoipMap = make(map[string]*BasicGeoIP)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory: %v", err)
		}
		if d.IsDir() {
			// log.Printf("skipping directory: %s", path)
			return nil
		}
		if !strings.HasSuffix(path, ".toml") {
			// log.Printf("skipping file: %s", path)
			return nil
		}
		geoipmap, err := WalkTOMLFile(path)
		if err != nil {
			log.Printf("failed to walk INI file: %v", err)
			return nil
		}
		for cidr, ipinfo := range geoipmap {
			collector.geoipMap[cidr] = ipinfo
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %v", err)
	}
	return collector.geoipMap, nil
}

type CacheAccess struct {
	GeoIPMap chan *pkgutils.RouteTable
}

type CachedGeoIPMapWrapper struct {
	Expiry    time.Duration
	Root      string
	serviceCh chan chan *CacheAccess
}

func NewCachedGeoIPMapWrapper(expiry time.Duration, root string) *CachedGeoIPMapWrapper {
	cache := new(CachedGeoIPMapWrapper)
	cache.Expiry = expiry
	cache.Root = root
	cache.serviceCh = make(chan chan *CacheAccess)
	return cache
}

func (cache *CachedGeoIPMapWrapper) refreshCache() (*pkgutils.RouteTable, time.Time, error) {
	geoipmap, err := IndexTOMLGeoIP(cache.Root)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to index INI GeoIP: %v", err)
	}
	routeTable, err := newRouteTableFromINIGeoIPMap(geoipmap)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to create route table from INI GeoIP map: %v", err)
	}

	log.Printf("Refreshed GeoIP map cache with %d entries", len(geoipmap))
	return routeTable, time.Now().Add(cache.Expiry), nil
}

func (cache *CachedGeoIPMapWrapper) Run(ctx context.Context) {
	go func() {
		defer close(cache.serviceCh)

		geoipmap, expire, err := cache.refreshCache()
		if err != nil {
			panic(err)
		}

		for {
			serviceSubCh := make(chan *CacheAccess)
			select {
			case <-ctx.Done():
				return
			case cache.serviceCh <- serviceSubCh:
				serviceAccess := <-serviceSubCh
				if expire.Before(time.Now()) {
					log.Printf("Refreshing GeoIP map cache due to expiry")
					geoipmap, expire, err = cache.refreshCache()
					if err != nil {
						panic(err)
					}
				}
				serviceAccess.GeoIPMap <- geoipmap
			}
		}

	}()
}

func (cache *CachedGeoIPMapWrapper) GetGeoIPMap(ctx context.Context) (*pkgutils.RouteTable, error) {
	serviceSubCh, ok := <-cache.serviceCh
	if !ok {
		return nil, fmt.Errorf("service channel closed")
	}

	serviceAccess := new(CacheAccess)
	serviceAccess.GeoIPMap = make(chan *pkgutils.RouteTable)
	serviceSubCh <- serviceAccess

	return <-serviceAccess.GeoIPMap, nil
}

func newRouteTableFromINIGeoIPMap(geoIPMap map[string]*BasicGeoIP) (*pkgutils.RouteTable, error) {
	anyMap := make(map[string]interface{})
	for cidrStr, entry := range geoIPMap {
		anyMap[cidrStr] = entry
	}
	routeTable, err := pkgutils.NewRouteTableFromMap(anyMap, true)
	if err != nil {
		log.Printf("failed to create route table from INI GeoIP map: %v", err)
		return nil, fmt.Errorf("failed to create route table from INI GeoIP map: %v", err)
	}
	return routeTable, nil
}
