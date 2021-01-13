package proxy

import "encoding/json"

type JSONRpcReq struct {
	Id     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type StratumReq struct {
	JSONRpcReq
	Worker string `json:"worker"`
}

// Stratum
type MiningNotifyMessage struct {
	// FIXME: Temporarily add ID for Claymore compliance
	Id     interface{} `json:"id"`
	Error  interface{} `json:"error,omitempty"`
	Method string      `json:"method"`
	// Version string      `json:"jsonrpc"`
	Params interface{} `json:"params"`
}

type JSONPushMessage struct {
	// FIXME: Temporarily add ID for Claymore compliance
	Id      int64       `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

type JSONRpcResp struct {
	Id      json.RawMessage `json:"id"`
	Version string          `json:"jsonrpc"`
	Result  interface{}     `json:"result"`
	Error   interface{}     `json:"error,omitempty"`
}

type JobReplyData struct {
	Blob   string `json:"blob"`
	JobId  string `json:"job_id"`
	Target string `json:"target"`
}

type SubmitReply struct {
	Status string `json:"status"`
}

type ErrorReply struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
