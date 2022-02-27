package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
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

const (
	PORT           = ":8080"
	GENSRTDIR      = "/gen_cert.sh"
	CERTKEY        = "cert.key"
	SUCCESSCONNECT = "HTTP/1.1 200 Connection established\r\n\r\n"
)

var (
	ErrHijackingNotSupported = errors.New("hijacking not supported")
)

func getCert(host string) (tls.Certificate, error) {
	rootDir, _ := os.Getwd()
	_, err := os.Stat(fmt.Sprintf("%s/certs/%s.crt", rootDir, host))
	log.Println(fmt.Sprintf("%s/certs/%s.crt", rootDir, host))
	if os.IsNotExist(err) {
		genCommand := exec.Command(rootDir+GENSRTDIR, host, strconv.Itoa(rand.Intn(100000)))
		_, err = genCommand.CombinedOutput()
		if err != nil {
			return tls.Certificate{}, err
		}
	}

	tlsCert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/certs/%s.crt", rootDir, host), CERTKEY)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tlsCert, nil
}

func connectHandle(w http.ResponseWriter) (net.Conn, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, ErrHijackingNotSupported
	}

	httpsConn, _, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	_, err = httpsConn.Write([]byte(SUCCESSCONNECT))
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
		log.Println(err)
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

func secureHandle(w http.ResponseWriter, r *http.Request) {
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
		Addr: PORT,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				secureHandle(w, r)
			} else {
				httpHandle(w, r)
			}
		}),
	}

	log.Fatal(server.ListenAndServe())
}
