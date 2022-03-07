package repositiory

import (
	"database/sql"
	"encoding/json"

	"httpproxy/internal/proxy/models"
)

type DB struct {
	con *sql.DB
}

func NewDB(con *sql.DB) *DB {
	return &DB{con: con}
}

func (db *DB) SaveRequest(req *models.Request) error {
	headers, err := json.Marshal(req.Headers)
	if err != nil {
		return err
	}

	query := `insert into requests (method, scheme, host, path, headers, body)
			values ($1, $2, $3, $4, $5, $6) returning id`

	err = db.con.QueryRow(query,
		req.Method,
		req.Scheme,
		req.Host,
		req.Path,
		string(headers),
		req.Body,
	).Scan(&req.Id)
	return err
}

func (db *DB) GetAllRequests() ([]*models.Request, error) {
	query := `select id, method, scheme, host, path, headers, body from requests`

	row, err := db.con.Query(query)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	requests := make([]*models.Request, 0)

	for row.Next() {
		request := &models.Request{}
		b := make([]byte, 0)

		row.Scan(&request.Id, &request.Method, &request.Scheme, &request.Host, &request.Path,
			&b, &request.Body)

		json.Unmarshal(b, &request.Headers)

		requests = append(requests, request)
	}

	return requests, nil
}

func (db *DB) GetRequest(id int) (*models.Request, error) {
	query := `select id, method, scheme, host, path, headers, body from requests
			where id = $1`

	request := &models.Request{}
	b := make([]byte, 0)

	err := db.con.QueryRow(query, id).Scan(&request.Id, &request.Method, &request.Scheme, &request.Host,
		&request.Path, &b, &request.Body)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &request.Headers)
	return request, err
}
