package proxy

import (
	"log"
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
	log.Printf("Stratum miner connected %v@%v", login, cs.ip)
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
	hash, _ := s.Jobs.Get("0x" + params[1])
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

func (s *ProxyServer) handleTCPSubscribeRPC(cs *Session, params []string, id string) ([]interface{}, *ErrorReply) {
	// todo verify params
	if len(params) == 0 {
		return nil, &ErrorReply{Code: -1, Message: "Invalid params"}
	}
	clientVersion := params[0]
	stratumVersion := params[1]

	extraNonce := "dc39"
	cs.exNonce = extraNonce
	resultArray := []string{"mining.notify", extraNonce, stratumVersion}
	result := []interface{}{resultArray, extraNonce}

	log.Printf("Stratum miner subscribe %v %v @%v", clientVersion, stratumVersion, cs.ip)
	return result, nil
}
