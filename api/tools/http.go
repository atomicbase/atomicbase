package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Request makes an HTTP request with optional JSON body and headers.
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

// ParseHeaderCommas splits comma-separated header values into individual strings.
func ParseHeaderCommas(strs []string) []string {
	out := make([]string, 0, len(strs))

	for _, s := range strs {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}

	return out
}
