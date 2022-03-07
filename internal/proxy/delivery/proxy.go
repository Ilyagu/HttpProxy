package delivery

import (
	"bufio"
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/sirupsen/logrus"

	"httpproxy/internal/proxy/models"
	"httpproxy/internal/proxy/repositiory"

	"httpproxy/internal/pkg"
)

type Proxy struct {
	rep *repositiory.DB
}

func NewProxy(rep *repositiory.DB) *Proxy {
	return &Proxy{
		rep: rep,
	}
}

func (p *Proxy) HandleProxyRequest(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)

	if r.URL.Scheme == "" {
		r.URL.Scheme = "https"
	}

	req := &models.Request{
		Method:  r.Method,
		Scheme:  r.URL.Scheme,
		Host:    r.Host,
		Path:    r.URL.Path,
		Headers: r.Header,
		Body:    string(body),
	}

	logrus.Info(req)

	err := p.rep.SaveRequest(req)
	if err != nil {
		logrus.Error(err)
	}

	if r.Method == http.MethodConnect {
		p.SecureHandle(w, r)
	} else {
		p.HttpHandle(w, r)
	}
	// _, err = p.HandleHTTPRequest(w, r)
	// if err != nil {
	// 	logrus.Info(err)
	// }
}

func (p *Proxy) SecureHandle(w http.ResponseWriter, r *http.Request) {
	var httpsConn net.Conn
	if r.URL.Scheme == "https" {
		httpsConn, err := pkg.ConnectHandle(w)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer httpsConn.Close()
	}

	tcpClient, tlsConfig, err := pkg.CreateTcpClientWithTlsConfig(r, httpsConn)
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

	err = p.ProxyHttpsRequest(tcpClient, tcpServer)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
}

func (p *Proxy) ProxyHttpsRequest(tcpClient *tls.Conn, tcpServer *tls.Conn) error {
	clientReader := bufio.NewReader(tcpClient)
	request, err := http.ReadRequest(clientReader)
	if err != nil {
		return err
	}

	body, _ := ioutil.ReadAll(request.Body)
	savingReq := &models.Request{
		Method:  request.Method,
		Scheme:  request.URL.Scheme,
		Host:    request.Host,
		Path:    request.URL.Path,
		Headers: request.Header,
		Body:    string(body),
	}

	p.rep.SaveRequest(savingReq)

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

func (p *Proxy) HttpHandle(w http.ResponseWriter, r *http.Request) {
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

	// decodedResponse, err := DecodeResponse(response)
	// log.Println(err)
	// log.Println(string(decodedResponse))
}

// func (p *Proxy) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) (string, error) {
// 	proxyResponse, err := p.DoHttpRequest(r)
// 	if err != nil {
// 		logrus.Info(err)
// 	}
// 	for header, values := range proxyResponse.Header {
// 		for _, value := range values {
// 			w.Header().Add(header, value)
// 		}
// 	}
// 	w.WriteHeader(proxyResponse.StatusCode)
// 	_, err = io.Copy(w, proxyResponse.Body)
// 	if err != nil {
// 		logrus.Info(err)
// 	}
// 	defer proxyResponse.Body.Close()

// 	decodedResponse, err := DecodeResponse(proxyResponse)
// 	if err != nil {
// 		return "", err
// 	}

// 	return string(decodedResponse), nil
// }

// func (p *Proxy) DoHttpRequest(r *http.Request) (*http.Response, error) {
// 	request, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	request.Header = r.Header

// 	proxyResponse, err := http.DefaultTransport.RoundTrip(request)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return proxyResponse, nil
// }

// func DecodeResponse(response *http.Response) ([]byte, error) {
// 	var body io.ReadCloser

// 	switch response.Header.Get("Content-Encoding") {
// 	case "gzip":
// 		var err error
// 		body, err = gzip.NewReader(response.Body)
// 		if err != nil {
// 			body = response.Body
// 		}
// 	default:
// 		body = response.Body
// 	}

// 	bodyByte, err := ioutil.ReadAll(body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	lineBreak := []byte("\n")
// 	bodyByte = append(bodyByte, lineBreak...)
// 	bodyByte = bodyByte[0:500]

// 	defer body.Close()

// 	return bodyByte, nil
// }
