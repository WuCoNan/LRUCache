package mycache

import (
	"context"
	"log"

	"github.com/golang/groupcache/consistenthash"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type peer interface {
	Get(group string, key string) ([]byte, bool)
	Set(group string, key string, value []byte) bool
	Delete(group string, key string) bool
}
type peerPicker interface {
	PickPeer(key string) peer
}
type ClientPicker struct {
	selfAddr       string
	serviceName    string
	clients        map[string]*Client
	consistentHash *consistenthash.Map
	etcdClient     *clientv3.Client
}

func NewClientPicker(selfAddr string, serviceName string, etcdClient *clientv3.Client) *ClientPicker {

	cp := &ClientPicker{
		selfAddr:       selfAddr,
		serviceName:    serviceName,
		clients:        make(map[string]*Client),
		consistentHash: consistenthash.New(50, nil),
		etcdClient:     etcdClient,
	}
	cp.startServiceDiscovery()
	return cp
}
func (cp *ClientPicker) startServiceDiscovery() {
	cp.fetchAllService()
	go cp.watchServiceChanges()
}
func (cp *ClientPicker) fetchAllService() {
	resp, err := cp.etcdClient.Get(context.Background(), "/mycache/"+cp.serviceName, clientv3.WithPrefix())
	if err != nil {
		log.Fatalf("fetch service from etcd error:%v", err)
	}
	for _, kv := range resp.Kvs {
		addr := string(kv.Value)
		if addr != cp.selfAddr && cp.clients[addr] == nil {
			cp.clients[addr] = NewClient(addr, cp.serviceName)
			cp.consistentHash.Add(addr)
		}
	}
}
func (cp *ClientPicker) watchServiceChanges() {
	watchChan := cp.etcdClient.Watch(context.Background(), "/mycache/"+cp.serviceName, clientv3.WithPrefix())
	for {
		select {
		case resp := <-watchChan:
			cp.handleServiceChange(resp.Events)
		}
	}
}
func (cp *ClientPicker) handleServiceChange(events []*clientv3.Event) {
	for _, ev := range events {
		addr := string(ev.Kv.Value)
		switch ev.Type {
		case clientv3.EventTypePut:
			if addr != cp.selfAddr && cp.clients[addr] == nil {
				cp.clients[addr] = NewClient(addr, cp.serviceName)
				cp.consistentHash.Add(addr)
			}
		case clientv3.EventTypeDelete:
			if cp.clients[addr] != nil {
				cp.clients[addr].Close()

			}
		}
	}
}
func (cp *ClientPicker) PickPeer(key string) peer {
	addr := cp.consistentHash.Get(key)
	if addr == "" || addr == cp.selfAddr {
		return nil
	}
	return cp.clients[addr]
}
