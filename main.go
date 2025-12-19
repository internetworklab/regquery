package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/google/btree"
)

type Profile struct {
	data map[string][]string
}

func (p *Profile) Dump() map[string][]string {
	clone := make(map[string][]string)
	for k, v := range p.data {
		clone[k] = append([]string{}, v...)
	}
	return clone
}

func ParseProfile(path string) (*Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := new(Profile)
	result.data = make(map[string][]string)

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
	RegistryPath string `help:"Path to the registry." type:"path" default:"registry"`
}

func (s *ServeCmd) Run() error {
	log.Printf("Serving queries from %s", s.RegistryPath)

	inet6NumIndexer := new(INetNumIndexer)
	inet6NumIndexer.ResourceGroups = btree.New(2)

	err := inet6NumIndexer.Index(INetFamilyIPv6, filepath.Join(s.RegistryPath, "data", "inet6num"))
	if err != nil {
		return err
	}

	inetNumIndexer := new(INetNumIndexer)
	inetNumIndexer.ResourceGroups = btree.New(2)

	err = inetNumIndexer.Index(INetFamilyIPv4, filepath.Join(s.RegistryPath, "data", "inetnum"))
	if err != nil {
		return err
	}

	ipToQuery := "172.20.143.17"
	ip, family, err := parseIP(ipToQuery)
	if err != nil {
		return err
	}

	if family == INetFamilyIPv4 {
		inetres, err := inetNumIndexer.Query(ip, family)
		if err != nil {
			return err
		}
		if inetres == nil {
			log.Printf("No inetnum resource found for %s", ip)
			return nil
		}
		profile, err := ParseProfile(inetres.ProfilePath)
		if err != nil {
			return err
		}
		profileJ, err := json.Marshal(profile.Dump())
		if err != nil {
			return err
		}
		log.Printf("Found inetnum resource for %s: %s", ipToQuery, string(profileJ))
	}

	return err
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

	// indexed collection of INetNumResourceGroup
	ResourceGroups *btree.BTree
}

func (indexer *INetNumIndexer) Index(family INetFamily, dirPath string) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
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

		if !indexer.ResourceGroups.Has(&INetNumResourceGroup{PrefixLen: maskLen}) {
			indexer.ResourceGroups.ReplaceOrInsert(&INetNumResourceGroup{PrefixLen: maskLen, Resources: btree.New(2)})
		}

		group := indexer.ResourceGroups.Get(&INetNumResourceGroup{PrefixLen: maskLen})
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
	indexer.ResourceGroups.Descend(func(item btree.Item) bool {

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
