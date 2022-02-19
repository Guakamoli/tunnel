package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/util/log"
)

var (
	kcpDoneCh chan struct{}
)

func init() {
	kcpDoneCh = make(chan struct{})
}

func Execute(content []byte) {
	err := runClient(content)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func handleSignal(svr *client.Service) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	svr.GracefulClose(500 * time.Millisecond)
	close(kcpDoneCh)
}

func runClient(content []byte) (err error) {
	var (
		cfg         config.ClientCommonConf
		pxyCfgs     map[string]config.ProxyConf
		visitorCfgs map[string]config.VisitorConf
	)

	content, err = config.RenderContent(content)
	if err != nil {
		return
	}
	configBuffer := bytes.NewBuffer(nil)
	configBuffer.Write(content)

	// Parse common section.
	//cfg := ini.Empty()
	//_ = cfg.Append([]byte(iniContent))
	cfg, err = config.UnmarshalClientConfFromIni(content)
	if err != nil {
		return
	}
	cfg.Complete()
	if err = cfg.Validate(); err != nil {
		err = fmt.Errorf("Parse config error: %v", err)
		return
	}

	buf := make([]byte, 0)
	configBuffer.WriteString("\n")
	configBuffer.Write(buf)

	// Parse all proxy and visitor configs.
	pxyCfgs, visitorCfgs, err = config.LoadAllProxyConfsFromIni(cfg.User, configBuffer.Bytes(), cfg.Start)
	if err != nil {
		return
	}

	return startService(cfg, pxyCfgs, visitorCfgs)
}

func startService(
	cfg config.ClientCommonConf,
	pxyCfgs map[string]config.ProxyConf,
	visitorCfgs map[string]config.VisitorConf,
) (err error) {

	log.InitLog(cfg.LogWay, cfg.LogFile, cfg.LogLevel,
		cfg.LogMaxDays, cfg.DisableLogColor)

	if cfg.DNSServer != "" {
		s := cfg.DNSServer
		if !strings.Contains(s, ":") {
			s += ":53"
		}
		// Change default dns server for frpc
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.Dial("udp", s)
			},
		}
	}
	svr, errRet := client.NewService(cfg, pxyCfgs, visitorCfgs, "")
	if errRet != nil {
		err = errRet
		return
	}

	// Capture the exit signal if we use kcp.
	if cfg.Protocol == "kcp" {
		go handleSignal(svr)
	}

	err = svr.Run()
	if err == nil && cfg.Protocol == "kcp" {
		<-kcpDoneCh
	}
	return
}
