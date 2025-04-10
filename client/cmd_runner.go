package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
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

	fmt.Println(c.Path, c.Args)

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

	fmt.Println(c.Path, c.Args)

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

	fmt.Println(c.Path, c.Args)

	err = c.Start()
	if err != nil {
		return nil, outs, err
	}

	return c, outs, nil
}

func run_cmd(cfg *CfgCase, proc *RunStatus) (*exec.Cmd, error) {
	proc.rb.Reset()
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

	go func() {
		proc.rb.ReadFrom(outs.out)
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
	}()

	proc.status = Running
	proc.c = c
	return c, nil
}
