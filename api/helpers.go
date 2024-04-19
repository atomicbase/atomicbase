package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/joe-ervin05/atomicbase/daos"
)

type DbHandler func(db daos.Database, req *http.Request) ([]byte, error)

type PrimaryHandler func(db daos.PrimaryDao, req *http.Request) ([]byte, error)

func withPrimary(handler PrimaryHandler) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		dao, err := daos.ConnPrimary()

		req.Body = http.MaxBytesReader(wr, req.Body, 1048576)
		if err != nil {
			respErr(wr, err)
			return
		}

		data, err := handler(dao, req)
		if err != nil {
			respErr(wr, err)
			return
		}

		wr.Write(data)
		defer dao.Client.Close()
		defer req.Body.Close()

	}
}

// for endpoints that can use either the primary or an external database
func withDB(handler DbHandler) http.HandlerFunc {
	return func(wr http.ResponseWriter, req *http.Request) {
		dao, err := connDb(req)

		req.Body = http.MaxBytesReader(wr, req.Body, 1048576)
		if err != nil {
			respErr(wr, err)
			return
		}

		data, err := handler(dao, req)
		if err != nil {
			respErr(wr, err)
			return
		}

		wr.Write(data)
		defer dao.Client.Close()
		defer req.Body.Close()

	}
}

func respErr(wr http.ResponseWriter, err error) {
	wr.WriteHeader(http.StatusInternalServerError)
	wr.Write([]byte(err.Error()))
}

func connDb(req *http.Request) (daos.Database, error) {
	dbName := req.Header.Get("DB-Name")

	dao, err := daos.ConnPrimary()
	if err != nil {
		return daos.Database{}, err
	}

	if dbName != "" {
		db, err := dao.ConnTurso(dbName)
		if err != nil {
			return daos.Database{}, err
		}

		dao.Client.Close()
		dao.Database = db
	}

	return dao.Database, nil

}

func Request(method, url string, headers map[string]string, body any) (*http.Response, error) {
	client := &http.Client{}
	var req *http.Request
	var err error

	if body != nil {
		var buf bytes.Buffer

		err = json.NewEncoder(&buf).Encode(body)
		if err != nil {
			return nil, err
		}

		req, err = http.NewRequest(method, url, &buf)
		if err != nil {
			return nil, err
		}
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}
	}

	for name, val := range headers {
		req.Header.Add(name, val)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		bod, err := io.ReadAll(res.Body)
		if err != nil {
			return res, err
		}

		if bod == nil {
			return res, errors.New(res.Status)
		}
		return res, errors.New(string(bod))
	}

	return res, nil
}
