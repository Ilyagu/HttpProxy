package delivery

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"httpproxy/internal/proxy/repositiory"
)

const (
	xxe                 = "<!DOCTYPE foo [\n  <!ELEMENT foo ANY >\n  <!ENTITY xxe SYSTEM \"file:///etc/passwd\" >]>\n<foo>&xxe;</foo>\n"
	xml                 = "<?xml"
	target              = "root:"
	requestIsVulnerable = "request is vulnerable!!!"
)

type Api struct {
	rep    *repositiory.DB
	proxy  *Proxy
	router *mux.Router
}

func NewApi(rep *repositiory.DB, proxy *Proxy) *Api {

	router := mux.NewRouter()

	api := &Api{
		rep:    rep,
		proxy:  proxy,
		router: router,
	}

	router.HandleFunc("/repeat/{request_id:[0-9]+}", api.RepeatRequest)
	router.HandleFunc("/requests/{request_id:[0-9]+}", api.GetRequest)
	router.HandleFunc("/requests", api.GetAllRequests)
	router.HandleFunc("/scan/{request_id:[0-9]+}", api.VulnerabilityScan)

	return api
}

func (api *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.router.ServeHTTP(w, r)
}

func (api *Api) RepeatRequest(w http.ResponseWriter, r *http.Request) {
	requestID, err := strconv.Atoi(mux.Vars(r)["request_id"])
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	request, err := api.rep.GetRequest(requestID)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	req := &http.Request{
		Method: request.Method,
		URL: &url.URL{
			Scheme: request.Scheme,
			Host:   request.Host,
			Path:   request.Path,
		},
		Body:   ioutil.NopCloser(strings.NewReader(request.Body)),
		Host:   request.Host,
		Header: request.Headers,
	}

	if request.Scheme == "" {
		api.proxy.SecureHandle(w, req, true)
	} else {
		api.proxy.HttpHandle(w, req)
	}
}

func (api *Api) GetRequest(w http.ResponseWriter, r *http.Request) {
	requestID, err := strconv.Atoi(mux.Vars(r)["request_id"])
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	request, err := api.rep.GetRequest(requestID)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	bytes, err := json.Marshal(request)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Write(bytes)
}

func (api *Api) GetAllRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := api.rep.GetAllRequests()
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	for _, request := range requests {
		bytes, err := json.Marshal(request)
		if err != nil {
			logrus.Error(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Write(bytes)
		w.Write([]byte("\n\n"))
	}
}

func (api *Api) VulnerabilityScan(w http.ResponseWriter, r *http.Request) {
	requestID, err := strconv.Atoi(mux.Vars(r)["request_id"])
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	request, err := api.rep.GetRequest(requestID)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if ind := strings.Index(request.Body, xml); ind != -1 {
		request.Body = request.Body[:ind] + xxe
	}
	body := bytes.NewBufferString(request.Body)

	if request.Scheme == "" {
		request.Scheme = "https"
	}
	urlStr := request.Scheme + "://" + request.Host + request.Path
	req, err := http.NewRequest(request.Method, urlStr, body)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for key, value := range request.Headers {
		req.Header.Add(key, strings.Join(value, " "))
	}

	response, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()

	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	bytes, err := json.Marshal(response.Body)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Write(bytes)
}
