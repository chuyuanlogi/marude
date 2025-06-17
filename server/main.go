package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"

	"doraemon.pocket/common"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
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

func prefix_detect(name string) bool {
	prefix := []string{}
	switch runtime.GOOS {
	case "linux":
		prefix = []string{"en", "eth"}
	case "windows":
		prefix = []string{"ethernet"}
	case "darwin":
		prefix = []string{"en0"}
	}

	for _, pfx := range prefix {
		if strings.HasPrefix(name, pfx) {
			return true
		}
	}

	return false
}

func get_ip() string {
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
			if ip.IsLoopback() || ip.To4() == nil || ip.String() == "127.0.0.1" {
				continue
			}

			if prefix_detect(ifname) {
				return ip.String()
			}
		}
	}

	return ""
}

type Machine struct {
	Client_info string
	Client_case string
	Client_prog string
}

const const_display_temp = "<a href=\"/display\" target=\"_blank\" class=\"pure-text-res\" " +
	"data-res-link=\"%s\">%s</a><br>"
const const_case = const_display_temp + "%s - %s"

const const_caselink = "http://%s:%s/run_case?name=%s&case=%s&fetch=0"

const const_status = const_display_temp

const const_statuslink = "http://%s:%s/run_case?name=%s&case=%s&fetch=3"

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

	engine := html.New("./view", ".html")
	engine.AddFunc(
		"unescape", func(s string) template.HTML {
			return template.HTML(s)
		},
	)

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Get("/", func(c *fiber.Ctx) error {
		machines := []Machine{}
		uni_id := 1

		for k, v := range clients {
			client_request(v, ReqClient{
				client_name:   k,
				method_params: []string{"list"},
			})

			buf := make([]byte, 256)
			leng, _ := v.RingBuf.Read(buf)
			res_str := strings.ReplaceAll(string(buf[:leng]), "\r\n", "\n")
			res_strlist := strings.Split(res_str, "\n")

			for i, d := range v.Dev {
				casename := res_strlist[i][6:]

				m := Machine{}
				m.Client_info = fmt.Sprintf("%s<br>%s:%s", k, v.Ip, v.Port)
				caselink := fmt.Sprintf(const_caselink, get_ip(), cfg.Service.Port, k, casename)
				m.Client_case =
					fmt.Sprintf(const_case,
						caselink, casename,
						d.Serial, d.Ip)

				client_request(v, ReqClient{
					client_name:   k,
					method_params: []string{"status", casename},
				})

				leng, _ = v.RingBuf.Read(buf)
				res_info := strings.ReplaceAll(string(buf[:leng]), "\r\n", "\n")
				res_infolist := strings.Split(res_info, "\n")
				if res_infolist[0][8:] == "Idle" {
					m.Client_prog = "Idle"
				} else {
					statuslink := fmt.Sprintf(const_statuslink, get_ip(), cfg.Service.Port, k, casename)
					m.Client_prog =
						fmt.Sprintf(const_status,
							statuslink, res_infolist[0][8:])
				}
				uni_id++
				machines = append(machines, m)
			}
		}

		data := fiber.Map{
			"Machines": machines,
		}
		return c.Render("index", data)
	})

	app.Get("/display", func(c *fiber.Ctx) error {
		return c.Render("display", nil)
	})

	app.Get("/proxy", func(c *fiber.Ctx) error {
		params := c.Queries()
		link := params["link"]
		if len(link) == 0 {
			return c.Status(fiber.StatusBadRequest).SendString("link infomation is lost")
		}

		parsedlink, err := url.Parse(link)
		if err != nil || (parsedlink.Scheme != "http") {
			return c.Status(fiber.StatusBadRequest).SendString("invalid link informat")
		}

		res, err := http.Get(link)
		if err != nil {
			Glogger.Errorf("get client %s status failed %v\n", link, err)
			return c.Status(fiber.StatusInternalServerError).SendString("failed to get status")
		}

		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			Glogger.Errorf("get %s status code %v\n", link, err)
			return c.Status(res.StatusCode).SendString("failed to get status")
		}

		c.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
		maxleng := int(res.ContentLength)
		buf := make([]byte, maxleng)
		if maxleng > 0 {
			_, err := io.ReadFull(res.Body, buf)
			if err != nil && err != io.EOF {
				Glogger.Errorf("read http res body failed: %v\n", err)
				return c.Status(res.StatusCode).SendString("failed to get status")
			}
		}
		return c.SendString(string(buf[:maxleng]))
	})

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
		} else if gr == "3" {
			_, err := client_request(client, ReqClient{
				client_name:   v,
				method_params: []string{"peeek", rc},
			})
			if err != nil {
				return c.Status(400).SendString(fmt.Sprintf("peek long %s -- %s failed, %v\n", v, rc, err))
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
