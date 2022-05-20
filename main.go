package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatedier/golib/crypto"
	"github.com/flxxyz/tunnel/cmd"
	flags "github.com/jessevdk/go-flags"
	"github.com/ouqiang/goutil/httpclient"
)

const (
	Version        = "0.1.1"
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
	ConfigUrl   = "https://tunnel.deno.dev/%s.json"
	CheckUrl    = "https://tunnel.deno.dev/check-version"
	HttpTimeout = time.Second * 60
)

type UserConfig struct {
	ServerAddr    string `json:"server_addr"`
	ServerPort    int    `json:"server_port"`
	Token         string `json:"token"`
	SubdomainHost string `json:"subdomain_host"`
	LocalAddr     string `short:"H" long:"host" default:"127.0.0.1" description:"代理的地址"`
	LocalPort     int    `short:"p" long:"port" default:"8080" description:"代理的端口"`
	Debug         bool   `short:"d" long:"debug" description:"打开调试模式"`
	Version       bool   `short:"v" long:"version" description:"显示版本号"`
}

var (
	defaultLetters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	config         = &UserConfig{}
	headers        = http.Header{}
)

func init() {
	rand.Seed(time.Now().UnixNano())

	// 1. 解析命令参数
	parseFlags()
	// 2. 检查版本号
	checkVersion()
	// 3. 拉取服务器参数
	fetchConfig()
}

func main() {
	crypto.DefaultSalt = "frp"
	name := randomString(6)
	content := replaceValues(name, config)
	fmt.Println(fmt.Sprintf("open tunnel address: https://%s.%s", name, config.SubdomainHost))
	cmd.Execute([]byte(content))
}

func httpGet(url string) *httpclient.Response {
	headers.Add("x-version", Version)
	httpOption := httpclient.WithConnectTimeout(HttpTimeout)
	req := httpclient.NewRequest(httpOption)
	response, err := req.Get(url, nil, headers)
	if err != nil {
		fmt.Printf("请求错误 error: %s", err.Error())
		os.Exit(1)
	}
	return response
}

func checkVersion() {
	headers := http.Header{}
	headers.Add("x-version", Version)
	response := httpGet(CheckUrl)
	content, err := response.Bytes()
	if err != nil {
		fmt.Printf("内部错误 error: %s", err.Error())
		os.Exit(1)
	}

	var version struct {
		Current string `json:"current"`
	}
	_ = json.Unmarshal(content, &version)

	if version.Current != Version {
		fmt.Printf("【发现新版本🆕 v%s 】\n", version.Current)
		fmt.Println("-------------------- 分割线 --------------------")
	}
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

func readIdentity() string {
	homeDir, _ := os.UserHomeDir()
	configPath := homeDir + "/.tunnel"

	_, err := os.Stat(configPath)
	if err != nil {
		fmt.Printf("读取用户失败 error: %s", err.Error())
		os.Exit(1)
	}

	identity, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("读取用户失败 error: %s", err.Error())
		os.Exit(1)
	}

	return strings.Trim(string(identity), "\n")
}

func fetchConfig() {
	identity := readIdentity()
	URL := fmt.Sprintf(ConfigUrl, identity)

	response := httpGet(URL)
	content, err := response.Bytes()
	if err != nil {
		fmt.Printf("获取配置失败 error: %s", err.Error())
		os.Exit(1)
	}

	var cipher struct {
		Encrypt string `json:"encrypt"`
		IV      string `json:"iv"`
	}

	if err := json.Unmarshal(content, &cipher); err != nil {
		fmt.Printf("获取配置失败 error: %s", err.Error())
		os.Exit(1)
	}

	// 解密AES数据，填充至 config
	parseAESData := decodeAES(identity, cipher.Encrypt, cipher.IV)
	if err := json.Unmarshal(parseAESData, &config); err != nil {
		fmt.Printf("解密数据失败 error: %s", err.Error())
		os.Exit(1)
	}
}

func decodeAES(key, encrypt, iv string) []byte {
	s := fmt.Sprintf("%x", sha512.Sum512([]byte(key)))
	cipherKey := []byte(s[:32])
	cipherIV, _ := base64.StdEncoding.DecodeString(iv)

	block, _ := aes.NewCipher(cipherKey)
	mode := cipher.NewCBCDecrypter(block, cipherIV)
	encryptBytes, _ := base64.StdEncoding.DecodeString(encrypt)
	mode.CryptBlocks(encryptBytes, encryptBytes)
	return PKCS7UNPadding(encryptBytes)
}

func PKCS7UNPadding(originBytes []byte) []byte {
	originLength := len(originBytes)
	if originLength == 0 {
		return originBytes
	}
	unpadding := int(originBytes[originLength-1])
	return originBytes[:(originLength - unpadding)]
}

func replaceValues(name string, config *UserConfig) string {
	template := strings.Replace(ConfigTemplate, "%server_addr%", config.ServerAddr, 1)
	template = strings.Replace(template, "%server_port%", strconv.Itoa(config.ServerPort), 1)
	template = strings.Replace(template, "%token%", config.Token, 1)

	template = strings.Replace(template, "%name%", name, -1)
	template = strings.Replace(template, "%local_ip%", config.LocalAddr, 1)
	template = strings.Replace(template, "%local_port%", strconv.Itoa(config.LocalPort), 1)

	logLevel := "error"
	if config.Debug {
		logLevel = "trace"
	}
	template = strings.Replace(template, "%log_level%", logLevel, 1)

	return template
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

func genBytes(bytes int, identity []byte) []byte {
	b := make([]byte, bytes)

	for i := 0; i < bytes; i++ {
		if i >= len(identity) {
			b[i] = byte(rand.Intn(254) + 1)
		} else {
			b[i] = identity[i]
		}
	}

	return b
}

func gen16bytes(identity string) []byte {
	return genBytes(16, []byte(identity))
}

func gen24bytes(identity string) []byte {
	return genBytes(24, []byte(identity))
}

func gen32bytes(identity string) []byte {
	return genBytes(32, []byte(identity))
}

func randomBytes(bytes int) []byte {
	return genBytes(bytes, nil)
}
