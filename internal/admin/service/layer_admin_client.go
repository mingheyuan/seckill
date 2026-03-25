package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
	"io"

	"seckill/internal/common/model"
)

type LayerAdminClient struct {
	baseURL string
	client *http.Client
}

func (c *LayerAdminClient) GetActivity() (model.ActivityConfig,error) {
	var out model.ActivityConfig

	resp,err :=c.client.Get(c.baseURL+"/internal/admin/activity")
	if err !=nil {
		return out,err
	}
	defer resp.Body.Close()

	if resp.StatusCode !=http.StatusOK {
		_,_=io.ReadAll(resp.Body)
		return out,io.ErrUnexpectedEOF
	}
	
	err =json.NewDecoder(resp.Body).Decode(&out)
	return out,err
}

func (c *LayerAdminClient) UpdateActivity(cfg model.ActivityConfig) error {
	b,_ :=json.Marshal(cfg)
	resp,err :=c.client.Post(c.baseURL+"/internal/admin/activity","application/json",bytes.NewReader(b))
	if err !=nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode !=http.StatusOK {
		return io.ErrUnexpectedEOF
	}
	return nil
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