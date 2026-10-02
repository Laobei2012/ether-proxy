# ether-proxy

Ethereum (ethash) mining proxy with web-interface.

**Proxy feature list:**

* Rigs availability monitoring
* Keep track of accepts, rejects, blocks stats
* Easy detection of sick rigs
* Daemon failover list

![Demo](proxy.png)

> **Status:** Ethereum mainnet moved to proof of stake, so ethash mining no longer works
> there. This project is kept for ethash-based chains and for reference.

### Building

Requires Go >= 1.16.

    go build -ldflags="-s -w" -o ether-proxy main.go

The files under `lib/` are modified copies of `net/rpc` and `ethash`. They are not needed
for a normal module build; don't copy them over your system Go installation unless you
know you need the modified behaviour.

### Configuration

Copy *config.example.json* to *config.json* and set the listen addresses and upstream URLs.

#### Example upstream section

```javascript
"upstream": [
  {
    "pool": true,
    "name": "Example pool",
    "url": "http://pool.example.com:8888/miner/YOUR_WALLET/proxy",
    "timeout": "10s"
  },
  {
    "name": "backup-geth",
    "url": "http://127.0.0.1:8545",
    "timeout": "10s"
  }
],
```

Here a mining pool is the main target and a local geth node is the solo backup.

With <code>"submitHashrate": true|false</code> proxy will forward <code>eth_submitHashrate</code> requests to upstream.

#### Running

    ./ether-proxy -mode 10 -config config.json

#### Mining

    ethminer -F http://x.x.x.x:8546/miner/5/gpu-rig -G
    ethminer -F http://x.x.x.x:8546/miner/0.1/cpu-rig -C

### Agent mode (optional)

`-mode 0` runs a rig agent instead of the proxy. It reports hardware info, including the
public IP (looked up via `ifconfig.co`), to an Alibaba Cloud MQTT broker every minute. It
refuses to start unless the `agent` section of the config (broker, accessKey, secretKey,
instanceId, groupId) is filled in. The proxy itself never connects to MQTT. The `exec` and
`config` MQTT handlers only log the message; they execute nothing.

### Credits and license

Forked from [sammy007/ether-proxy](https://github.com/sammy007/ether-proxy).
MIT licensed, see `LICENSE`. Bundled third-party code is listed in `THIRD_PARTY.md`.
