package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func getCert(host string) (tls.Certificate, error) {
	_, err := os.Stat("certs/" + host + ".crt")
	if os.IsNotExist(err) {
		genCommand := exec.Command("gen_cert.sh", host, strconv.Itoa(rand.Intn(1000)))
		_, err = genCommand.CombinedOutput()
		if err != nil {
			log.Println(err)
			return tls.Certificate{}, err
		}
	}

	tlsCert, err := tls.LoadX509KeyPair("certs/"+host+".crt", "cert.key")
	if err != nil {
		log.Println("error loading pair", err)
		return tls.Certificate{}, err
	}

	return tlsCert, nil
}

func connectHandle(w http.ResponseWriter) (net.Conn, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("Hijacking not supported")
	}

	httpsConn, _, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	_, err = httpsConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		httpsConn.Close()
		return nil, err
	}

	return httpsConn, nil
}

func createTcpClientWithTlsConfig(r *http.Request, httpsConn net.Conn) (*tls.Conn, *tls.Config, error) {
	host := strings.Split(r.Host, ":")[0]

	caCert, err := getCert(host)
	if err != nil {
		return nil, nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{caCert},
		ServerName:   r.URL.Scheme,
	}

	tcpClient := tls.Server(httpsConn, tlsConfig)
	err = tcpClient.Handshake()
	if err != nil {
		tcpClient.Close()
		return nil, nil, err
	}

	return tcpClient, tlsConfig, nil
}

func proxyHttpsRequest(tcpClient *tls.Conn, tcpServer *tls.Conn) error {
	clientReader := bufio.NewReader(tcpClient)
	request, err := http.ReadRequest(clientReader)
	if err != nil {
		return err
	}

	dumpRequest, err := httputil.DumpRequest(request, true)
	if err != nil {
		return err
	}
	_, err = tcpServer.Write(dumpRequest)
	if err != nil {
		return err
	}

	serverReader := bufio.NewReader(tcpServer)
	response, err := http.ReadResponse(serverReader, request)
	if err != nil {
		return err
	}

	rawResponse, err := httputil.DumpResponse(response, true)
	if err != nil {
		return err
	}

	_, err = tcpClient.Write(rawResponse)
	if err != nil {
		return err
	}

	return nil
}

func httpsHandle(w http.ResponseWriter, r *http.Request) {
	httpsConn, err := connectHandle(w)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer httpsConn.Close()

	tcpClient, tlsConfig, err := createTcpClientWithTlsConfig(r, httpsConn)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer tcpClient.Close()

	tcpServer, err := tls.Dial("tcp", r.URL.Host, tlsConfig)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer tcpServer.Close()

	err = proxyHttpsRequest(tcpClient, tcpServer)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
}

func httpHandle(w http.ResponseWriter, r *http.Request) {
	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, response.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	server := http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				httpsHandle(w, r)
			} else {
				httpHandle(w, r)
			}
		}),
	}

	log.Fatal(server.ListenAndServe())
}
