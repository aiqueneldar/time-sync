package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/config"
)

func HealthCheck() (bool, error) {
	cfg := config.Load()
	var resp *http.Response
	var err error
	if cfg.TLSEnabled {
		resp, err = http.Get(fmt.Sprintf("https://localhost:%s/healthz", cfg.Port))
		if err != nil {
			return false, err
		}
	} else {
		resp, err = http.Get(fmt.Sprintf("http://localhost:%s/healthz", cfg.Port))
		if err != nil {
			return false, err
		}
	}

	read_body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if string(read_body) != "OK" {
		return false, nil
	}

	return true, nil
}
