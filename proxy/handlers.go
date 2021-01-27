package proxy

import (
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"../httprpc"
	"../util"
	"github.com/ethereum/go-ethereum/common"
)

func (s *ProxyServer) handleGetWorkRPC(cs *Session, diff, id string) (reply []string, errorReply *ErrorReply) {
	t := s.currentBlockTemplate()
	if len(t.Header) == 0 {
		return nil, &ErrorReply{Code: -1, Message: "Work not ready"}
	}
	targetHex := t.Target

	if !s.rpc().Pool {
		minerDifficulty, err := strconv.ParseFloat(diff, 64)
		if err != nil {
			log.Printf("Invalid difficulty %v from %v@%v ", diff, id, cs.ip)
			minerDifficulty = 5
		}
		targetHex = util.MakeTargetHex(minerDifficulty)
	}
	reply = []string{t.Header, t.Seed, targetHex}
	return
}

func (s *ProxyServer) handleSubmitRPC(cs *Session, diff string, id string, params []string) (reply bool, errorReply *ErrorReply) {
	miner, ok := s.miners.Get(id)
	if !ok {
		miner = NewMiner(id, cs.ip)
		s.registerMiner(miner)
	}

	t := s.currentBlockTemplate()
	reply = miner.processShare(s, t, diff, params)
	return
}

func (s *ProxyServer) handleSubmitHashrate(cs *Session, req *JSONRpcReq) bool {
	reply, _ := s.rpc().SubmitHashrate(req.Params)
	return reply
}

// func (s *ProxyServer) handleUnknownRPC(cs *Session, req *JSONRpcReq) *ErrorReply {
// 	log.Printf("Unknown RPC method: %v", req)
// 	return &ErrorReply{Code: -1, Message: "Invalid method"}
// }

// Allow only lowercase hexadecimal with 0x prefix
var noncePattern = regexp.MustCompile("^0x[0-9a-f]{16}$")
var hashPattern = regexp.MustCompile("^0x[0-9a-f]{64}$")
var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_]{1,8}$")

// Stratum
func (s *ProxyServer) handleLoginRPC(cs *Session, params []string, id string) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	login := strings.ToLower(params[0])
	// if !util.IsValidHexAddress(login) {
	// 	return false, &ErrorReply{Code: -1, Message: "Invalid login"}
	// }
	// if !s.policy.ApplyLoginPolicy(login, cs.ip) {
	// 	return false, &ErrorReply{Code: -1, Message: "You are blacklisted"}
	// }
	cs.login = login
	s.registerSession(cs)
	log.Printf("Stratum miner login %v@%v", login, cs.ip)
	return true, nil
}

func (s *ProxyServer) handleTCPGetWorkRPC(cs *Session) ([]string, *ErrorReply) {
	t := s.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || s.isSick() {
		return nil, &ErrorReply{Code: 0, Message: "Work not ready"}
	}
	return []string{t.Header, t.Seed, s.diff}, nil
}

// Stratum
func (s *ProxyServer) handleETHSubmitWorkRPC(cs *Session, id string, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return s.handleSubmitRPC(cs, cs.login, id, params)
}

func (s *ProxyServer) handleTCPMiningSubmitRPC(cs *Session, id string, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return s.handleMiningSubmitRPC(cs, cs.login, id, params)
}

func (s *ProxyServer) handleTCPMiningHashrateRPC(cs *Session, id string, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}

	var replyBool bool
	t := s.currentBlockTemplate()
	// todo: second param should be a rand string
	rate := []string{params[0], t.Header}
	err := s.UpstreamTCP.Call("eth_submitHashrate", rate, &replyBool)
	if err != nil {
		log.Fatal("rpc call error: ", err)
	}
	log.Printf("submit Hashrate: %v, return: %v", rate, replyBool)

	return replyBool, nil
}

func (s *ProxyServer) handleTCPMiningHelloRPC(cs *Session, id string, params map[string]string) (map[string]string, *ErrorReply) {
	var result = make(map[string]string)
	result["proto"] = "EthereumStratum/2.0.0"
	result["encoding"] = "plain"
	result["resume"] = "0"
	result["timeout"] = "b4"
	result["maxerrors"] = "5"
	result["node"] = "Geth/v1.8.18-unstable-f08f596a/linux-amd64/go1.10.4"
	cs.Protocol = "EthereumStratum/2.0.0"
	return result, nil
}

func (s *ProxyServer) handleMiningSubmitRPC(cs *Session, login, id string, params []string) (bool, *ErrorReply) {
	start := time.Now()

	if !workerPattern.MatchString(id) {
		id = "0"
	}
	if len(params) != 3 {
		// s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed params from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	// if !noncePattern.MatchString(params[0]) || !hashPattern.MatchString(params[1]) || !hashPattern.MatchString(params[2]) {
	// 	// s.policy.ApplyMalformedPolicy(cs.ip)
	// 	log.Printf("Malformed PoW result from %s@%s %v", login, cs.ip, params)
	// 	return false, &ErrorReply{Code: -1, Message: "Malformed PoW result"}
	// }
	// t := s.currentBlockTemplate()
	// exist, validShare := s.processShare(login, id, cs.ip, t, params)
	// // ok := s.policy.ApplySharePolicy(cs.ip, !exist && validShare)

	// if exist {
	// 	log.Printf("Duplicate share from %s@%s %v", login, cs.ip, params)
	// 	return false, &ErrorReply{Code: 22, Message: "Duplicate share"}
	// }

	// if !validShare {
	// 	log.Printf("Invalid share from %s@%s", login, cs.ip)
	// 	// Bad shares limit reached, return error and close
	// 	if !ok {
	// 		return false, &ErrorReply{Code: 23, Message: "Invalid share"}
	// 	}
	// 	return false, nil
	// }
	// log.Printf("Valid share from %s@%s", login, cs.ip)

	// if !ok {
	// 	return true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
	// }

	// mining.submit : [username, job ID, minernonce]
	// compose eth_submitwork message and send to pool
	nonce := "0x" + cs.exNonce + params[2]
	hashI, _ := s.Jobs.Get("0x" + params[1])
	hash := hashI.(string)
	var mixDigest *common.Hash

	t := s.currentBlockTemplate()
	h, _ := t.headers[hash]
	value, _ := strconv.ParseUint(cs.exNonce+params[2], 16, 64)

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hash),
		difficulty:  h.diff,
		nonce:       uint64(value),
		mixDigest:   common.HexToHash(hash),
	}

	mixDigest = s.Hasher.MakeMixDigest(share)
	// log.Printf("MakeMixDigest time: %s %v %v", time.Since(start), share, value)
	if mixDigest == nil {
		log.Fatal("generate mix error!")
	}

	work := []interface{}{nonce, hash, mixDigest}
	var replyBool bool
	err := s.UpstreamTCP.Call("eth_submitWork", work, &replyBool)
	if err != nil {
		log.Fatal("rpc call error: ", err)
	}
	log.Printf("submit work: %v, return: %v, time: %s", work, replyBool, time.Since(start))
	GRStat.TotalShares++
	GRStat.TotalShareSubmitTime += time.Since(start).Microseconds()

	return replyBool, nil
}

func (s *ProxyServer) handleGetBlockByNumberRPC() *httprpc.GetBlockReplyPart {
	t := s.currentBlockTemplate()
	var reply *httprpc.GetBlockReplyPart
	if t != nil {
		reply = t.GetPendingBlockCache
	}
	return reply
}

func (s *ProxyServer) handleUnknownRPC(cs *Session, m string) *ErrorReply {
	log.Printf("Unknown request method %s from %s", m, cs.ip)
	// s.policy.ApplyMalformedPolicy(cs.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}

func (s *ProxyServer) generateExNonce() string {
	var n int64
	for {
		min := int64(0)
		max := int64(ExtraNonceSize)
		n = rand.Int63n(max - min)
		if s.ExtraNonces[n] == 0 {
			break
		}
	}
	// s.ExtraNonces need to clear when session close
	s.ExtraNonces[n] = 1
	return fmt.Sprintf("%04x", n)
}

func (s *ProxyServer) handleTCPSubscribeRPC(cs *Session, params []string, id string) ([]interface{}, *ErrorReply) {
	if len(params) == 0 {
		return nil, &ErrorReply{Code: -1, Message: "Invalid params"}
	}
	clientVersion := params[0]
	stratumVersion := params[1]
	if stratumVersion != "EthereumStratum/1.0.0" {
		return nil, &ErrorReply{Code: -1, Message: "unsupport stratum version."}
	}

	cs.exNonce = s.generateExNonce()
	resultArray := []string{"mining.notify", cs.exNonce, stratumVersion}
	result := []interface{}{resultArray, cs.exNonce}

	log.Printf("Stratum miner subscribe %v %v %v %v", clientVersion, stratumVersion, cs.ip, cs.exNonce)
	return result, nil
}

func (s *ProxyServer) handleTCPSubscribeV2RPC(cs *Session, params string, id string) (string, *ErrorReply) {
	cs.exNonce = s.generateExNonce()
	log.Printf("Stratum V2 miner subscribe %v %v", cs.ip, cs.exNonce)
	return "s-" + cs.exNonce, nil
}
