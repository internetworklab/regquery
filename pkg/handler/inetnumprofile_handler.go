package handler

import (
	"log"
	"net"
	"net/http"

	pkgdn42 "example.com/registryquery/pkg/dn42"
	pkgutils "example.com/registryquery/pkg/utils"
)

type INetNumProfileHandler struct {
	DN42Indexer *pkgdn42.DN42ResIndexer
}

func (handler *INetNumProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dn42Indexer := handler.DN42Indexer
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
}
