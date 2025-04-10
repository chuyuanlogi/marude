package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

func prefix_detect(net_type string, name string) bool {
	prefix := []string{}
	switch net_type {
	case "lan":
		switch runtime.GOOS {
		case "linux":
			prefix = []string{"en", "eth"}
		case "windows":
			prefix = []string{"ethernet"}
		case "darwin":
			prefix = []string{"en0"}
		}
	case "wifi":
		switch runtime.GOOS {
		case "linux":
			prefix = []string{"wl", "wlan"}
		case "windows":
			prefix = []string{"wi-fi", "wifi"}
		case "darwin":
			prefix = []string{"en1"}
		}
	}

	for _, pfx := range prefix {
		if strings.HasPrefix(name, pfx) {
			return true
		}
	}

	return false
}

func get_ip(net_type string) string {
	interf, err := net.Interfaces()
	if err != nil {
		Glogger.Fatalf("failed to get network interfaces: %v\n", err)
		return ""
	}

	for _, iface := range interf {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		ifname := strings.ToLower(iface.Name)
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			if ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			if prefix_detect(net_type, ifname) {
				return ip.String()
			}
		}
	}

	return ""
}

func register(cfg *CfgData) bool {
	Url := fmt.Sprintf("http://%s:%s/register", cfg.Server.Ip, cfg.Server.Port)
	param := url.Values{}
	param.Add("name", cfg.Init.Name)
	param.Add("port", cfg.Init.ClientPort)
	param.Add("ip", get_ip(cfg.Init.Nettype))
	for _, adb_usb := range cfg.Init.AdbUsb {
		param.Add("device", adb_usb)
	}
	for _, adb_ip := range cfg.Init.AdbIp {
		param.Add("device_ip", adb_ip)
	}

	fullURL := Url + "?" + param.Encode()
	//log.Printf("request url: %s\n", fullURL)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		Glogger.Fatalf("register failed: %v\n", err)
		return false
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		Glogger.Fatalf("register http request failed\n")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		Glogger.Fatalf("server reject register\n")
		return false
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		Glogger.Fatalf("read response body failed\n")
		return false
	}

	if !(strings.HasPrefix(string(body), "Register Success") || strings.HasPrefix(string(body), "Registered")) {
		Glogger.Fatalf("register failed!\n")
		return false
	}

	return true
}
