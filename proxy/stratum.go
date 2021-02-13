package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"../util"
)

// const (
// 	MaxReqSize = 1024
// )

func (s *ProxyServer) ListenTCP() {
	timeout := util.MustParseDuration(s.config.Proxy.Stratum.Timeout)
	s.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp", s.config.Proxy.Stratum.Listen)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer server.Close()

	log.Printf("Stratum listening on %s", s.config.Proxy.Stratum.Listen)
	var accept = make(chan int, s.config.Proxy.Stratum.MaxConn)
	n := 0

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		// log.Printf("AcceptTCP: %v", ip)

		// if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) {
		// 	conn.Close()
		// 	continue
		// }
		n += 1
		cs := &Session{conn: conn, ip: ip}

		accept <- n
		go func(cs *Session) {
			err = s.handleTCPClient(cs)
			if err != nil {
				s.removeSession(cs)
				conn.Close()
			}
			<-accept
		}(cs)
	}
}

func (s *ProxyServer) handleTCPClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connbuff := bufio.NewReaderSize(cs.conn, MaxReqSize)
	s.setDeadline(cs.conn)

	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Printf("Socket flood detected from %s", cs.ip)
			// s.policy.BanClient(cs.ip)
			return err
		} else if err == io.EOF {
			log.Printf("Client %s disconnected", cs.ip)
			s.removeSession(cs)
			break
		} else if err != nil {
			log.Printf("Error reading from socket: %v", err)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			var reqWithoutWorker JSONRpcReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				err = json.Unmarshal(data, &reqWithoutWorker)
				if err != nil {
					// s.policy.ApplyMalformedPolicy(cs.ip)
					log.Printf("Malformed stratum request from %s: %v", cs.ip, err)
					return err
				}
				req = StratumReq{JSONRpcReq: reqWithoutWorker, Worker: "0"}
			}
			s.setDeadline(cs.conn)
			err = cs.handleTCPMessage(s, &req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cs *Session) handleTCPMessage(s *ProxyServer, req *StratumReq) error {
	// Handle RPC methods
	switch req.Method {
	case "eth_submitLogin":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Printf("eth_submitLogin Malformed stratum request params from %s, %v", cs.ip, req.Params)
			return err
		}
		reply, errReply := s.handleLoginRPC(cs, params, req.Worker)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, reply)
	case "eth_getWork":
		reply, errReply := s.handleTCPGetWorkRPC(cs)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, &reply)
	case "eth_submitWork":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Printf("eth_submitWork Malformed stratum request params from %s, %v", cs.ip, req.Params)
			return err
		}
		reply, errReply := s.handleETHSubmitWorkRPC(cs, req.Worker, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, &reply)
	case "eth_submitHashrate":
		return cs.sendTCPResult(req.Id, true)

	case "mining.subscribe":
		var errReply *ErrorReply
		var reply interface{}

		switch cs.Protocol {
		case 2: // "EthereumStratum/2.0.0":
			var params = ""
			if len(req.Params) > 0 {
				err := json.Unmarshal(req.Params, &params)
				if err != nil {
					log.Printf("mining.subscribe Malformed stratum request params from %s, %v", cs.ip, req.Params)
					return err
				}
			}
			reply, errReply = s.handleTCPSubscribeV2RPC(cs, params, req.Worker)
		default:
			var params []string
			err := json.Unmarshal(req.Params, &params)
			if err != nil {
				log.Printf("mining.subscribe Malformed stratum request params from %s, %v", cs.ip, req.Params)
				return err
			}
			reply, errReply = s.handleTCPSubscribeRPC(cs, params, req.Worker)
		}

		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, reply)

	case "mining.extranonce.subscribe":
		return nil
	case "mining.authorize":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Printf("mining.authorize Malformed stratum request params from %s, %v", cs.ip, req.Params)
			return err
		}
		reply, errReply := s.handleLoginRPC(cs, params, req.Worker)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}

		t := s.currentBlockTemplate()
		if t == nil || len(t.Header) == 0 || s.isSick() {
			log.Printf("Current Block Template error")
			return errors.New("Current Block Template error")
		}

		switch cs.Protocol {
		case 2: // "EthereumStratum/2.0.0":
			cs.sendTCPResult(req.Id, "w-"+cs.exNonce)
			cs.pushMiningSet(fmt.Sprintf("%x", t.Height/30000), t.Target)
			// jobId, block id, headerhash, "0"
			currentJob := []interface{}{t.Header[2:10], fmt.Sprintf("%x", t.Height),
				t.Header[2:], "0"}
			return cs.pushNewJobV2(currentJob)
		case 1:
			cs.sendTCPResult(req.Id, reply)

			// set difficulty
			diff, _ := t.headers[t.Header].doubleDiff.Float64()
			cs.pushSetDifficulty(diff)

			currentJob := []interface{}{t.Header[2:10], t.Seed, t.Header, true}
			return cs.pushNewJobV1(currentJob)
		default:
			str := fmt.Sprintf("unsupported mining.authorize message with protocol: %d", cs.Protocol)
			log.Println(str)
			return errors.New(str)
		}

	case "mining.submit":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Println("mining.submit Malformed stratum request params from %s, %v", cs.ip, req.Params)
			return err
		}
		reply, errReply := s.handleTCPMiningSubmitRPC(cs, req.Worker, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, &reply)

	case "mining.hello":
		var params map[string]string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Printf("mining.hello Malformed stratum request params from %s, %v", cs.ip, params)
			return err
		}
		reply, errReply := s.handleTCPMiningHelloRPC(cs, req.Worker, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, &reply)
	case "mining.noop":
		return cs.sendTCPResult(req.Id, true)
	case "mining.hashrate":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Println("mining.hashrate Malformed stratum request params from %s, %v", cs.ip, req.Params)
			return err
		}
		_, errReply := s.handleTCPMiningHashrateRPC(cs, req.Worker, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, true)

	default:
		errReply := s.handleUnknownRPC(cs, req.Method)
		return cs.sendTCPError(req.Id, errReply)
	}
}

func (cs *Session) sendTCPResult(id json.RawMessage, result interface{}) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	// log.Printf("sendTCPResult message %v", message)
	return cs.enc.Encode(&message)
}

func (cs *Session) pushSetDifficulty(diff float64) error {
	cs.Lock()
	defer cs.Unlock()

	message := MiningNotifyMessage{Id: nil, Method: "mining.set_difficulty", Params: []float64{diff}}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushMiningSet(epoch string, target string) error {
	cs.Lock()
	defer cs.Unlock()

	params := make(map[string]string)
	params["epoch"] = epoch
	params["target"] = target[2:]
	params["algo"] = "ethash"
	params["extranonce"] = cs.exNonce

	message := MiningNotifyMessage{Id: nil, Method: "mining.set", Params: params}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJobV2(result interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	// message := JSONPushMessage{Version: "2.0", Result: result, Id: 0}
	message := MiningNotifyMessage{Params: result, Id: nil, Error: nil, Method: "mining.notify"}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJobV1(result interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	message := MiningNotifyMessage{Params: result, Id: nil, Error: nil, Method: "mining.notify"}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJob(result interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	message := JSONPushMessage{Version: "2.0", Result: result, Id: 0}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendTCPError(id json.RawMessage, reply *ErrorReply) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	err := cs.enc.Encode(&message)
	if err != nil {
		return err
	}
	return errors.New(reply.Message)
}

func (self *ProxyServer) setDeadline(conn *net.TCPConn) {
	conn.SetDeadline(time.Now().Add(self.timeout))
}

func (s *ProxyServer) registerSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[cs] = struct{}{}
}

func (s *ProxyServer) removeSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	// release extra nonce
	n, _ := strconv.Atoi(cs.exNonce)
	s.ExtraNonces[n] = 0
	log.Printf("remove session %v, %v@%v", cs.exNonce, cs.login, cs.ip)
	delete(s.sessions, cs)
}

func (s *ProxyServer) OnPoolNotify(resp []string) {
	// log.Println("recv: ", resp)
	// getwork array [headerhash, seedhash, target]
	s.fetchBlockTemplate(resp)
}

func (s *ProxyServer) broadcastNewJobs() {
	t := s.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || s.isSick() {
		return
	}
	// reply := []string{t.Header, t.Seed, s.diff}

	// eth-proxy
	// Eth: Received: {"id":0,"jsonrpc":"2.0","result":[
	// "0x92c27eb3ab118c0a3cef199f0c50fb8947b0ef7283954d2ad92c37c865bd2bc9",
	// "0x543b6279479743f05c14c59a68fb51f82e8cd46e9dd674900b4d8d0322dddf3d",
	// "0x0000000089705f4136b4a59731680a88f8953030fdd7645e011abac9f387295d"]}
	// jobhash, seedhash, target

	// https://github.com/nicehash/Specifications/blob/master/EthereumStratum_NiceHash_v1.0.0.txt
	// stratum v1
	// 	{
	//   "id": null,
	//   "method": "mining.notify",
	//   "params": [
	//     "bf0488aa",  //job ID
	//     "abad8f99f3918bf903c6a909d9bbc0fdfa5a2f4b9cb1196175ec825c6610126c",  // seedhash
	//     "645cf20198c2f3861e947d4f67e3ab63b7b2e24dcc9095bd9123e7b33371f6cc",	// headerhash
	//     true
	//   ]
	// }\n

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := len(s.sessions)
	// log.Printf("Broadcasting new job to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m, _ := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {
			var err error
			// todo change Protocol string to number
			switch cs.Protocol {
			case 2:
				replyV2 := []interface{}{t.Header[2:10], fmt.Sprintf("%x", t.Height),
					t.Header[2:], "0"}
				err = cs.pushNewJobV2(&replyV2)
			case 1:
				replyV1 := []interface{}{t.Header[2:10], t.Seed, t.Header, true}
				err = cs.pushNewJobV1(&replyV1)
			default:
				reply := []interface{}{t.Header, t.Seed, t.Target}
				err = cs.pushNewJob(&reply)
			}
			<-bcast
			if err != nil {
				log.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			} else {
				s.setDeadline(cs.conn)
			}
		}(m)
	}
	GRStat.TotalBroadcasts += int64(count)
	GRStat.TotalJobs++
	if count > 0 {
		GRStat.TotalBroadcastTime += time.Since(start).Microseconds()
	}

	// log.Printf("Jobs broadcast to %v miners finished %s", count, time.Since(start))
}
