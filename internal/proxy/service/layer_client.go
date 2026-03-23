package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"seckill/internal/common/model"
)

type LayerClient struct {
	baseURL string
	client 	*http.Client
}

func NewLayerClient(baseURL string) *LayerClient {
	return &LayerClient{
		baseURL:baseURL,
		client:&http.Client{
			Timeout:2*time.Second,
		},
	}
}

func (lc *LayerClient) Seckill(req model.SeckillRequest) (model.InternalSeckillResponse,error) {
	var out model.InternalSeckillResponse

	b,_:=json.Marshal(req)
	resp,err :=lc.client.Post(lc.baseURL+"/internal/seckill","application/json",bytes.NewReader(b))
	if err !=nil {
		return out,err
	}
	defer resp.Body.Close()

	err =json.NewDecoder(resp.Body).Decode(&out)
	return out,err
}