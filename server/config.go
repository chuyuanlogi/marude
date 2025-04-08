package main

import (
	"strconv"
	"os"
	"runtime"
	"fmt"

	"github.com/go-git/gcfg"
)

// config file format:
// 
// [service]
// port=abcd
// log=path
// 
// 

type CfgData struct {
	Service struct {
		Port		string
		log			string
	}
}

var g_cfg = CfgData{}

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
	if !validate_port(g_cfg.Service.Port) {
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

	_, err := os.Stat(conf_file)
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