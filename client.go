package mycache

import (
	"context"
	"log"
	"time"

	pb "github.com/WuCoNan/MyCache/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	addr        string
	serviceName string
	conn        *grpc.ClientConn
	rpcClient   pb.MyCacheClient
}

func NewClient(addr string, serviceName string) *Client {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to addr:%s error:%v", addr, err)
	}

	c := &Client{
		addr:        addr,
		serviceName: serviceName,
		conn:        conn,
		rpcClient:   pb.NewMyCacheClient(conn),
	}

	return c
}

func (c *Client) Get(groupName string, key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.rpcClient.Get(ctx, &pb.GetRequest{
		Group: groupName,
		Key:   key,
	})

	if err != nil {
		log.Fatalf("rpc get error:%v\n", err)
	}

	return resp.GetValue(), resp.GetFlag()
}

func (c *Client) Set(groupName string, key string, value []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.rpcClient.Set(ctx, &pb.SetRequest{
		Group: groupName,
		Key:   key,
		Value: value,
	})
	if err != nil {
		log.Fatalf("rpc set error:%v\n", err)
	}

	return resp.GetFlag()
}

func (c *Client) Delete(groupName string, key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.rpcClient.Delete(ctx, &pb.DeleteRequest{
		Group: groupName,
		Key:   key,
	})
	if err != nil {
		log.Fatalf("rpc delete error:%v\n", err)
	}

	return resp.GetFlag()
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
