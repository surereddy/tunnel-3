package main

import (
	"bufio"
	"flag"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cosiner/gohper/utils/encodeio"
	"github.com/cosiner/tunnel/proxy"
	"github.com/cosiner/tunnel/server"
	"github.com/cosiner/ygo/log"
)

type Config struct {
	Socks []struct {
		Addr     string            `json:"addr"`
		UserPass map[string]string `json:"userPass"`
	} `json:"socks"`
	Tunnels []struct {
		Addr   string `json:"addr"`
		Method string `json:"method"`
		Key    string `json:"key"`
	} `json:"tunnels"`
}

var (
	conf      string
	runLocal  bool
	runRemote bool

	white string
	black string
)

func init() {
	flag.StringVar(&conf, "conf", "tunnel.json", "config file in json")
	flag.BoolVar(&runLocal, "local", false, "run as local server")
	flag.BoolVar(&runRemote, "remote", false, "run as remote server")
	flag.StringVar(&white, "white", "", "white site list doesn't using tunnel proxy")
	flag.StringVar(&black, "black", "", "black site list using tunnel proxy")
	flag.Parse()

	if (runLocal && runRemote) || (!runLocal && !runRemote) {
		log.Fatal("running mode is ambiguous.")
	}
	if white != "" && black != "" {
		log.Fatal("list mode is ambiguous.")
	}
}

func newSocks(cfg *Config) []proxy.Proxy {
	socks := make([]proxy.Proxy, len(cfg.Socks))
	for i, s := range cfg.Socks {
		methods := []byte{proxy.AUTH_NOT_REQUIRED}
		if len(s.UserPass) == 0 {
			methods = append(methods, proxy.AUTH_USER_PASS)
		}
		sock, err := proxy.NewSocks5(methods, proxy.NewUserPass(s.UserPass), s.Addr)
		if err != nil {
			log.Fatal("create local socks5 proxy failed", err)
		}
		socks[i] = sock
	}
	return socks
}

func newTunnels(cfg *Config) []proxy.Proxy {
	tunnels := make([]proxy.Proxy, len(cfg.Tunnels))
	for i, t := range cfg.Tunnels {
		tunnel, err := proxy.NewTunnel(t.Method, t.Key, t.Addr)
		if err != nil {
			log.Fatal("create local tunnel proxy failed", err)
		}
		tunnels[i] = tunnel
	}
	return tunnels
}

func newList(white, black string) (*server.SiteList, error) {
	var listFile string
	if white != "" {
		listFile = white
	} else if black != "" {
		listFile = black
	} else {
		return nil, nil
	}

	fd, err := os.Open(listFile)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	br := bufio.NewReader(fd)

	mode := server.LIST_WHITE
	if black != "" {
		mode = server.LIST_BLACK
	}
	list := server.NewList(mode)
	for {
		line, _, err := br.ReadLine()
		if len(line) > 0 {
			site := string(line)
			if !strings.HasPrefix(site, "//") {
				list.Add(site)
			}
		}

		if err == nil {
			continue
		}
		if err == io.EOF {
			err = nil
		}
		break
	}
	return list, err
}

func waitOsSignal() os.Signal {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return <-sigs
}

func main() {
	var cfg Config
	err := encodeio.ReadJSONWithComment(conf, &cfg)
	if err != nil {
		log.Fatal("parsing config file failed:", err)
	}
	if (runLocal && len(cfg.Socks) == 0) || len(cfg.Tunnels) == 0 {
		log.Fatal("empty socks or tunnels")
	}

	var (
		sig     server.Signal
		tunnels = newTunnels(&cfg)
	)
	if runLocal {
		list, err := newList(white, black)
		if err != nil {
			log.Fatal("parsing list file failed:", err)
		}

		socks := newSocks(&cfg)
		sig, err = server.RunMultipleLocal(socks, tunnels, list)
		if err != nil {
			log.Fatal("create local proxies:", err)
		}
		log.Infof("%d servers running.\n", len(socks))
	} else {
		sig, err = server.RunMultipleRemote(tunnels)
		if err != nil {
			log.Fatal("create remote proxies failed:", err)
		}
		log.Infof("%d servers running.\n", len(tunnels))
	}

	waitOsSignal()
	sig.Close()
}
