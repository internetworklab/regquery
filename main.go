package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	pkgdn42 "example.com/registryquery/pkg/dn42"
	pkghandler "example.com/registryquery/pkg/handler"
	pkginigeoip "example.com/registryquery/pkg/inigeoip"
	pkgiso3166 "example.com/registryquery/pkg/iso3166"
	"github.com/alecthomas/kong"
)

type ServeCmd struct {
	RegistryPath              string `help:"Path to the registry." type:"path" default:"registry"`
	ISOCountryCodeDataPath    string `help:"Path to the ISO country code data." type:"path" default:"ISO-3166-Countries-with-Regional-Codes"`
	ListenAddress             string `help:"Address to listen on." type:"string" default:":18080"`
	AutoReIndexInterval       string `help:"Interval to auto re-index the index." type:"string" default:"12h"`
	GeoIPCacheRefreshIntvSecs int    `help:"Interval to refresh the GeoIP cache." type:"int" default:"86400"`
	INIGeoIPDBPath            string `help:"Path to the INI GeoIP database." type:"path" default:"dn42-geoip/data"`
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

	isoAlpha2Map, err := pkgiso3166.NewISO3166CountryCodeRecords(filepath.Join(s.ISOCountryCodeDataPath, "all", "all.json"))
	if err != nil {
		return err
	}

	dn42Indexer, err := pkgdn42.NewDN42ResIndexer(ctx, autoReIndexInterval, s.RegistryPath)
	if err != nil {
		return fmt.Errorf("failed to create DN42 res indexer: %v", err)
	}

	geoipCache := pkginigeoip.NewCachedGeoIPMapWrapper(time.Duration(s.GeoIPCacheRefreshIntvSecs)*time.Second, s.INIGeoIPDBPath)
	geoipCache.Run(ctx)

	serveMux := http.NewServeMux()

	serveMux.Handle("/ip2location/v1/query", &pkghandler.IP2LocationHandler{
		DN42Indexer: dn42Indexer,
		GeoIPCache:  geoipCache,
	})

	serveMux.Handle("/ipinfo/lite/query", &pkghandler.IPInfoLiteHandler{
		DN42Indexer:  dn42Indexer,
		ISOAlpha2Map: isoAlpha2Map,
	})

	serveMux.Handle("/query", &pkghandler.INetNumProfileHandler{
		DN42Indexer: dn42Indexer,
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
