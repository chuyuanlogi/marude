package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

const CONFIG_FILE string = "ctrl.conf"

var Version = "debug"
var Url string

func checkUrl() {
	if len(Url) == 0 {
		if !read_config_file(CONFIG_FILE) {
			fmt.Printf("load config file failed and lost url information\n")
			os.Exit(1)
		}
		cfg := get_config()
		Url = fmt.Sprintf("http://%s:%s", cfg.Server.Ip, cfg.Server.Port)
	} else if Url[:5] != "http:" {
		Url = "http://" + Url
	}
	if Url[len(Url)-1] == '/' {
		Url = Url[:len(Url)-1]
	}
}

func subCmdVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("%s\n", Version)
}

func subCmdRun(cmd *cobra.Command, args []string) {
	checkUrl()
	param := url.Values{}
	param.Add("name", args[0])
	param.Add("case", args[1])
	requrl := fmt.Sprintf("%s/run_case", Url)
	if !HttpGet(requrl, param) {
		log.Printf("command failed\n")
	}
}

func subCmdRead(cmd *cobra.Command, args []string) {
	checkUrl()
	param := url.Values{}
	param.Add("name", args[0])
	param.Add("case", args[1])
	param.Add("fetch", "1")
	requrl := fmt.Sprintf("%s/run_case", Url)
	if !HttpGet(requrl, param) {
		log.Printf("command failed\n")
	}
}

func subCmdPeek(cmd *cobra.Command, args []string) {
	checkUrl()
	param := url.Values{}
	param.Add("name", args[0])
	param.Add("case", args[1])
	param.Add("fetch", "2")
	requrl := fmt.Sprintf("%s/run_case", Url)
	if !HttpGet(requrl, param) {
		log.Printf("command failed\n")
	}
}

func subCmdPeeek(cmd *cobra.Command, args []string) {
        checkUrl()
        param := url.Values{}
        param.Add("name", args[0])
        param.Add("case", args[1])
        param.Add("fetch", "3")
        requrl := fmt.Sprintf("%s/run_case", Url)
        if !HttpGet(requrl, param) {
                log.Printf("command failed\n")
        }
}


func subCmdList(cmd *cobra.Command, args []string) {
	checkUrl()
	requrl := fmt.Sprintf("%s/list", Url)
	if !HttpGet(requrl, url.Values{}) {
		log.Printf("command failed\n")
	}
}

func subCmdAskClientReg(cmd *cobra.Command, args []string) {
	if Url[:5] != "http:" {
		Url = "http://" + Url
	}
	if Url[len(Url)-1] == '/' {
		Url = Url[:len(Url)-1]
	}
	parsed, err := url.Parse(Url)
	if err != nil {
		log.Printf("url parsing failed: %v\n", err)
	}
	if len(parsed.Port()) == 0 {
		Url = fmt.Sprintf("http://%s:%s/%s", parsed.Host, "25305", "ask_reg")
	}
	requrl := fmt.Sprintf("%s", Url)
	if !HttpGet(requrl, url.Values{}) {
		log.Printf("command failed\n")
	}
}

func main() {
	argparse := &cobra.Command{
		Use:   "marude_ctrl",
		Short: "communicte with marude server",
	}

	argparse.SetHelpFunc(func(cmd *cobra.Command, argv []string) {
		fmt.Printf("%s\n\n", cmd.Short)
		fmt.Println(cmd.UsageString())
		os.Exit(0)
	})

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "version information",
		Run:   subCmdVersion,
	}
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "run the specific stress test",
		Run:   subCmdRun,
		Args:  cobra.ExactArgs(2),
	}
	readCmd := &cobra.Command{
		Use:   "read",
		Short: "read the stdout from the specific stress test and the buffer will be cleaned",
		Run:   subCmdRead,
		Args:  cobra.ExactArgs(2),
	}
	peekCmd := &cobra.Command{
		Use:   "peek",
		Short: "read the stdout from the specific stress test and won't clean the buffer)",
		Run:   subCmdPeek,
		Args:  cobra.ExactArgs(2),
	}
        peeekCmd := &cobra.Command{
                Use:   "peeek",
                Short: "read the stdout from the specific stress test and won't clean the buffer)",
                Run:   subCmdPeeek,
                Args:  cobra.ExactArgs(2),
        }
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list all machine and cases",
		Run:   subCmdList,
	}
	askregCmd := &cobra.Command{
		Use:   "askreg",
		Short: "ask client to do the register process",
		Run:   subCmdAskClientReg,
	}

	argparse.PersistentFlags().StringVarP(&Url, "url", "u", "", "server url")

	argparse.AddCommand(versionCmd)
	argparse.AddCommand(runCmd)
	argparse.AddCommand(readCmd)
	argparse.AddCommand(peekCmd)
	argparse.AddCommand(peeekCmd)
	argparse.AddCommand(listCmd)
	argparse.AddCommand(askregCmd)

	if err := argparse.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
