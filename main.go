package main

import (
	"fmt"
	"github.com/spf13/viper"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

var testURL = ""
var baseDir = ""
var targetProxy = ""

var currentServer = ""
var availableServers = make([]string, 0)
var globalSwitchChan = make(chan string)

func main() {
	initViper()
	validateServerFile()
	if len(availableServers) == 0 {
		fmt.Printf("[EMER] No available servers. Exiting.\n")
		return
	} else {
		fmt.Printf("[INFO] %d server(s) found.\n", len(availableServers))
	}

	go agent()

	globalSwitchChan <- useNewServer("")
	randN := rand.New(rand.NewSource(time.Now().Unix()))
	firstStart := true

	for {
		for {
			if firstStart {
				time.Sleep(2 * time.Second)
				firstStart = false
			}
			fmt.Println("[INFO] Checking server...")
			ret := testConnWrapper()
			if ret {
				break
			}
			fmt.Println("[INFO] Server failed, switching.")
			globalSwitchChan <- useNewServer(currentServer)
			firstStart = true
		}
		dur := time.Duration(60+randN.Intn(60)) * time.Second
		fmt.Println("[INFO] Next check after", dur)
		time.Sleep(dur)
	}
}

func initViper() {
	viper.SetConfigName("sss")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("Failed parsing config: %s \n", err)
		os.Exit(255)
	}
	viper.SetDefault("testurl", "https://zh.wikipedia.org")
	viper.SetDefault("basedir", ".")
	viper.SetDefault("servers", []string{})
	viper.SetDefault("proxy", "socks5://127.0.0.1:9050")
	testURL = viper.GetString("testurl")
	baseDir = viper.GetString("basedir")
	availableServers = viper.GetStringSlice("servers")
	targetProxy = viper.GetString("proxy")
}

func validateServerFile() {
	newServers := make([]string, 0)
	for _, name := range availableServers {
		if _, err := os.Stat(baseDir + "/" + name + ".json"); os.IsNotExist(err) {
			fmt.Printf("[WARN] File %s.json is not found.\n", name)
		} else {
			newServers = append(newServers, name)
		}
	}
	availableServers = newServers
}

func randFrom(randList []string) string {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	return randList[r.Intn(len(randList))]
}

func useNewServer(currServer string) string {
	for {
		candidate := randFrom(availableServers)
		if candidate != currServer {
			return candidate
		}
	}
}

func testConnWrapper() bool {
	ret := testConnection()
	if ret == false {
		return testConnection()
	}
	return true
}

func testConnection() bool {
	targetProxyURL, err := url.Parse(targetProxy)
	if err != nil {
		log.Fatal("[WARN] Error parsing proxy URL:", targetProxy, ".", err)
	}

	targetTransport := &http.Transport{Proxy: http.ProxyURL(targetProxyURL)}
	client := &http.Client{Transport: targetTransport, Timeout: time.Second * 3}

	start := time.Now()
	resp, err := client.Get(testURL)
	currentTime := time.Now()
	if err != nil {
		fmt.Printf("[WARN] [%s] Error making GET request: %s\n", currentTime.Format("15:04:05"), err)
		return false
	}
	defer resp.Body.Close()

	fmt.Printf("[INFO] [%s] HTTP %d in %s\n", currentTime.Format("15:04:05"), resp.StatusCode, time.Since(start))
	return resp.StatusCode == 200
}

func agent() {
	var err error
	var currSvr string
	var proxyProcess = new(exec.Cmd)
	for {
		currSvr = <-globalSwitchChan
		if proxyProcess.Process != nil {
			err = proxyProcess.Process.Kill()
		}
		if err != nil {
			fmt.Printf("Error killing process: %s\n", err.Error())
			os.Exit(1)
			return
		}
		proxyProcess = exec.Command("ss-local", "-c", baseDir+"/"+currSvr+".json")
		err = proxyProcess.Start()
		if err != nil {
			fmt.Printf("Error starting process: %s\n", err.Error())
			os.Exit(1)
			return
		}
		fmt.Printf("[INFO] Server %s started.\n", currSvr)
		time.Sleep(1 * time.Second)
		currentServer = currSvr
	}
}
