package main

import (
	"fmt"
	"io"
	"net/http"
)

type ReqClient struct {
	client_name   string
	method_params []string
}

func client_request(client *DeClient, req_client ReqClient) (res *http.Response, err error) {
	Url := fmt.Sprintf("http://%s:%s", client.Ip, client.Port)

	if len(req_client.method_params) <= 0 {
		return nil, fmt.Errorf("invalid http request method\n")
	} else if len(req_client.method_params) == 1 {
		Url = fmt.Sprintf("%s/%s", Url, req_client.method_params[0])
	} else if len(req_client.method_params) > 1 {
		Url = fmt.Sprintf("%s/%s/%s", Url, req_client.method_params[0], req_client.method_params[1])
	}

	req, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		Glogger.Errorf("http request failed: %v\n", err)
		return nil, err
	}

	httpclient := &http.Client{
		/*Timeout: 100 * time.Second,*/
	}
	res, err = httpclient.Do(req)
	if err != nil {
		Glogger.Errorf("http request failed: %v\n", err)
		return nil, err
	}

	client.RingBuf.Reset()

	if req_client.method_params[0] == "run" || req_client.method_params[0] == "resume" {
		go func() {
			defer res.Body.Close()
			//fmt.Printf("getting res from http req: %s\n", Url)
			client.RingBuf.ReadFrom(res.Body)
			//fmt.Printf("finished http req: %s\n", Url)
			client.RingBuf.CloseWriter()
		}()
	} else {
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		client.RingBuf.Write(b)
		return nil, nil
	}

	fmt.Printf("req: %s finished\n", Url)
	return res, nil
}
