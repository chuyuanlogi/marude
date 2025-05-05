package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"doraemon.pocket/common"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/smallnest/ringbuffer"
	"github.com/spf13/cobra"
)

const CONFIG_FILE string = "marude.conf"

type ClientDevice struct {
	Ip     string
	Serial string
}

type DeClient struct {
	Ip      string
	Port    string
	RingBuf *ringbuffer.RingBuffer
	Dev     []ClientDevice
}

var Glogger *logrus.Logger
var Version = "debug"

type readonly struct{ io.Reader }

func (readonly) Close() error { return nil }
func removeCloseMethod(rc io.ReadCloser) io.Reader {
	return readonly{rc}
}

func validate_ip(addr string) bool {
	if len(addr) == 0 {
		Glogger.Infof("ip address is empty")
		return false
	}

	var PatternIp = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$`)

	return PatternIp.MatchString(addr)
}

func queries(c *fiber.Ctx) (name string, client *DeClient, err error) {
	params := c.Queries()
	Glogger.Infof("remote connection parameters: %v\n", params)

	client = &DeClient{
		Ip:      params["ip"],
		Port:    params["port"],
		Dev:     []ClientDevice{},
		RingBuf: ringbuffer.New(256 * 1024).SetBlocking(true),
	}
	name = params["name"]

	if len(name) == 0 {
		Glogger.Infof("empty name is not allowed\n")
		return "", nil, fmt.Errorf("empty name is not allowed\n")
	}

	if !validate_ip(client.Ip) {
		return "", nil, fmt.Errorf("invalid ip address is not allowed\n")
	}

	if len(client.Port) == 0 {
		client.Port = "25305"
	}

	devices := c.Context().QueryArgs().PeekMulti("device")
	devices_ip := c.Context().QueryArgs().PeekMulti("device_ip")

	for i, _ := range devices {
		if !validate_ip(string(devices_ip[i])) {
			return "", nil, fmt.Errorf("invalid dev ip address is not allowed\n")
		}

		client.Dev = append(client.Dev, ClientDevice{Ip: string(devices_ip[i]), Serial: string(devices[i])})
	}

	Glogger.Infof("get req parameters: %v\n", client)
	return name, client, nil
}

func main() {
	var show_version bool = false
	argparse := &cobra.Command{
		Use:   "marude_server [--version]",
		Short: "run long time stress test tool server",
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

	clients := make(map[string]*DeClient)

	app := fiber.New()

	app.Get("/register", func(c *fiber.Ctx) error {
		name, client, err := queries(c)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintln(err))
		}

		_, ok := clients[name]
		var res string = "Register Success!\n"
		if !ok {
			Glogger.Infof("new register: %v\n", client)
			clients[name] = client

		} else {
			res = "Registered!!\n"
		}

		return c.Status(200).SendString(res)
	})

	app.Get("/update", func(c *fiber.Ctx) error {
		name, client, err := queries(c)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintln(err))
		}

		_, ok := clients[name]
		if !ok {
			return c.Status(400).SendString("client is not registered!\n")
		}

		clients[name] = client

		return c.Status(200).SendString("Update Success!\n")
	})

	app.Get("/delete", func(c *fiber.Ctx) error {
		params := c.Queries()

		v, ok := params["name"]
		if !ok {
			return c.Status(400).SendString("client is not registered!\n")
		}

		delete(clients, v)

		return c.Status(200).SendString(fmt.Sprintf("delete client %s success!\n", v))
	})

	app.Get("/list", func(c *fiber.Ctx) error {
		var res string = ""
		for k, v := range clients {
			res = res + fmt.Sprintf("client: %s, ip: %s:%s\n", k, v.Ip, v.Port)

			client_request(v, ReqClient{
				client_name:   k,
				method_params: []string{"list"},
			})

			buf := make([]byte, 256)
			leng, _ := v.RingBuf.Read(buf)
			res_str := strings.ReplaceAll(string(buf[:leng]), "\r\n", "\n")
			res_strlist := strings.Split(res_str, "\n")

			fmt.Printf("%v\n", res_strlist)

			for i, d := range v.Dev {
				casename := res_strlist[i][6:]
				res = res + fmt.Sprintf("\tcase: %s -- device: %s, device ip: %s\n", casename, d.Serial, d.Ip)
				client_request(v, ReqClient{
					client_name:   k,
					method_params: []string{"status", casename},
				})

				leng, _ = v.RingBuf.Read(buf)
				res = fmt.Sprintf("%s-----------------------\n%s********************\n", res, string(buf[:leng]))
			}
		}

		return c.Status(200).SendString(res)
	})

	app.Get("/run_case", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Transfer-Encoding", "chunked")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		params := c.Queries()

		v, ok := params["name"]
		if !ok {
			return c.Status(400).SendString(fmt.Sprintf("client [%v] is not registered\n", v))
		}

		rc, ok := params["case"]
		if !ok {
			return c.Status(400).SendString(fmt.Sprintf("case [%v] is not registered\n", rc))
		}

		gr, ok := params["fetch"]
		if !ok {
			gr = "0"
		}

		client, ok := clients[v]
		if !ok {
			return c.Status(400).SendString(fmt.Sprintf("fetch result %s -- %s failed\n", v, rc))
		}

		if gr == "1" {
			_, err = client_request(client, ReqClient{
				client_name:   v,
				method_params: []string{"resume", rc},
			})

			if err != nil {
				return c.Status(400).SendString(fmt.Sprintf("run %s -- %s failed, %v\n", v, rc, err))
			}
			reader := removeCloseMethod(client.RingBuf.ReadCloser())
			return c.SendStream(reader)
		} else if gr == "2" {
			_, err := client_request(client, ReqClient{
				client_name:   v,
				method_params: []string{"peek", rc},
			})
			if err != nil {
				return c.Status(400).SendString(fmt.Sprintf("peek %s -- %s failed, %v\n", v, rc, err))
			}

			leng := client.RingBuf.Length()
			data := make([]byte, leng)
			client.RingBuf.Peek(data)
			return c.SendString(string(data))
		} else {
			_, err := client_request(client, ReqClient{
				client_name:   v,
				method_params: []string{"run", rc},
			})

			if err != nil {
				return c.Status(400).SendString(fmt.Sprintf("run %s -- %s failed, %v\n", v, rc, err))
			}

			return c.Status(200).SendString(fmt.Sprintf("run %s -- %s Success!\n", v, rc))
		}
		return c.Status(400).SendString("Orz!\n")
	})

	Glogger.Fatal(app.Listen(fmt.Sprintf(":%s", cfg.Service.Port)))

}
