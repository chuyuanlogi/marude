package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"doraemon.pocket/common"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/smallnest/ringbuffer"
	"github.com/spf13/cobra"
)

const CONFIG_FILE string = "config.ini"
const BUFSIZE = 32 * 1024
const RBSIZE = 256 * 1024
const RBCKSIZE = 4 * 1024

type Status int
type Nrbbuf struct {
	*ringbuffer.RingBuffer
	rbck *ringbuffer.RingBuffer
}

const (
	Idle Status = iota
	Running
	Finished
	Queued
)

type RunStatus struct {
	status    Status
	cmdline   string
	c         *exec.Cmd
	rb        *Nrbbuf
	done_chan chan struct{}
}

var Glogger *logrus.Logger
var Version = "debug"
var Gapp *fiber.App

func (r *Nrbbuf) ReadFrom(rd io.Reader) (err error) {
	read_buf := make([]byte, RBSIZE/16)
	var zeroReads int = 0
	for {
		remining := r.RingBuffer.Free()
		if remining < (RBSIZE / 3) {
			drop := make([]byte, RBSIZE/2)
			r.RingBuffer.Read(drop)
			Glogger.Debugf("drop old data %d\n", RBSIZE/2)
		}
		remining = r.rbck.Free()
		if remining < (RBCKSIZE / 3) {
			drop := make([]byte, RBCKSIZE/2)
			r.rbck.Read(drop)
		}

		nr, rerr := rd.Read(read_buf)
		if rerr != nil && rerr != io.EOF {
			Glogger.Infof("read io.reader failed: %v\n", rerr)
			return rerr
		} else if rerr == io.EOF {
			Glogger.Infof("io.reader EOF, ready for closing write ringbuffer\n")
			return nil
		}
		if nr == 0 && rerr == nil {
			zeroReads++
			if zeroReads >= 300 {
				Glogger.Errorf("read 0 length over than 300 times\n")
			}
			continue
		}
		zeroReads = 0
		r.RingBuffer.Write(read_buf[:nr])
		r.rbck.Write(read_buf[:nr])
	}
	return nil
}

func (r *Nrbbuf) ReadCloser() io.ReadCloser {
	return r.RingBuffer.ReadCloser()
}

func (r *Nrbbuf) Capacity() int {
	return r.RingBuffer.Capacity()
}

func (r *Nrbbuf) Length() int {
	return r.RingBuffer.Length()
}

func (r *Nrbbuf) Free() int {
	return r.RingBuffer.Free()
}

func (r *Nrbbuf) CloseWriter() {
	r.RingBuffer.CloseWriter()
}

func (r *Nrbbuf) Reset() {
	r.RingBuffer.Reset()
	r.rbck.Reset()
}

func (r *Nrbbuf) CheckResult() []byte {
	b := make([]byte, RBCKSIZE)
	n, _ := r.rbck.Peek(b)
	return b[:n]
}

func initRunStatus(cfg *CfgData, caseStatus map[string]*RunStatus) bool {
	for k, v := range cfg.Case {
		caseStatus[k] = &RunStatus{
			status:  Idle,
			cmdline: v.Exec,
			c:       nil,
			rb: &Nrbbuf{
				RingBuffer: ringbuffer.New(RBSIZE).SetBlocking(true),
				rbck:       ringbuffer.New(RBCKSIZE),
			},
		}
	}
	return true
}

type readonly struct{ io.Reader }

func (readonly) Close() error { return nil }
func removeCloseMethod(rc io.ReadCloser) io.Reader {
	return readonly{rc}
}

func clear_marude_env() {
	for _, v := range os.Environ() {
		sz := strings.SplitN(v, "=", 2)
		if len(sz) == 2 {
			if strings.HasPrefix(sz[0], "MARUDE_") {
				os.Unsetenv(sz[0])
			}
		}
	}
}

type QCmd struct {
	caseInfo  *CfgCase
	runStatus *RunStatus
}

var queue_cmd = make(chan QCmd, 30)

func cmd_queue_handler(q_cmd <-chan QCmd) {
	for cmd := range q_cmd {
		cmd.runStatus.done_chan = make(chan struct{})
		clear_marude_env()
		cmd.runStatus.rb.Reset()
		Glogger.Infof("run command %s from queue\n", cmd.caseInfo.Exec)
		_, err := run_cmd(cmd.caseInfo, cmd.runStatus)
		if err != nil {
			Glogger.Infof("run command %s in queue failed\n", cmd.caseInfo.Exec)
			continue
		}
		if cmd.runStatus.done_chan != nil {
			Glogger.Infof("wait for cmd finish in queue\n")
			<-cmd.runStatus.done_chan
			Glogger.Infof("ok, ready to run next cmd in queue\n")
		}
	}
}

func IsAnyCmdRun(cfg *CfgData, caseStatus map[string]*RunStatus) bool {
	for k, _ := range cfg.Case {
		if caseStatus[k].status == Queued || caseStatus[k].status == Running {
			return true
		}
	}
	return false
}

func fiber_service(cfg *CfgData, caseStatus map[string]*RunStatus) {
	app := fiber.New(fiber.Config{
		ReadBufferSize:  BUFSIZE,
		WriteBufferSize: BUFSIZE,
	})
	Gapp = app

	app.Get("/run/*", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Transfer-Encoding", "chunked")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		arg := c.Params("*1")
		for key, value := range cfg.Case {
			if arg == key {
				if value.Single == "yes" && (caseStatus[key].status == Running || caseStatus[key].status == Queued) {
					Glogger.Infof("the case %s still running", key)
					return c.SendString(fmt.Sprintf("the case %s is running now...\n", key))
				} else if value.Single == "yes" {
					if IsAnyCmdRun(cfg, caseStatus) {
						Glogger.Infof("enqueue the case %s\n", key)
						caseStatus[key].status = Queued
						queue_cmd <- QCmd{value, caseStatus[key]}
						return c.SendString(fmt.Sprintf("the case %s enqueued\n", key))
					}
				}
				// run exec command
				clear_marude_env()
				caseStatus[key].rb.Reset()
				caseStatus[key].done_chan = nil
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
		params := c.Queries()

		s, ok := caseStatus[arg]
		if ok {
			if s.c != nil && s.c.ProcessState != nil && s.c.ProcessState.Exited() {
				s.status = Idle
				s.cmdline = ""
				s.c = nil
			}

			p, ok := params["rb"]
			if ok && p == "1" {
				str := fmt.Sprintf("case: %s ringbuffer: %d, %d, %d\n", arg, s.rb.Capacity(), s.rb.Length(), s.rb.Free())
				return c.SendString(str)
			}

			str := fmt.Sprintf("status: %s\ncmdline: %s\n", []string{"Idle", "Running", "Finished", "Queued"}[s.status], s.cmdline)
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

	app.Get("/peek/*", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Transfer-Encoding", "chunked")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		arg := c.Params("*1")

		s, ok := caseStatus[arg]
		if ok {
			leng := s.rb.rbck.Length()
			data := make([]byte, leng)
			s.rb.rbck.Peek(data)
			return c.SendString(string(data))
		}

		return c.SendString("the case is not supported\n")
	})

	app.Get("/peeek/*", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Transfer-Encoding", "chunked")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		arg := c.Params("*1")

		s, ok := caseStatus[arg]
		if ok {
			leng := s.rb.RingBuffer.Length()
			data := make([]byte, leng)
			s.rb.RingBuffer.Peek(data)
			return c.SendString(string(data))
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
	var ignore_reg bool = false
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
	argparse.Flags().BoolVar(&ignore_reg, "noregister", false, "ignore register to server")
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

	if ignore_reg == false {
		if !register(cfg) {
			Glogger.Fatal("Register failed!\n")
			return
		}
	}

	go cmd_queue_handler(queue_cmd)

	Init_service(cfg, caseStatus)

}
