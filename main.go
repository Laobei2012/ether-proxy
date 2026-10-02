package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc/jsonrpc"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ether-proxy/agent"
	"ether-proxy/proxy"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/yvasiyarov/gorelic"
)

var cfg proxy.Config

func startProxy() *proxy.ProxyServer {
	if cfg.Threads > 0 {
		runtime.GOMAXPROCS(cfg.Threads)
		log.Printf("Running with %v threads", cfg.Threads)
	} else {
		n := runtime.NumCPU()
		runtime.GOMAXPROCS(n)
		log.Printf("Running with default %v threads", n)
	}

	s := proxy.NewEndpoint(&cfg)

	go func() {
		startFrontend(&cfg, s)

		r := mux.NewRouter()
		r.Handle("/miner/{diff:.+}/{id:.+}", s)
		err := http.ListenAndServe(cfg.Proxy.Listen, r)
		if err != nil {
			log.Fatal(err)
		}
	}()
	return s
}

func startFrontend(cfg *proxy.Config, s *proxy.ProxyServer) {
	r := mux.NewRouter()
	r.HandleFunc("/stats", s.StatsIndex)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./www/")))
	var err error
	if len(cfg.Frontend.Password) > 0 {
		auth := httpauth.SimpleBasicAuth(cfg.Frontend.Login, cfg.Frontend.Password)
		err = http.ListenAndServe(cfg.Frontend.Listen, auth(r))
	} else {
		err = http.ListenAndServe(cfg.Frontend.Listen, r)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func startNewrelic() {
	if cfg.NewrelicEnabled {
		nr := gorelic.NewAgent()
		nr.Verbose = cfg.NewrelicVerbose
		nr.NewrelicLicense = cfg.NewrelicKey
		nr.NewrelicName = cfg.NewrelicName
		nr.Run()
	}
}

func readConfig(cfg *proxy.Config, configFileName string) {
	configFileName, _ = filepath.Abs(configFileName)
	log.Printf("Loading config: %v", configFileName)

	configFile, err := os.Open(configFileName)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&cfg); err != nil {
		log.Fatal("Config error: ", err.Error())
	}

	for i, v := range cfg.Upstream {
		u, err := url.Parse(v.Url)
		if err != nil {
			panic(err)
		}
		cfg.Upstream[i].Scheme = u.Scheme
		cfg.Upstream[i].User = u.User.Username()
		h := strings.Split(u.Host, ":")
		cfg.Upstream[i].Host = h[0]
		cfg.Upstream[i].Port = h[1]
		// fmt.Println(cfg.Upstream[i])
	}
}

var ctx = context.Background()

func startPool(server *proxy.ProxyServer) {
	pool := cfg.Upstream[0]
	maxNotifyInterval, _ := time.ParseDuration(cfg.UpstreamMaxNotifyInterval)

	endChan := make(chan int, 1)

	for {
		conn, err := net.DialTimeout("tcp", pool.Host+":"+pool.Port, 1000*1000*1000*30)
		if err != nil {
			log.Fatalf("connect to pool failed: %s", err)
			// os.Exit(-1)
		}
		log.Printf("connected %v:%v", pool.Host, pool.Port)

		// ch := channel.RawJSON(conn, conn)
		// cli := jrpc2.NewClient(ch, &jrpc2.ClientOptions{
		// 	OnNotify: func(req *jrpc2.Request) {
		// 		// notes = append(notes, req.Method())
		// 		log.Printf("OnNotify handler saw method %q", req.Method())
		// 	}}) // nil for default options

		// var params = [2]string{"sp_miner2020", "X"}
		// var replyBool bool
		// err = cli.CallResult(ctx, "eth_submitLogin", params, &replyBool)
		// if err != nil {
		// 	log.Fatal("rpc call error: ", err)
		// }
		// log.Printf("return: %v", replyBool)

		// // var replyArray []interface{}
		// rsp, err := cli.Call(ctx, "eth_getWork", nil)
		// if err != nil {
		// 	log.Fatal("rpc call error: ", err)
		// }
		// log.Printf("return: %v", rsp)
		// log.Printf("return: %v", replyArray)

		// for {
		// 	rsp.wait()
		// 	log.Printf("return: %v", rsp)
		// }

		clientRPC := jsonrpc.NewClient(conn)
		server.UpstreamTCP = clientRPC

		// var params = "sp_miner2020"
		var replyBool bool
		err = clientRPC.Call("eth_submitLogin", []string{pool.User}, &replyBool)
		if err != nil {
			log.Fatal("rpc call error: ", err)
		}
		log.Printf("login with: %v", pool.User)

		// var replyArray []interface{}
		// err = clientRPC.Call("eth_getWork", []string{""}, &replyArray)
		// if err != nil {
		// 	log.Fatal("rpc call error: ", err)
		// }
		// log.Printf("getwork return: %v", replyArray)
		/*/
		var clientInfo = [...]string{"ethminer-0.19.0", "EthereumStratum/1.0.0"}
		var subscribeReply []interface{}
		var replyBool bool

		err = clientRPC.Call("mining.subscribe", clientInfo, &subscribeReply)
		if err != nil {
			log.Fatal("rpc call error: ", err)
		}
		poolExtraNonce := subscribeReply[1]
		log.Printf("mining.subscribe return: %v, poolExtraNonce: %v", subscribeReply, poolExtraNonce)

		err = clientRPC.Call("mining.authorize", pool.User, &replyBool)
		if err != nil {
			log.Fatal("rpc call error: ", err)
		}
		log.Printf("mining.authorize return: %v", replyBool)
		/*/

		go func() {
			for {
				select {
				case rep := <-clientRPC.PushChan:
					server.OnPoolNotify(rep)
					// log.Println("recv: ", rep)
				case <-time.After(maxNotifyInterval):
					log.Println("recv timeout")
					endChan <- 1
					return
				}
			}
		}()
		_ = <-endChan
		clientRPC.Close()
		conn.Close()
	}
	// fmt.Printf("return : %v %T", reply[0].([]interface{})[0], reply[0])
}

func main() {
	var cliMode = flag.Int("mode", 0, "agent running mode")
	var cliConfigFile = flag.String("config", "config.json", "agent config file")
	var cliLogPath = flag.String("logPath", "./", "agent log file path")

	flag.Parse()

	f, err := os.OpenFile(*cliLogPath+"/agent-"+time.Now().Format("2006-01-02")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	// log.SetOutput(f)

	if *cliMode == 10 {
		// proxy mode
		readConfig(&cfg, *cliConfigFile)
		// startNewrelic()
		s := startProxy()
		startPool(s)

	} else {
		// agent mode
		readConfig(&cfg, *cliConfigFile)
		if cfg.Agent.Broker == "" || cfg.Agent.AccessKey == "" || cfg.Agent.SecretKey == "" {
			log.Fatal("agent mode requires an \"agent\" section (broker, accessKey, secretKey, ...) in ", *cliConfigFile)
		}
		agent.StartMQTTClient(cfg.Agent)

	}

}
