package service

import (
	"net/http"
	"time"
)

type LayerAdminClient struct {
	baseURL string
	client *http.Client
}

func NewLayerAdminClient(baseURL string) *LayerAdminClient {
	return &LayerAdminClient{
		baseURL:baseURL,
		client:&http.Client{Timeout:2*time.Second},
	}
}