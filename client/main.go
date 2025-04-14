package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"doraemon.pocket/common"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/smallnest/ringbuffer"
	"github.com/spf13/cobra"
)

const CONFIG_FILE string = "config.ini"

type Status int

const (
	Idle Status = iota
	Running
	Finished
)

type RunStatus struct {
	status  Status
	cmdline string
	c       *exec.Cmd
	rb      *ringbuffer.RingBuffer
}

var Glogger *logrus.Logger
var Version = "debug"
var Gapp *fiber.App

func initRunStatus(cfg *CfgData, caseStatus map[string]*RunStatus) bool {
	for k, v := range cfg.Case {
		caseStatus[k] = &RunStatus{
			status:  Idle,
			cmdline: v.Exec,
			c:       nil,
			rb:      ringbuffer.New(256 * 1024).SetBlocking(true),
		}
	}
	return true
}

type readonly struct{ io.Reader }

func (readonly) Close() error { return nil }
func removeCloseMethod(rc io.ReadCloser) io.Reader {
	return readonly{rc}
}

func fiber_service(cfg *CfgData, caseStatus map[string]*RunStatus) {
	app := fiber.New()
	Gapp = app

	app.Get("/run/*", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Transfer-Encoding", "chunked")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		arg := c.Params("*1")
		for key, value := range cfg.Case {
			if arg == key {
				if value.Single == "yes" && caseStatus[key].status == Running {
					Glogger.Infof("the case %s still running", key)
					return c.SendString(fmt.Sprintf("the case %s is running now...\n", key))
				}
				// run exec command
				_, err := run_cmd(value, caseStatus[key])

				if err != nil {
					Glogger.Infof("run command: %s failed, err: %v", value.Exec, err)
				}

				reader := removeCloseMethod(caseStatus[key].rb.ReadCloser())
				return c.SendStream(reader)
			}
		}
		return c.SendString(fmt.Sprintf("not supported cases %s\n", arg))
	})

	app.Get("/list", func(c *fiber.Ctx) error {
		var cases string = ""
		for key, _ := range cfg.Case {
			cases += fmt.Sprintf("Case: %s\n", key)
		}

		return c.SendString(cases)
	})

	app.Get("/status/*", func(c *fiber.Ctx) error {
		arg := c.Params("*1")

		s, ok := caseStatus[arg]
		if ok {
			if s.c != nil && s.c.ProcessState != nil && s.c.ProcessState.Exited() {
				s.status = Idle
				s.cmdline = ""
				s.c = nil
			}
			str := fmt.Sprintf("status: %s\ncmdline: %s\n", []string{"Idle", "Running", "Finished"}[s.status], s.cmdline)
			return c.SendString(str)
		}

		return c.SendString("the case is not supported\n")
	})

	app.Get("/resume/*", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Transfer-Encoding", "chunked")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		arg := c.Params("*1")

		s, ok := caseStatus[arg]
		if ok {
			if s.status == Finished {
				fmt.Printf("try to resume!!\n")
				reader := removeCloseMethod(s.rb.ReadCloser())
				s.status = Idle
				s.c = nil
				return c.SendStream(reader)
			} else if s.c != nil && s.c.ProcessState != nil && s.c.ProcessState.Exited() {
				s.status = Idle
				s.c = nil
				return c.SendString("the case is not running now\n")
			} else if s.c != nil && s.rb != nil {
				fmt.Printf("try to resume!!\n")
				reader := removeCloseMethod(s.rb.ReadCloser())
				return c.SendStream(reader)
			}
		}

		return c.SendString("the case is not supported\n")
	})

	app.Get("/terminate/*", func(c *fiber.Ctx) error {
		arg := c.Params("*1")

		s, ok := caseStatus[arg]
		if ok {
			if s.c != nil && s.c.Process != nil {
				if err := s.c.Process.Kill(); err != nil {
					Glogger.Fatalf("error! %v\n", err)
				}
				s.status = Idle
				s.c = nil

				return c.SendString("process is terminated\n")
			}

		}

		return c.SendString("the case is not supported\n")
	})

	app.Get("/ask_reg", func(c *fiber.Ctx) error {
		if !register(cfg) {
			Glogger.Fatal("Register failed!\n")
			return c.SendString("register failed\n")
		}
		return c.SendString("registered!\n")
	})

	Glogger.Fatal(app.Listen(fmt.Sprintf(":%s", cfg.Init.ClientPort)))
}

func main() {
	var show_version bool = false
	argparse := &cobra.Command{
		Use:   "marude_client [--version]",
		Short: "run long time stress test tool client",
		Run: func(cmd *cobra.Command, argv []string) {
			if show_version {
				fmt.Printf("version: %s\n", Version)
				os.Exit(0)
			}
		},
	}
	argparse.SetHelpFunc(func(cmd *cobra.Command, argv []string) {
		fmt.Printf("%s\n\n", cmd.Short)
		fmt.Println(cmd.UsageString())
		os.Exit(0)
	})
	argparse.Flags().BoolVar(&show_version, "version", false, "show current version")
	if err := argparse.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	logger, err := common.InitLog("")

	if err != nil {
		log.Fatalf("logger create failed: %v\n", err)
	}

	Glogger = logger

	if !read_config_file(CONFIG_FILE) {
		return
	}

	cfg := get_config()

	caseStatus := make(map[string]*RunStatus)
	initRunStatus(cfg, caseStatus)

	fmt.Println(fmt.Sprintf(":%s", cfg.Init.ClientPort))

	if !register(cfg) {
		Glogger.Fatal("Register failed!\n")
		return
	}

	Init_service(cfg, caseStatus)

}
