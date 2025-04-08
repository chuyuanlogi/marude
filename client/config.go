package main

import (
	"regexp"
	"strconv"
	"os"
	"runtime"
	"fmt"

	"github.com/go-git/gcfg"
)

// config file format:
// 
// [server]
// ip=x.x.x.x
// port=abcd
// 
// [init]
// adbusb=serial1
// adbusb=serial2
// adbip=a.a.a.a
// adbip=b.b.b.b
// port=abcd
// name=cccc
// nettype=wifi
// 
// 
// [case "a"]
// exec="xxxxx"
// adbdevice=serial or ip
// uart=uart id
// baud=xxxxx
// single=yes/no
// 
// 

type CfgCase struct {
	Exec		string
	AdbDevice	string
	Uart		string
	Baud		string
	UartLogName string
	Single		string
}

type CfgData struct {
	Server struct {
		Ip			string
		Port		string
	}
	Init struct {
		AdbUsb		[]string
		AdbIp		[]string
		ClientPort	string
		Name		string
		Nettype		string
	}
	Case map[string]*CfgCase
}

var g_cfg = CfgData{}

func validate_ip(addr string) bool {
	if len(addr) == 0 {
		Glogger.Fatalf("ip address is empty")
		return false
	}

	var PatternIp = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$`)

	return PatternIp.MatchString(addr)
}

func validate_port(port string) bool {
	i, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	} else if i < 0 {
		Glogger.Fatalf("invalid network port")
		return false
	} else if i > 65535 {
		Glogger.Fatalf("invalid network port")
		return false	
	}

	return true
}

func validate_config() bool {
	if !validate_ip(g_cfg.Server.Ip) {
		Glogger.Fatalf("invalid server ip address")
		return false
	}
	if !validate_port(g_cfg.Server.Port) {
		return false
	}
	if !validate_port(g_cfg.Init.ClientPort) {
		return false
	}

	if len(g_cfg.Init.Nettype) == 0 {
		g_cfg.Init.Nettype = "lan"
	} else if g_cfg.Init.Nettype != "lan" && g_cfg.Init.Nettype != "wifi" {
		g_cfg.Init.Nettype = "lan"
	}

	for _, addr := range g_cfg.Init.AdbIp {
		if !validate_ip(addr) {
			Glogger.Fatalf("invalid adb ip address: %s", addr)
			return false
		}
	}

	for _, c := range g_cfg.Case {
		if len(c.Single) == 0 {
			c.Single = "yes"
		} else if c.Single != "yes" {
			c.Single = "no"
		}

		if len(c.Baud) == 0 {
			c.Baud = "115200"
		}

		if len(c.UartLogName) <= 3 {
			c.UartLogName = "uart-%s"
		}
	}

	return true
}

func proc_conf_path(file string) string {
	// priority working dir --> userconf --> sysconf

	// 1. current working directory
	// 2. user configuration directory
	//		* linux: ~/.config/marude/config.ini
	//		* windows: %AppData%\Roaming\marude\config.ini
	//		* macos: /Users/xxx/Library/Application Support/marude/config.ini
	// 3. system configuration directory
	//		* linux: /etc/marude/config.ini
	//		* windows: %ProgramData%\marude/config.ini
	//		* macos: /Library/Application Support/marude/config.ini
	conf_file := file

	_, err := os.Stat(conf_file)
	if !os.IsNotExist(err) {
		return conf_file
	}

	switch(runtime.GOOS) {
	case "linux":
		user_path := os.Getenv("HOME")
		conf_file = fmt.Sprintf("%s/.config/marude/%s", user_path, file)
	case "windows":
		user_path, _ := os.UserConfigDir()
		conf_file = fmt.Sprintf("%s/marude/%s", user_path, file)
	case "darwin":
		user_path := os.Getenv("HOME")
		conf_file = fmt.Sprintf("%s/Library/Application Support/marude/%s", user_path, file)
	}

	_, err = os.Stat(conf_file)
	if !os.IsNotExist(err) {
		return conf_file
	}

	switch(runtime.GOOS) {
	case "linux":
		conf_file = fmt.Sprintf("/etc/marude/%s", file)
	case "windows":
		sysdata_path := os.Getenv("ProgramData")
		conf_file = fmt.Sprintf("%s/marude/%s", sysdata_path, file)
	case "darwin":
		conf_file = fmt.Sprintf("/Library/Application Support/marude/%s", file)
	}

	_, err = os.Stat(conf_file)
	if !os.IsNotExist(err) {
		return conf_file
	}

	return ""
}

func read_config_file(file string) bool {
	cfg_file := proc_conf_path(file)
	Glogger.Infof("load conf file: %s\n", cfg_file)

	err := gcfg.ReadFileInto(&g_cfg, cfg_file)
	if err != nil {
		Glogger.Fatalf("Failed to parse config data: %s", err)
		return false
	}

	return validate_config()
}

func get_config() *CfgData {
	return &g_cfg
}