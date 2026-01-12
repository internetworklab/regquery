package dn42

import (
	"bytes"
	"context"
	"io/fs"
	"log"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	pkgutils "example.com/registryquery/pkg/utils"
	"github.com/google/btree"
)

type INetNumResource struct {
	StartAddress net.IP
	Family       pkgutils.INetFamily
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
	Family pkgutils.INetFamily

	DirPath string

	// indexed collection of INetNumResourceGroup
	ResourceGroups *btree.BTree

	lock sync.Mutex
}

func NewINetNumIndexer(family pkgutils.INetFamily, dirPath string) *INetNumIndexer {
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

func doBuildIndex(family pkgutils.INetFamily, dirPath string) (*btree.BTree, error) {
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
		if family == pkgutils.INetFamilyIPv4 {
			startAddr = parsedIP.To4()
		} else if family == pkgutils.INetFamilyIPv6 {
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

func (indexer *INetNumIndexer) Query(address net.IP, family pkgutils.INetFamily) (*INetNumResource, error) {
	var result *INetNumResource = new(INetNumResource)
	var found *bool = new(bool)
	*found = false

	indexer.getIndexRef().Descend(func(item btree.Item) bool {
		resGroup, ok := item.(*INetNumResourceGroup)
		if !ok {
			panic("item is not an INetNumResourceGroup (impossible)")
		}

		var maskedIPAddress net.IP
		if family == pkgutils.INetFamilyIPv4 {
			maskedIPAddress = address.Mask(net.CIDRMask(resGroup.PrefixLen, 32))
		} else if family == pkgutils.INetFamilyIPv6 {
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
