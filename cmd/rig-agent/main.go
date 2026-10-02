// rig-agent runs on a mining rig and reports hardware info to an MQTT broker.
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"time"

	"ether-proxy/agent"
)

func main() {
	var cliConfigFile = flag.String("config", "agent.json", "agent config file")
	var cliLogPath = flag.String("logPath", "./", "agent log file path")

	flag.Parse()

	f, err := os.OpenFile(*cliLogPath+"/agent-"+time.Now().Format("2006-01-02")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, f))

	configFile, err := os.Open(*cliConfigFile)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()

	var cfg agent.Config
	if err = json.NewDecoder(configFile).Decode(&cfg); err != nil {
		log.Fatal("Config error: ", err.Error())
	}
	if cfg.Broker == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		log.Fatal("broker, accessKey and secretKey are required in ", *cliConfigFile)
	}

	agent.StartMQTTClient(cfg)
}
