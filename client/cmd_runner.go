package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/shlex"
	"go.bug.st/serial"
)

type CmdOut struct {
	out io.ReadCloser
	err io.ReadCloser
}

func run_win_cmd(cmd ...string) (c *exec.Cmd, outs CmdOut, err error) {
	cmd = slices.Insert(cmd, 0, "/c")
	c = exec.Command("cmd.exe", cmd...)
	outs.out, _ = c.StdoutPipe()
	outs.err, _ = c.StderrPipe()

	Glogger.Println(c.Args)

	err = c.Start()
	if err != nil {
		return nil, outs, err
	}

	return c, outs, nil
}

func run_linux_cmd(cmd ...string) (c *exec.Cmd, outs CmdOut, err error) {
	//v := make([]interface{}, len(cmd))
	//for i, s := range cmd {
	//	v[i] = s
	//}
	//arg := fmt.Sprintln(v)
	//c = exec.Command("/bin/bash", "--login", "-c", arg)
	//c = exec.Command(cmd[0], cmd[1:]...)

	if cmd[0][:6] == "python" {
		c = exec.Command(cmd[0], cmd[1:]...)
	} else if cmd[0][len(cmd[0])-3:] == ".sh" {
		cmd = slices.Insert(cmd, 0, "--login")
		c = exec.Command("/bin/bash", cmd...)
	}
	outs.out, _ = c.StdoutPipe()
	outs.err, _ = c.StderrPipe()

	Glogger.Println(c.Args)

	err = c.Start()
	if err != nil {
		return nil, outs, err
	}

	return c, outs, nil
}

func run_macos_cmd(cmd ...string) (c *exec.Cmd, outs CmdOut, err error) {
	v := make([]interface{}, len(cmd))
	for i, s := range cmd {
		v[i] = s
	}

	arg := fmt.Sprintln(v)
	c = exec.Command("zsh", "-c", arg)
	outs.out, _ = c.StdoutPipe()
	outs.err, _ = c.StderrPipe()

	Glogger.Println(c.Args)

	err = c.Start()
	if err != nil {
		return nil, outs, err
	}

	return c, outs, nil
}

func check_win_cmd(result []byte, cmd ...string) string {
	cmd = slices.Insert(cmd, 0, "/c")
	c := exec.Command("cmd.exe", cmd...)
	proc_stdin, _ := c.StdinPipe()
	proc_out, _ := c.StdoutPipe()

	Glogger.Println("check for the result: ", c.Args)

	proc_stdin.Write(result)
	proc_stdin.Close()
	var out_buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(&out_buf, proc_out)
		if err != nil {
			Glogger.Errorf("sync stdout to buffer error: %v\n", err)
			return
		}
	}()

	err := c.Start()
	if err != nil {
		Glogger.Errorf("check result error1: %v\n", err)
		return ""
	}

	if err = c.Wait(); err != nil {
		Glogger.Errorf("check result error2: %v\n", err)
		return ""
	}

	wg.Wait()

	return out_buf.String()
}

func check_linux_cmd(result []byte, cmd ...string) string {
	var c *exec.Cmd

	if cmd[0][:6] == "python" {
		c = exec.Command(cmd[0], cmd[1:]...)
	} else if cmd[0][len(cmd[0])-3:] == ".sh" {
		cmd = slices.Insert(cmd, 0, "--login")
		c = exec.Command("/bin/bash", cmd...)
	}

	proc_stdin, _ := c.StdinPipe()
	proc_out, _ := c.StdoutPipe()
	var out_buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(&out_buf, proc_out)
		if err != nil {
			Glogger.Errorf("sync stdout to buffer error: %v\n", err)
			return
		}
	}()

	Glogger.Println("check for the result: ", c.Args)

	proc_stdin.Write(result)
	proc_stdin.Close()

	err := c.Start()
	if err != nil {
		Glogger.Errorf("check result error1: %v\n", err)
		return ""
	}

	if err = c.Wait(); err != nil {
		Glogger.Errorf("check result error2: %v\n", err)
		return ""
	}
	wg.Wait()

	return out_buf.String()
}

func check_macos_cmd(result []byte, cmd ...string) string {
	v := make([]interface{}, len(cmd))
	for i, s := range cmd {
		v[i] = s
	}

	arg := fmt.Sprintln(v)
	c := exec.Command("zsh", "-c", arg)

	proc_stdin, _ := c.StdinPipe()
	proc_out, _ := c.StdoutPipe()
	var out_buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(&out_buf, proc_out)
		if err != nil {
			Glogger.Errorf("sync stdout to buffer error: %v\n", err)
			return
		}
	}()

	Glogger.Println("check for the result: ", c.Args)

	proc_stdin.Write(result)
	proc_stdin.Close()

	err := c.Start()
	if err != nil {
		Glogger.Errorf("check result error1: %v\n", err)
		return ""
	}

	if err = c.Wait(); err != nil {
		Glogger.Errorf("check result error2: %v\n", err)
		return ""
	}
	wg.Wait()

	return out_buf.String()
}

func set_osenv_from_check(result string) bool {
	if len(result) == 0 {
		return false
	}

	reader := strings.NewReader(result)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		l := scanner.Text()
		i := strings.SplitN(l, "=", 2)
		if len(i) == 2 {
			k := strings.TrimSpace(i[0])
			v := strings.TrimSpace(i[1])
			fmt.Printf("set os envinmont:--%s:%s\n", k, v)
			if k == "RETRY" || k == "MARUDE_RETRY" {
				if v == "0" {
					return false
				}
			}
			os.Setenv(k, v)
		}
	}
	return true
}

func check_result(cfg *CfgCase, proc *RunStatus) {
	if len(cfg.Checkcmd) == 0 {
		Glogger.Infof("ignore check result process\n")
		return
	}

	args, _ := shlex.Split(cfg.Checkcmd)
	result := proc.rb.CheckResult()
	Glogger.Infof("case output length: %d\n", len(result))

	var r string = ""
	switch runtime.GOOS {
	case "linux":
		r = check_linux_cmd(result, args...)

	case "windows":
		r = check_win_cmd(result, args...)

	case "darwin":
		r = check_macos_cmd(result, args...)
	}

	if len(r) != 0 {
		if set_osenv_from_check(r) {
			Glogger.Infof("check result is not finished, re-run the command: %s\n", cfg.Exec)
			run_cmd(cfg, proc)
		}
	}
}

func run_cmd(cfg *CfgCase, proc *RunStatus) (*exec.Cmd, error) {
	//proc.rb.Reset()
	args, err := shlex.Split(cfg.Exec)
	if err != nil {
		Glogger.Fatalf("Failed to split command: %v", err)
		return nil, err
	}

	var c *exec.Cmd = nil
	var outs CmdOut

	switch runtime.GOOS {
	case "linux":
		c, outs, err = run_linux_cmd(args...)
		if err != nil {
			return nil, err
		}

	case "windows":
		c, outs, err = run_win_cmd(args...)
		if err != nil {
			return nil, err
		}

	case "darwin":
		c, outs, err = run_macos_cmd(args...)
		if err != nil {
			return nil, err
		}
	}

	Glogger.Infof("run command: %s\n", cfg.Exec)

	go func() {
		reader := io.MultiReader(outs.out, outs.err)
		proc.rb.ReadFrom(reader)
		proc.rb.CloseWriter()
	}()

	br, _ := strconv.Atoi(cfg.Baud)
	mode := &serial.Mode{
		BaudRate: br,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	var uart serial.Port = nil
	if len(cfg.Uart) != 0 {
		uart, err = serial.Open(cfg.Uart, mode)
	}

	if err != nil {
		Glogger.Errorf("PID: %d start to get uart log failed: %v\n", c.Process.Pid, err)
	}

	go func() {
		if uart == nil {
			return
		}

		fname := fmt.Sprintf(cfg.UartLogName, time.Now().Format("2006-01-02 15:04:05"))
		f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			Glogger.Errorf("PID: %d create uart log failed: %v\n", c.Process.Pid, err)
			return
		}
		defer f.Close()

		reader := bufio.NewScanner(uart)
		for reader.Scan() {
			ts := time.Now().Format("2006-01-02 15:04:05.000")
			l := fmt.Sprintf("[%s] %s\n", ts, reader.Text())
			if _, err = f.WriteString(l); err != nil {
				Glogger.Errorf("PID %d: write log failed: %v\n", c.Process.Pid, err)
				return
			}
		}
	}()

	Glogger.Infof("PID: %d command: %s is running\n", c.Process.Pid, cfg.Exec)

	go func() {
		err := c.Wait()
		if err != nil {
			fmt.Println(err)
		}
		proc.status = Finished
		proc.c = nil
		if uart != nil {
			uart.Close()
		}
		Glogger.Infof("PID: %d has been finished\n", c.Process.Pid)
		check_result(cfg, proc)
	}()

	proc.status = Running
	proc.c = c
	return c, nil
}
