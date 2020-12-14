package service

import (
	"encoding/base64"
	"log"
	"net/url"
	"path"
	"strings"

	"github.com/docker/libkv"
	kvstore "github.com/docker/libkv/store"
	"github.com/docker/libkv/store/zookeeper"
)

type ZooKeeperRegistry struct {
	kv kvstore.Store
}

func (r *ZooKeeperRegistry) initRegistry() {
	zookeeper.Register()

	if strings.HasPrefix(ServerConfig.ServiceBaseURL, "/") {
		ServerConfig.ServiceBaseURL = ServerConfig.ServiceBaseURL[1:]
	}

	if strings.HasSuffix(ServerConfig.ServiceBaseURL, "/") {
		ServerConfig.ServiceBaseURL = ServerConfig.ServiceBaseURL[0 : len(ServerConfig.ServiceBaseURL)-1]
	}

	kv, err := libkv.NewStore(kvstore.ZK, []string{ServerConfig.RegistryURL}, nil)
	if err != nil {
		log.Printf("cannot create etcd registry: %v", err)
		return
	}
	r.kv = kv

	return
}

func (r *ZooKeeperRegistry) FetchServices() []*Service {
	var services []*Service

	kvs, err := r.kv.List(ServerConfig.ServiceBaseURL)
	if err != nil {
		log.Printf("failed to list services %s: %v", ServerConfig.ServiceBaseURL, err)
		return services
	}

	for _, value := range kvs {
		serviceName := value.Key

		nodes, err := r.kv.List(ServerConfig.ServiceBaseURL + "/" + value.Key)
		if err != nil {
			log.Printf("failed to list  %s: %v", ServerConfig.ServiceBaseURL+"/"+value.Key, err)
			continue
		}

		for _, n := range nodes {
			var serviceAddr = n.Key

			v, err := url.ParseQuery(string(n.Value[:]))
			if err != nil {
				log.Println("etcd value parse failed. error: ", err.Error())
				continue
			}
			state := "n/a"
			group := ""
			if err == nil {
				state = v.Get("state")
				if state == "" {
					state = "active"
				}
				group = v.Get("group")
			}
			id := base64.StdEncoding.EncodeToString([]byte(serviceName + "@" + serviceAddr))
			service := &Service{ID: id, Name: serviceName, Address: serviceAddr, Metadata: string(n.Value[:]), State: state, Group: group}
			services = append(services, service)
		}

	}

	return services
}

func (r *ZooKeeperRegistry) DeactivateService(name, address string) error {
	key := path.Join(ServerConfig.ServiceBaseURL, name, address)

	kv, err := r.kv.Get(key)

	if err != nil {
		return err
	}

	v, err := url.ParseQuery(string(kv.Value[:]))
	if err != nil {
		log.Println("etcd value parse failed. err ", err.Error())
		return err
	}
	v.Set("state", "inactive")
	err = r.kv.Put(kv.Key, []byte(v.Encode()), &kvstore.WriteOptions{IsDir: false})
	if err != nil {
		log.Println("etcd set failed, err : ", err.Error())
	}

	return err
}

func (r *ZooKeeperRegistry) ActivateService(name, address string) error {
	key := path.Join(ServerConfig.ServiceBaseURL, name, address)
	kv, err := r.kv.Get(key)

	v, err := url.ParseQuery(string(kv.Value[:]))
	if err != nil {
		log.Println("etcd value parse failed. err ", err.Error())
		return err
	}
	v.Set("state", "active")
	err = r.kv.Put(kv.Key, []byte(v.Encode()), &kvstore.WriteOptions{IsDir: false})
	if err != nil {
		log.Println("etcdv3 put failed. err: ", err.Error())
	}

	return err
}

func (r *ZooKeeperRegistry) UpdateMetadata(name, address string, metadata string) error {
	key := path.Join(ServerConfig.ServiceBaseURL, name, address)
	err := r.kv.Put(key, []byte(metadata), &kvstore.WriteOptions{IsDir: false})
	return err
}
