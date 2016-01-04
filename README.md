# Tunnel

tunnel is a proxy tool inspired by shadowsocks that support multiple socks and
tunnels.

# Install
```sh
$ go install github.com/cosiner/tunnel
```

```sh
$ tunnel --help
Usage of ./tunnel:
  -black string
        black site list using tunnel proxy
  -conf string
        config file in json (default "tunnel.json")
  -local
        run as local server
  -remote
        run as remote server
  -white string
        white site list doesn't using tunnel proxy
```
