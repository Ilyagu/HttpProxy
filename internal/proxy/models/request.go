package models

type Request struct {
	Id      int64
	Method  string
	Host    string
	Path    string
	Headers map[string][]string
	Body    string
}
