package util

import (
	"math/big"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
)

var pow256 = math.BigPow(2, 256)

func Random() string {
	min := int64(100000000000000)
	max := int64(999999999999999)
	n := rand.Int63n(max-min+1) + min
	return strconv.FormatInt(n, 10)
}

func MakeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func MakeTargetHex(minerDifficulty float64) string {
	minerAdjustedDifficulty := int64(minerDifficulty * 1000000 * 100)
	difficulty := big.NewInt(minerAdjustedDifficulty)
	diff1 := new(big.Int).Div(pow256, difficulty)
	return string(hexutil.Encode(diff1.Bytes()))
}

func TargetHexToDiff(targetHex string) *big.Int {
	// log.Printf("targetHex %x / %s", pow256, targetHex)
	targetBytes := common.FromHex(targetHex)
	return new(big.Int).Div(pow256, new(big.Int).SetBytes(targetBytes))
}

func TargetHexToDoubleDiff(targetHex string) *big.Float {
	targetBytes := common.FromHex(targetHex)
	diff1 := new(big.Int).SetBytes(common.FromHex("0x00000000FFFF0000000000000000000000000000000000000000000000000000"))
	target := new(big.Int).SetBytes(targetBytes)
	return new(big.Float).Quo(new(big.Float).SetInt(diff1), new(big.Float).SetInt(target))
}

var Ether = math.BigPow(10, 18)
var Shannon = math.BigPow(10, 9)

var addressPattern = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
var zeroHash = regexp.MustCompile("^0?x?0+$")

func IsValidHexAddress(s string) bool {
	if IsZeroHash(s) || !addressPattern.MatchString(s) {
		return false
	}
	return true
}

func IsZeroHash(s string) bool {
	return zeroHash.MatchString(s)
}

func GetTargetHex(diff int64) string {
	difficulty := big.NewInt(diff)
	diff1 := new(big.Int).Div(pow256, difficulty)
	return string(hexutil.Encode(diff1.Bytes()))
}

func ToHex(n int64) string {
	return "0x0" + strconv.FormatInt(n, 16)
}

func FormatReward(reward *big.Int) string {
	return reward.String()
}

func FormatRatReward(reward *big.Rat) string {
	wei := new(big.Rat).SetInt(Ether)
	reward = reward.Quo(reward, wei)
	return reward.FloatString(8)
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func MustParseDuration(s string) time.Duration {
	value, err := time.ParseDuration(s)
	if err != nil {
		panic("util: Can't parse duration `" + s + "`: " + err.Error())
	}
	return value
}

func String2Big(num string) *big.Int {
	n := new(big.Int)
	n.SetString(num, 0)
	return n
}
