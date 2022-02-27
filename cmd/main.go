package main

import (
	"database/sql"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"httpproxy/internal/proxy/delivery"
	"httpproxy/internal/proxy/repositiory"
)

func main() {
	con, err := sql.Open("postgres",
		"host=localhost port=5432 user=ilyagu dbname=proxy password=password sslmode=disable")

	if err != nil {
		logrus.Fatal(err)
	}
	err = con.Ping()
	if err != nil {
		logrus.Fatal(err)
	}

	rep := repositiory.NewDB(con)
	proxy := delivery.NewProxy(rep)

	proxyServer := http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(proxy.HandleProxyRequest),
	}

	go func() {
		err = proxyServer.ListenAndServe()
		logrus.Fatal(err)
	}()

	api := delivery.NewApi(rep, proxy)

	apiServer := http.Server{
		Addr:    ":8000",
		Handler: api,
	}

	err = apiServer.ListenAndServe()

	if err != nil {
		logrus.Error(err)
	}
}
