package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/go-git/gcfg"
)

// config file format:
//
// [Server]
// Ip=x.x.x.x
// Port=xxxxx
//
//

type CfgData struct {
	Server struct {
		Ip   string
		Port string
	}
}

var g_cfg = CfgData{}

func validate_port(port string) bool {
	i, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	} else if i < 0 {
		log.Fatalf("invalid network port\n")
		return false
	} else if i > 65535 {
		log.Fatalf("invalid network port\n")
		return false
	}

	return true
}

func validate_ip(addr string) bool {
	if len(addr) == 0 {
		log.Fatalf("ip address is empty\n")
		return false
	}

	var PatternIp = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$`)

	return PatternIp.MatchString(addr)
}

func validate_config() bool {
	if !validate_port(g_cfg.Server.Port) {
		return false
	}

	if !validate_ip(g_cfg.Server.Ip) {
		log.Fatalf("invalid server ip\n")
		return false
	}

	return true
}

func proc_conf_path(file string) string {
	// priority userconf --> sysconf --> working dir

	// 1. user configuration directory
	//		* linux: ~/.config/marude/marude.conf
	//		* windows: %AppData%\Roaming\marude\marude.conf
	//		* macos: /Users/xxx/Library/Application Support/marude/marude.conf
	// 2. system configuration directory
	//		* linux: /etc/marude/marude.conf
	//		* windows: %ProgramData%\marude/marude.conf
	//		* macos: /Library/Application Support/marude/marude.conf
	var conf_file string = ""

	switch runtime.GOOS {
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

	_, err := os.Stat(conf_file)
	if !os.IsNotExist(err) {
		return conf_file
	}

	return ""
}

func read_config_file(file string) bool {
	cfg_file := proc_conf_path(file)
	log.Printf("load conf file: %s\n", cfg_file)

	err := gcfg.ReadFileInto(&g_cfg, cfg_file)
	if err != nil {
		log.Fatalf("Failed to parse config data: %s\n", err)
		return false
	}

	return validate_config()
}

func get_config() *CfgData {
	return &g_cfg
}
