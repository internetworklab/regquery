# regquery

## Introduction

regquery is a simple and efficient internet registry lookup service program, it returns the LPM match from the registry of the given input IP address string.

## Examples

To return IP2Location-like GeoIP response:

```
curl -s -o - --url-query ip=172.20.143.17 https://regquery.ping2.sh/ip2location/v1/query | jq
{
  "ip": "172.20.143.17",
  "country_code": "US",
  "country_name": "United States",
  "region_name": "California",
  "city_name": "Los Angeles",
  "latitude": 34.05223,
  "longitude": -118.24368,
  "asn": "AS4242421771",
  "as": "DUSTSTARS-DN42",
  "is_proxy": false
}
```

Query for LPM match in `data/inetnum` and `data/inet6num` directory:

```shell
curl --url-query ip=172.20.143.15 https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=172.20.143.16 https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=172.20.143.17 https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=172.20.143.32 https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=172.20.143.33 https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=172.20.192.64 https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=fdda:8ca4:1556:: https://dn42-query.netneighbor.me/query | jq
curl --url-query ip=fdda:8ca4:1556::a https://dn42-query.netneighbor.me/query | jq
```

Query for IPInfoLite-like response (but much simplified):

```shell
curl --url-query ip=172.20.143.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
curl --url-query ip=172.20.142.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
curl --url-query ip=172.20.145.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
curl --url-query ip=172.20.149.1 https://dn42-query.netneighbor.me/ipinfo/lite/query | jq
```
