package main

import (
	"fmt"
	"net/http"
	"io"
)

type ReqClient struct {
	client_name		string
	method_params	[]string
}

func client_request(client *DeClient, req_client ReqClient) (err error) {
	Url := fmt.Sprintf("http://%s:%s", client.Ip, client.Port)

	if len(req_client.method_params) <= 0 {
		return fmt.Errorf("invalid http request method\n")
	} else if len(req_client.method_params) == 1 {
		Url = fmt.Sprintf("%s/%s", Url, req_client.method_params[0])
	} else if len(req_client.method_params) > 1 {
		Url = fmt.Sprintf("%s/%s/%s", Url, req_client.method_params[0], req_client.method_params[1])
	}

	req, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		Glogger.Errorf("http request failed: %v\n", err)
		return err
	}

	httpclient := &http.Client{
		/*Timeout: 100 * time.Second,*/
	}
	res, _ := httpclient.Do(req)

	client.RingBuf.Reset()

	if req_client.method_params[0] == "run" || req_client.method_params[0] == "resume" {
		go func() {
			//fmt.Printf("getting res from http req: %s\n", Url)
			client.RingBuf.ReadFrom(res.Body)
			//fmt.Printf("finished http req: %s\n", Url)
			client.RingBuf.CloseWriter()
			
		}()
	} else {
		b, _ := io.ReadAll(res.Body)
		client.RingBuf.Write(b)
	}

	fmt.Printf("req: %s finished\n", Url)
	return nil
}