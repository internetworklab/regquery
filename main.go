package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/btree"
	"gopkg.in/yaml.v3"
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
	RegistryPath           string `help:"Path to the registry." type:"path" default:"registry"`
	ISOCountryCodeDataPath string `help:"Path to the ISO country code data." type:"path" default:"ISO-3166-Countries-with-Regional-Codes"`
	ListenAddress          string `help:"Address to listen on." type:"string" default:":18080"`
	AutoReIndexInterval    string `help:"Interval to auto re-index the index." type:"string" default:"12h"`
}

type ErrResponse struct {
	Err string `json:"err"`
}

type Formattable interface {
	String() string
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

func (s *ServeCmd) Run() error {
	ctx := context.Background()

	log.Printf("Serving queries from %s", s.RegistryPath)

	var autoReIndexInterval time.Duration
	var err error = nil
	autoReIndexInterval, err = time.ParseDuration(s.AutoReIndexInterval)
	if err != nil {
		return err
	}

	var isoCountryCodeRecords []ISOCountryCodeRecord = nil
	isoCountryDataF, err := os.Open(filepath.Join(s.ISOCountryCodeDataPath, "all", "all.json"))
	if err != nil {
		return err
	}
	if err := json.NewDecoder(isoCountryDataF).Decode(&isoCountryCodeRecords); err != nil {
		return err
	}

	isoAlpha2Map := make(map[string][]ISOCountryCodeRecord)
	for _, record := range isoCountryCodeRecords {
		key := record.Alpha2
		if key != nil && *key != "" {
			isoAlpha2Map[*key] = append(isoAlpha2Map[*key], record)
		}
	}

	inet6NumIndexer := NewINetNumIndexer(
		INetFamilyIPv6,
		filepath.Join(s.RegistryPath, "data", "inet6num"),
	)
	if err := inet6NumIndexer.Index(ctx); err != nil {
		return err
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

	inetNumIndexer := NewINetNumIndexer(
		INetFamilyIPv4,
		filepath.Join(s.RegistryPath, "data", "inetnum"),
	)

	err = inetNumIndexer.Index(ctx)
	if err != nil {
		return err
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
	routeIndexer := NewINetNumIndexer(
		INetFamilyIPv4,
		filepath.Join(s.RegistryPath, "data", "route"),
	)

	if err := routeIndexer.Index(ctx); err != nil {
		return err
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
	route6Indexer := NewINetNumIndexer(
		INetFamilyIPv6,
		filepath.Join(s.RegistryPath, "data", "route6"),
	)

	if err := route6Indexer.Index(ctx); err != nil {
		return err
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

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/ipinfo/lite/query", func(w http.ResponseWriter, r *http.Request) {
		var err error = nil
		var ip net.IP = nil
		var family INetFamily = INetFamilyUnknown
		if ipToQuery := r.URL.Query().Get("ip"); ipToQuery != "" {
			ip, family, err = parseIP(ipToQuery)
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}

			var inetres *INetNumResource = nil
			var routeRes *INetNumResource = nil
			if family == INetFamilyIPv4 {
				inetres, err = inetNumIndexer.Query(ip, family)
				if err == nil {
					routeRes, err = routeIndexer.Query(ip, family)
				}
			} else if family == INetFamilyIPv6 {
				inetres, err = inet6NumIndexer.Query(ip, family)
				if err == nil {
					routeRes, err = route6Indexer.Query(ip, family)
				}
			}
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}
			if inetres == nil {
				encResp(r, w, ErrResponse{Err: "No inetnum resource found for " + ipToQuery})
				return
			}
			inetNumProfile, err := ParseProfile(inetres.ProfilePath)
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
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

			if routeRes != nil {
				routeProfile, err := ParseProfile(routeRes.ProfilePath)
				if err == nil && routeProfile != nil {
					asn := routeProfile.GetFirst("origin")
					if asn != nil && *asn != "" {
						ipinfoResponse.ASN = asn

						asnPattern := regexp.MustCompile(`^AS(\d+)$`)
						if ok := asnPattern.MatchString(*asn); ok {
							asnProfile, err := ParseProfile(filepath.Join(s.RegistryPath, "data", "aut-num", *asn))
							if err == nil && asnProfile != nil {
								asnName := asnProfile.GetFirst("as-name")
								if asnName != nil && *asnName != "" {
									*asnName = strings.TrimPrefix(*asnName, "AS-")
									ipinfoResponse.ASName = asnName
								}
							}
						}
					}
				}
			}
			encResp(r, w, ipinfoResponse)
			return
		}
	})

	serveMux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		var err error = nil
		var ip net.IP = nil
		var family INetFamily = INetFamilyUnknown

		if ipToQuery := r.URL.Query().Get("ip"); ipToQuery != "" {
			ip, family, err = parseIP(ipToQuery)
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}

			var inetres *INetNumResource = nil
			if family == INetFamilyIPv4 {
				inetres, err = inetNumIndexer.Query(ip, family)
			} else if family == INetFamilyIPv6 {
				inetres, err = inet6NumIndexer.Query(ip, family)
			}
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}
			if inetres == nil {
				encResp(r, w, ErrResponse{Err: "No inetnum resource found for " + ipToQuery})
				return
			}
			profile, err := ParseProfile(inetres.ProfilePath)
			if err != nil {
				encResp(r, w, ErrResponse{Err: err.Error()})
				return
			}
			encResp(r, w, profile)
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

type INetFamily int

const (
	INetFamilyUnknown INetFamily = iota
	INetFamilyIPv4    INetFamily = 4
	INetFamilyIPv6    INetFamily = 6
)

type INetNumResource struct {
	StartAddress net.IP
	Family       INetFamily
	Prefix       string
	PrefixLen    int
	ProfilePath  string
}

func (res *INetNumResource) Less(other btree.Item) bool {
	if other, ok := other.(*INetNumResource); ok {
		return bytes.Compare(res.StartAddress, other.StartAddress) < 0
	}
	panic("other is not an INetNumResource (impossible)")
}

func (resGroup *INetNumResourceGroup) Less(other btree.Item) bool {
	if other, ok := other.(*INetNumResourceGroup); ok {
		return resGroup.PrefixLen < other.PrefixLen
	}
	panic("other is not an INetNumResourceGroup (impossible)")
}

type INetNumResourceGroup struct {
	PrefixLen int

	// indexed collection of INetNumResource
	Resources *btree.BTree
}

type INetNumIndexer struct {
	Family INetFamily

	DirPath string

	// indexed collection of INetNumResourceGroup
	ResourceGroups *btree.BTree

	lock sync.Mutex
}

func NewINetNumIndexer(family INetFamily, dirPath string) *INetNumIndexer {
	indexer := &INetNumIndexer{
		Family:         family,
		DirPath:        dirPath,
		ResourceGroups: btree.New(2),
		lock:           sync.Mutex{},
	}

	return indexer
}

func (indexer *INetNumIndexer) setIndex(index *btree.BTree) {
	indexer.lock.Lock()
	defer indexer.lock.Unlock()
	indexer.ResourceGroups = index
}

func (indexer *INetNumIndexer) getIndexRef() *btree.BTree {
	indexer.lock.Lock()
	defer indexer.lock.Unlock()
	return indexer.ResourceGroups
}

func doBuildIndex(family INetFamily, dirPath string) (*btree.BTree, error) {
	index := btree.New(2)
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error walking %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}

		pattern := regexp.MustCompile(`_(\d+)$`)
		matches := pattern.FindStringSubmatch(d.Name())
		if matches == nil {
			return nil
		}

		if len(matches) < 2 {
			return nil
		}

		maskLenStr := matches[1]
		maskLen, err := strconv.Atoi(maskLenStr)
		if err != nil {
			log.Printf("Error converting %s to int when walking %s: %v", maskLenStr, path, err)
			return nil
		}
		prefix := d.Name()[:len(d.Name())-len(matches[0])]

		if !index.Has(&INetNumResourceGroup{PrefixLen: maskLen}) {
			index.ReplaceOrInsert(&INetNumResourceGroup{PrefixLen: maskLen, Resources: btree.New(2)})
		}

		group := index.Get(&INetNumResourceGroup{PrefixLen: maskLen})
		if group == nil {
			panic("group is nil (impossible)")
		}

		var startAddr net.IP
		parsedIP := net.ParseIP(prefix)
		if family == INetFamilyIPv4 {
			startAddr = parsedIP.To4()
		} else if family == INetFamilyIPv6 {
			startAddr = parsedIP.To16()
		} else {
			panic("family is not valid (impossible)")
		}
		inetRes := &INetNumResource{StartAddress: startAddr, Family: family, Prefix: prefix, PrefixLen: maskLen, ProfilePath: path}
		group.(*INetNumResourceGroup).Resources.ReplaceOrInsert(inetRes)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return index, err
}

func (indexer *INetNumIndexer) Index(ctx context.Context) error {
	newIndex, err := doBuildIndex(indexer.Family, indexer.DirPath)
	if err != nil {
		return err
	}

	indexer.setIndex(newIndex)
	return nil
}

func (indexer *INetNumIndexer) AutoReIndex(ctx context.Context, every time.Duration) <-chan error {
	errChan := make(chan error)
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := indexer.Index(ctx); err != nil {
					errChan <- err
				} else {
					errChan <- nil
				}
			}
		}
	}()
	return errChan
}

func parseIP(address string) (ip net.IP, family INetFamily, err error) {
	ip = net.ParseIP(address)
	if ip4 := ip.To4(); ip4 != nil {
		family = INetFamilyIPv4
		ip = ip4
	} else if ip6 := ip.To16(); ip6 != nil {
		family = INetFamilyIPv6
		ip = ip6
	} else {
		return nil, INetFamilyUnknown, fmt.Errorf("%s is not an ip address of known family", address)
	}
	return ip, family, nil
}

func (indexer *INetNumIndexer) Query(address net.IP, family INetFamily) (*INetNumResource, error) {
	var result *INetNumResource = new(INetNumResource)
	var found *bool = new(bool)
	*found = false

	indexer.getIndexRef().Descend(func(item btree.Item) bool {
		resGroup, ok := item.(*INetNumResourceGroup)
		if !ok {
			panic("item is not an INetNumResourceGroup (impossible)")
		}

		var maskedIPAddress net.IP
		if family == INetFamilyIPv4 {
			maskedIPAddress = address.Mask(net.CIDRMask(resGroup.PrefixLen, 32))
		} else if family == INetFamilyIPv6 {
			maskedIPAddress = address.Mask(net.CIDRMask(resGroup.PrefixLen, 128))
		} else {
			panic("family is not valid (impossible)")
		}

		inetresItem := resGroup.Resources.Get(&INetNumResource{StartAddress: maskedIPAddress})
		if inetres, ok := inetresItem.(*INetNumResource); ok {
			*result = *inetres
			*found = true
			return false
		}

		return true
	})

	if *found {
		return result, nil
	}
	return nil, nil
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
