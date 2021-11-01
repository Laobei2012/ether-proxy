package agent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/robfig/cron.v3"
)

type PayloadExec struct {
	ClientId  string `json:"clientId"`
	Seq       int    `json:"seq"`
	Cmd       string `json:"cmd"`
	Timestamp int    `json:"timestamp"`
}
type PayloadConfig struct {
	ClientId   string `json:"clientId"`
	Seq        int    `json:"seq"`
	ConfigType string `json:"configType"`
	Data       string `json:"data"`
	Timestamp  int    `json:"timestamp"`
}

type PayloadCommonResp struct {
	ClientId  string `json:"clientId"`
	Seq       int    `json:"seq"`
	Res       int    `json:"res"`
	Timestamp int    `json:"timestamp"`
}

var CURRENT_PATH string
var FARM string
var DEVICEID string

func ComputeHmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha1.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

var messageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	// topic: bos/cmd/{farm}/{client ID}/{cmd keyword}
	string_slice := strings.Split(msg.Topic(), "/")
	switch string_slice[4] {
	case "exec":
		var payload PayloadExec
		json.Unmarshal(msg.Payload(), &payload)
		log.Print(payload)
	case "config":
		var payload PayloadConfig
		json.Unmarshal(msg.Payload(), &payload)
		log.Print(payload)

	}
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	opts := client.OptionsReader()
	log.Println("MQTT broker connected", opts.ClientID())
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	opts := client.OptionsReader()
	log.Printf("MQTT broker connect lost: %v, %s", err, opts.ClientID())
}

func publish(client mqtt.Client, topic string, msg string) {
	token := client.Publish(topic, 0, false, msg)
	token.Wait()
	err := token.Error()
	if err != nil {
		log.Printf("publish Error: %s", err.Error())
	}
	log.Printf("publish to topic: %s, message: %s", topic, msg)
}

func subscribe(client mqtt.Client, topic string) {
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	err := token.Error()
	if err != nil {
		log.Fatal("subscribe Error: ", err.Error())
	}
	log.Printf("Subscribed to topic: %s", topic)
}

func reportOnStart(client mqtt.Client) {
	cmd := exec.Command(CURRENT_PATH + "/scripts/bminer/hw_info.sh")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("failed to call Run(): %v", err)
	}
	log.Printf("hw_info out:\n%s\nerr:\n%s", stdout.String(), stderr.String())

	var topic = "bos/data/" + FARM + "/" + DEVICEID + "/report"
	publish(client, topic, stdout.String())
}

func StartMQTTClient() {
	ex, err := os.Executable()
	if err != nil {
		log.Println(err)
	}
	CURRENT_PATH = filepath.Dir(ex)
	FARM = "default"
	DEVICEID = "0011"

	log.Println("starting MQTT client at ", CURRENT_PATH)
	c := make(chan os.Signal, 1)

	var broker = "mqtt-cn-zvp2f141s0i.mqtt.aliyuncs.com"
	var port = 1883
	var accessKey = "LTAI5tEn9BLvScdXZcvGXDVQ"
	var secretKey = "Jzvho1IB1pJjWh5HwTqvuJNJZIlWDl"
	var instanceId = "mqtt-cn-zvp2f141s0i"
	var groupId = "GID_szmqtt"
	var clientId = groupId + "@@@" + DEVICEID

	var userName = "Signature" + "|" + accessKey + "|" + instanceId
	var password = ComputeHmac256(clientId, secretKey)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID(clientId)
	opts.SetUsername(userName)
	opts.SetPassword(password)
	opts.SetDefaultPublishHandler(messageHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("MQTT error: ", token.Error())
		panic(token.Error())
	}

	var subTopic = "bos/cmd/" + FARM + "/" + DEVICEID + "/+"
	subscribe(client, subTopic)

	crontab := cron.New()
	crontab.AddFunc("* * * * *", func() {
		reportOnStart(client)
	})

	crontab.Start()

	<-c
	// client.Disconnect(250)
}
