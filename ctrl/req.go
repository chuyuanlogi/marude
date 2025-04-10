package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

func HttpGet(url string, param url.Values) bool {
	var fullURL string = ""
	if len(param) != 0 {
		fullURL = url + "?" + param.Encode()
	} else {
		fullURL = url
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Fatalf("request failed: %v\n", err)
		return false
	}

	client := &http.Client{
		//Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("request http request failed: %v\n", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		log.Fatalf("http res code: %d\n%s\n", res.StatusCode, string(b[:]))
		return false
	}

	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Printf("get server data failed: %v\n", err)
			return false
		} else if err == io.EOF {
			break
		}
		fmt.Printf("%s", line)
	}

	return true
}
