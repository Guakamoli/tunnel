package main

import (
	"encoding/json"
	"fmt"
	"github.com/flxxyz/tunnel/cmd"
	"github.com/jessevdk/go-flags"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/fatedier/frp/assets/frpc"
	"github.com/fatedier/golib/crypto"
	"github.com/ouqiang/goutil/httpclient"
)

const (
	Version        = "0.0.1"
	ConfigTemplate = `[common]
server_addr = %server_addr%
server_port = %server_port%
token = %token%
protocol = kcp
log_level = %log_level%
pool_count = 2

[http-%name%]
type = http
local_ip = %local_ip%
local_port = %local_port%
use_encryption = false
use_compression = true
subdomain = %name%`
	ConfigUrl = "https://guaka-tunnel.oss-ap-southeast-1.aliyuncs.com/%s.json"
)

type UserConfig struct {
	ServerAddr string `json:"server_addr"`
	ServerPort int    `json:"server_port"`
	Token      string `json:"token"`
	serverName string
	LocalAddr  string `short:"H" long:"host" default:"127.0.0.1" description:"代理的地址"`
	LocalPort  int    `short:"p" long:"port" default:"8080" description:"代理的端口"`
	Debug      bool   `short:"d" long:"debug" description:"打开调试模式"`
	Version    bool   `short:"v" long:"version" description:"显示版本号"`
}

var (
	defaultLetters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	config         = &UserConfig{}
)

func init() {
	parseFlags()
	fetchConfig()
}

func main() {
	crypto.DefaultSalt = "frp"
	rand.Seed(time.Now().UnixNano())

	serverName, content := replaceValues(config)
	fmt.Println(fmt.Sprintf("open tunnel address: https://%s", strings.Replace(config.ServerAddr, "tunnel", serverName, 1)))
	cmd.Execute(content)
}

func parseFlags() {
	config.Debug = false
	config.LocalAddr = "127.0.0.1"
	config.LocalPort = 8080
	parser := flags.NewParser(config, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	if config.Version {
		fmt.Println(Version)
		os.Exit(0)
	}
}

func readUsername() string {
	homeDir, _ := os.UserHomeDir()
	configPath := homeDir + "/.tunnel"

	_, err := os.Stat(configPath)
	if err != nil {
		fmt.Printf("读取用户失败 error: %s", err.Error())
		os.Exit(1)
	}

	username, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("读取用户失败 error: %s", err.Error())
		os.Exit(1)
	}

	return strings.Trim(string(username), "\n")
}

func fetchConfig() {
	username := readUsername()
	URL := fmt.Sprintf(ConfigUrl, username)

	headers := http.Header{}
	headers.Add("x-version", Version)
	response, _ := httpclient.Get(URL, nil, headers)
	content, err := response.Bytes()
	if err != nil {
		fmt.Printf("获取配置失败 error: %s", err.Error())
		os.Exit(1)
	}

	if err := json.Unmarshal(content, config); err != nil {
		fmt.Printf("获取配置失败 error: %s", err.Error())
		os.Exit(1)
	}
}

func replaceValues(config *UserConfig) (string, []byte) {
	template := strings.Replace(ConfigTemplate, "%server_addr%", config.ServerAddr, 1)
	template = strings.Replace(template, "%server_port%", strconv.Itoa(config.ServerPort), 1)
	template = strings.Replace(template, "%token%", config.Token, 1)

	serverName := randomString(6)
	template = strings.Replace(template, "%name%", serverName, -1)
	template = strings.Replace(template, "%local_ip%", config.LocalAddr, 1)
	template = strings.Replace(template, "%local_port%", strconv.Itoa(config.LocalPort), 1)

	logLevel := "error"
	if config.Debug {
		logLevel = "trace"
	}
	template = strings.Replace(template, "%log_level%", logLevel, 1)

	return serverName, []byte(template)
}

func randomString(n int, allowedChars ...[]rune) string {
	var letters []rune

	if len(allowedChars) == 0 {
		letters = defaultLetters
	} else {
		letters = allowedChars[0]
	}

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}
