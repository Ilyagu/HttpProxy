package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	BUFFERLENGTH = 256
)

type HttpRequest struct {
	Method      string
	Schema      string
	HostAndPort string
	Path        string
	Headers     string
	Body        string
}

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		if err != nil {
			log.Fatal(err)
		}

		go proxyHandler(conn)
	}
}

func proxyHandler(conn net.Conn) {
	var request []byte
	requestLength := 0
	for {
		resHeaderBytes := make([]byte, BUFFERLENGTH)
		numberOfBytes, err := conn.Read(resHeaderBytes)
		if err != nil {
			log.Fatal(err)
		}

		requestLength += numberOfBytes
		request = append(request, resHeaderBytes...)

		if numberOfBytes < BUFFERLENGTH {
			request = request[:requestLength]
			break
		}
	}

	httpRequest, _ := parseHttpRequest(string(request))

	response := mainHandler(httpRequest)

	fmt.Println(string(response))
	_, err := conn.Write([]byte(response))
	if err != nil {
		log.Fatal(err)
	}

	conn.Close()
}

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func parseHttpRequest(request string) (HttpRequest, error) {
	var httpRequest HttpRequest
	requestHeaderAndBody := strings.Split(request, "\r\n\r\n")

	header := requestHeaderAndBody[0]

	headerStrs := strings.Split(header, "\r\n")

	firstStr := strings.Split(headerStrs[0], " ")
	fmt.Println(firstStr)
	schemaHostPath := strings.Split(firstStr[1], "/")
	fmt.Println(schemaHostPath)

	headers := headerStrs[2:]

	var headersNoProxy []string
	for idx, lol := range headers {
		if strings.Contains(lol, "Proxy-Connection") {
			headersNoProxy = remove(headers, idx)
			break
		}
	}

	httpRequest.Body = requestHeaderAndBody[1]
	httpRequest.Method = firstStr[0]
	httpRequest.Schema = schemaHostPath[0]
	httpRequest.HostAndPort = schemaHostPath[2]
	httpRequest.Path = "/" + schemaHostPath[3]
	httpRequest.Headers = strings.Join(headersNoProxy, "\r\n")

	return httpRequest, nil
}

func mainHandler(httpRequest HttpRequest) string {
	var port string
	hostAndPort := strings.Split(httpRequest.HostAndPort, ":")
	host := hostAndPort[0]
	if len(hostAndPort) > 1 {
		port = ":" + hostAndPort[1]
	} else {
		port = ":80"
	}
	connProxy, err := net.Dial("tcp", host+port)
	if err != nil {
		log.Fatal(err)
	}
	defer connProxy.Close()

	requestOptions := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\n", httpRequest.Method, httpRequest.Path, httpRequest.HostAndPort)

	request := requestOptions + httpRequest.Headers + "\r\n\r\n" + httpRequest.Body

	fmt.Println(string(request))
	_, err = connProxy.Write([]byte(request))
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(time.Duration(100) * time.Millisecond)
	var response []byte
	for {
		resHeaderBytes := make([]byte, BUFFERLENGTH)
		numberOfBytes, err := connProxy.Read(resHeaderBytes)
		if err != nil {
			log.Fatal(err)
		}

		response = append(response, resHeaderBytes...)

		if numberOfBytes < BUFFERLENGTH {
			break
		}
	}

	return string(response)
}
