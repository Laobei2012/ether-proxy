package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/ethash"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"

	"../httprpc"
	"../util"
)

type ProxyServer struct {
	config          *Config
	miners          MinersMap
	blockTemplate   atomic.Value
	upstream        int32
	upstreams       []*httprpc.RPCClient
	hashrateWindow  time.Duration
	timeout         time.Duration
	roundShares     int64
	blocksMu        sync.RWMutex
	blockStats      map[int64]float64
	luckWindow      int64
	luckLargeWindow int64

	diff       string
	failsCount int64

	sessionsMu sync.RWMutex
	sessions   map[*Session]struct{}

	Jobs        util.Cache_fifo // save jobs hash for calculate mixDigest
	UpstreamTCP *rpc.Client
	Hasher      *ethash.Ethash
	SeedHashs   map[common.Hash]uint64
	ExtraNonces [ExtraNonceSize]int8
}

type Session struct {
	enc *json.Encoder
	ip  string

	// Stratum
	sync.Mutex
	conn    *net.TCPConn
	login   string
	exNonce string

	Protocol string
	HashRate int64
}

type RuningStat struct {
	TotalBroadcasts      int64
	TotalBroadcastTime   int64
	TotalShares          int64
	TotalShareSubmitTime int64
	TotalJobs            int64
}

var (
	GRStat       RuningStat
	GRStatByMin  util.Cache_fifo
	GRStatByHour util.Cache_fifo
	GRStatByDay  util.Cache_fifo
)

const (
	MaxReqSize     = 1 * 1024
	ExtraNonceSize = 65536
)

func NewEndpoint(cfg *Config) *ProxyServer {
	GRStatByMin.Init(100)
	GRStatByHour.Init(100)
	GRStatByDay.Init(100)

	proxy := &ProxyServer{
		config: cfg, blockStats: make(map[int64]float64), SeedHashs: make(map[common.Hash]uint64),
	}
	proxy.Jobs.Init(100)
	proxy.Hasher = ethash.New()
	proxy.miners = NewMinersMap()

	// todo expend seed hash length
	for i := uint64(385); i <= 400; i++ {
		seedhash := proxy.Hasher.MakeSeedHash(i)
		proxy.SeedHashs[seedhash] = i
	}

	timeout, _ := time.ParseDuration(cfg.Proxy.ClientTimeout)
	proxy.timeout = timeout

	hashrateWindow, _ := time.ParseDuration(cfg.Proxy.HashrateWindow)
	proxy.hashrateWindow = hashrateWindow

	luckWindow, _ := time.ParseDuration(cfg.Proxy.LuckWindow)
	proxy.luckWindow = int64(luckWindow / time.Millisecond)
	luckLargeWindow, _ := time.ParseDuration(cfg.Proxy.LargeLuckWindow)
	proxy.luckLargeWindow = int64(luckLargeWindow / time.Millisecond)

	if cfg.Proxy.Stratum.Enabled {
		proxy.sessions = make(map[*Session]struct{})
		go proxy.ListenTCP()
	}

	proxy.blockTemplate.Store(&BlockTemplate{})

	refreshIntv, _ := time.ParseDuration(cfg.Proxy.BlockRefreshInterval)
	refreshTimer := time.NewTimer(refreshIntv)
	log.Printf("Set block refresh every %v", refreshIntv)

	checkIntv, _ := time.ParseDuration(cfg.UpstreamCheckInterval)
	checkTimer := time.NewTimer(checkIntv)

	runingTimerMin := time.NewTimer(time.Minute)

	switch cfg.UpstreamProto {
	case "eth-proxy":
		// use channel to comm
		log.Printf("eth-proxy Upstream")

	case "http":
		proxy.upstreams = make([]*httprpc.RPCClient, len(cfg.Upstream))
		for i, v := range cfg.Upstream {
			client, err := httprpc.NewRPCClient(v.Name, v.Url, v.Timeout, v.Pool)
			if err != nil {
				log.Fatal(err)
			} else {
				proxy.upstreams[i] = client
				log.Printf("http Upstream: %s => %s", v.Name, v.Url)
			}
		}
		log.Printf("Default upstream: %s => %s", proxy.rpc().Name, proxy.rpc().Url)
		proxy.fetchBlockTemplate(nil)

		go func() {
			for {
				select {
				case <-refreshTimer.C:
					proxy.fetchBlockTemplate(nil)
					refreshTimer.Reset(refreshIntv)
				}
			}
		}()
	}

	go func() {
		for {
			select {
			case <-checkTimer.C:
				proxy.checkUpstreams()
				checkTimer.Reset(checkIntv)
			case <-runingTimerMin.C:
				currentMin := time.Now().Unix() / 60
				lastStatI, err := GRStatByMin.Get(currentMin - 1)
				avgCast, avgSubmit, totalAvgCast, totalAvgSubmit := int64(0), int64(0), int64(0), int64(0)
				if GRStat.TotalBroadcasts > 0 {
					totalAvgCast = GRStat.TotalBroadcastTime / GRStat.TotalBroadcasts
				}
				if GRStat.TotalShares > 0 {
					totalAvgSubmit = GRStat.TotalShareSubmitTime / GRStat.TotalShares / 1000
				}

				if err == nil {
					lastStat := lastStatI.(RuningStat)
					if GRStat.TotalBroadcasts-lastStat.TotalBroadcasts > 0 {
						avgCast = (GRStat.TotalBroadcastTime - lastStat.TotalBroadcastTime) /
							(GRStat.TotalBroadcasts - lastStat.TotalBroadcasts)
					}
					if GRStat.TotalShares-lastStat.TotalShares > 0 {
						avgSubmit = (GRStat.TotalShareSubmitTime - lastStat.TotalShareSubmitTime) /
							(GRStat.TotalShares - lastStat.TotalShares) / 1000
					}
					log.Printf("Runing info: clients:%v, cast:%v, avgCast(us):%v(%v), jobs:%v(%v), shares:%v(%v), avgSubmit(ms):%v(%v)",
						len(proxy.sessions),
						GRStat.TotalBroadcasts-lastStat.TotalBroadcasts,
						avgCast, totalAvgCast,
						GRStat.TotalJobs-lastStat.TotalJobs, GRStat.TotalJobs,
						GRStat.TotalShares-lastStat.TotalShares, GRStat.TotalShares,
						avgSubmit, totalAvgSubmit,
					)
				} else {
					log.Printf("Runing info: clients:%v, cast:%v, avgCast(us):%v, jobs:%v, shares:%v, avgSubmit(ms):%v",
						len(proxy.sessions), GRStat.TotalBroadcasts,
						totalAvgCast,
						GRStat.TotalJobs,
						GRStat.TotalShares, totalAvgSubmit,
					)
				}
				GRStatByMin.Add(currentMin, GRStat)
				runingTimerMin.Reset(time.Minute)
			}
		}
	}()

	return proxy
}

func (s *ProxyServer) rpc() *httprpc.RPCClient {
	i := atomic.LoadInt32(&s.upstream)
	return s.upstreams[i]
}

func (s *ProxyServer) checkUpstreams() {
	candidate := int32(0)
	backup := false

	for i, v := range s.upstreams {
		ok, err := v.Check()
		if err != nil {
			log.Printf("Upstream %v didn't pass check: %v", v.Name, err)
		}
		if ok && !backup {
			candidate = int32(i)
			backup = true
		}
	}

	if s.upstream != candidate {
		log.Printf("Switching to %v upstream", s.upstreams[candidate].Name)
		atomic.StoreInt32(&s.upstream, candidate)
	}
}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, 405, "rpc: POST method required, received "+r.Method)
		return
	}
	s.handleClient(w, r)
}

func (s *ProxyServer) handleClient(w http.ResponseWriter, r *http.Request) error {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	cs := &Session{ip: ip, enc: json.NewEncoder(w)}
	defer r.Body.Close()
	connbuff := bufio.NewReaderSize(r.Body, MaxReqSize)

	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Printf("Socket flood detected")
			return errors.New("Socket flood")
		} else if err == io.EOF {
			break
		}

		if len(data) > 1 {
			var req JSONRpcReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				log.Printf("Malformed request: %v", err)
				return err
			}
			cs.handleMessage(s, r, &req)
		}
	}
	return nil
}

func (cs *Session) handleMessage(s *ProxyServer, r *http.Request, req *JSONRpcReq) {
	if req.Id == nil {
		log.Println("Missing RPC id")
		r.Close = true
		return
	}

	vars := mux.Vars(r)

	// Handle RPC methods
	switch req.Method {
	case "eth_getWork":
		reply, errReply := s.handleGetWorkRPC(cs, vars["diff"], vars["id"])
		if errReply != nil {
			cs.sendError(req.Id, errReply)
			break
		}
		cs.sendResult(req.Id, &reply)
	case "eth_submitWork":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Println("Unable to parse params")
			break
		}
		reply, errReply := s.handleSubmitRPC(cs, vars["diff"], vars["id"], params)
		if errReply != nil {
			err = cs.sendError(req.Id, errReply)
			break
		}
		cs.sendResult(req.Id, &reply)
	case "eth_submitHashrate":
		reply := true
		if s.config.Proxy.SubmitHashrate {
			reply = s.handleSubmitHashrate(cs, req)
		}
		cs.sendResult(req.Id, reply)
	default:
		errReply := s.handleUnknownRPC(cs, req.Method)
		cs.sendError(req.Id, errReply)
	}
}

func (cs *Session) sendResult(id json.RawMessage, result interface{}) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendError(id json.RawMessage, reply *ErrorReply) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	return cs.enc.Encode(&message)
}

func (s *ProxyServer) writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
}

func (s *ProxyServer) currentBlockTemplate() *BlockTemplate {
	t := s.blockTemplate.Load()
	if t != nil {
		return t.(*BlockTemplate)
	} else {
		return nil
	}
}

func (s *ProxyServer) registerMiner(miner *Miner) {
	s.miners.Set(miner.Id, miner)
}

func (s *ProxyServer) markSick() {
	atomic.AddInt64(&s.failsCount, 1)
}

func (s *ProxyServer) isSick() bool {
	x := atomic.LoadInt64(&s.failsCount)
	if s.config.Proxy.HealthCheck && x >= s.config.Proxy.MaxFails {
		return true
	}
	return false
}

func (s *ProxyServer) markOk() {
	atomic.StoreInt64(&s.failsCount, 0)
}
