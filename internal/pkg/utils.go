package pkg

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	GENSRTDIR      = "gen_cert.sh"
	CERTKEY        = "cert.key"
	SUCCESSCONNECT = "HTTP/1.1 200 Connection established\r\n\r\n"
)

var (
	ErrHijackingNotSupported = errors.New("hijacking not supported")
)

func getCert(host string) (tls.Certificate, error) {
	rootDir, _ := os.Getwd()
	_, err := os.Stat(fmt.Sprintf("%s/certs/%s.crt", rootDir, host))
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

func ConnectHandle(w http.ResponseWriter) (net.Conn, error) {
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

func CreateTcpClientWithTlsConfig(r *http.Request, httpsConn net.Conn) (*tls.Conn, *tls.Config, error) {
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
