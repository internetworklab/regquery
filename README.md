# regquery

## Introduction

regquery is a simple and efficient internet registry lookup service program, it returns the LPM match from the registry of the given input IP address string.

## Examples

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
