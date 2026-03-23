package service

import (
	"bytes"
	"encoding/json"
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

func (c *LayerAdminClient) Init(activityID,stock int64) error {
	body := map[string]int64{
		"activity_id" 	:activityID,
		"stock":		stock,
	}
	b,_:=json.Marshal(body)
	resp,err:=c.client.Post(c.baseURL+"/internal/admin/init","application/json",bytes.NewReader(b))
	if err !=nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}