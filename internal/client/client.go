package client

import "github.com/nickqweaver/weave-queue/internal/store"

type Client struct {
	producer *Producer
}

func NewClient(s store.Store) *Client {
	p := NewProducer(s)
	return &Client{
		producer: p,
	}
}

func (c *Client) Enqueue(queue string, id string) {
	c.producer.Enqueue(queue, id)
}
