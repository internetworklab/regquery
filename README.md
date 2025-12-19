# regquery

## Introduction

regquery is a simple and efficient internet registry lookup service program, it returns the LPM match from the registry of the given input IP address string.

## Examples

Query for LPM match in `data/inetnum` and `data/inet6num` directory:

```shell
curl --url-query ip=172.20.143.15 https://dn42-query.netneighbor.me/query
curl --url-query ip=172.20.143.16 https://dn42-query.netneighbor.me/query
curl --url-query ip=172.20.143.17 https://dn42-query.netneighbor.me/query
curl --url-query ip=172.20.143.32 https://dn42-query.netneighbor.me/query
curl --url-query ip=172.20.143.33 https://dn42-query.netneighbor.me/query
curl --url-query ip=172.20.192.64 https://dn42-query.netneighbor.me/query
curl --url-query ip=fdda:8ca4:1556:: https://dn42-query.netneighbor.me/query
curl --url-query ip=fdda:8ca4:1556::a https://dn42-query.netneighbor.me/query
```

Query for IPInfoLite-like response (but much simplified):

```shell
curl --url-query ip=172.20.143.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
curl --url-query ip=172.20.142.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
curl --url-query ip=172.20.145.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
curl --url-query ip=172.20.149.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
```
