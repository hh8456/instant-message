package config

import (
	"encoding/json"
	"os"
)

type IMConfig struct {
	Mysql string `json:"mysql"`
	Local struct {
		Listen string `json:"listen"`
		Topic  string `json:"topic"`
	} `json:"local"`
	Pulsar string   `json:"pulsar"`
	Etcd   []string `json:"etcd"`
	Redis  struct {
		Addr string `json:"addr"`
		Pwd  string `json:"pwd"`
	} `json:"redis"`
}

func ReadConfig(configFilePath string) (*IMConfig, error) {
	imConfig := &IMConfig{}
	file, err := os.Open(configFilePath)
	if nil != err {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(imConfig); nil != err {
		return nil, err
	}
	return imConfig, nil
}

type IMPublicConfig struct {
	Mysql  string   `json:"mysql"`
	Pulsar string   `json:"pulsar"`
	Etcd   []string `json:"etcd"`
	Redis  struct {
		Addr string `json:"addr"`
		Pwd  string `json:"pwd"`
	} `json:"redis"`
}

func ReadPublicConfig(configFilePath string) (*IMPublicConfig, error) {
	cfg := &IMPublicConfig{}
	file, err := os.Open(configFilePath)
	if nil != err {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(cfg); nil != err {
		return nil, err
	}
	return cfg, nil
}

type IMLocalConfig struct {
	Local struct {
		Listen string `json:"listen"`
		Topic  string `json:"topic"`
	} `json:"local"`
}

func ReadLocalConfig(configFilePath string) (*IMLocalConfig, error) {
	cfg := &IMLocalConfig{}
	file, err := os.Open(configFilePath)
	if nil != err {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(cfg); nil != err {
		return nil, err
	}
	return cfg, nil
}

type MsgServerIpListConfig struct {
	MsgServerIpList []string `json:"msg-server-ip-list"`
}

func (s MsgServerIpListConfig) GetIpAndWeight() []string {
	// eg. "192.168.10.240:5001,10"
	ipAndWeights := []string{}
	for _, v := range s.MsgServerIpList {
		// v value eg: "192.168.10.240:5001,10"
		ipAndWeights = append(ipAndWeights, v)
	}

	return ipAndWeights
}

func ReadMsgSrvIpListConfig(configFilePath string) (*MsgServerIpListConfig, error) {
	cfg := &MsgServerIpListConfig{}
	file, err := os.Open(configFilePath)
	if nil != err {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(cfg); nil != err {
		return nil, err
	}
	return cfg, nil
}

type WSMsgAgent struct {
	Msgaddr   string `json:"msgaddr"`
	WSMsgaddr string `json:"wsmsgaddr"`
}

type WSLoginAgentConfig struct {
	LoginAddr     string        `json:"loginaddr"`
	MsgServerList []*WSMsgAgent `json:"msgservers"`
}

func ReadWSLoginAgentConfig(configFilePath string) (*WSLoginAgentConfig, error) {
	cfg := &WSLoginAgentConfig{}
	file, err := os.Open(configFilePath)
	if nil != err {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(cfg); nil != err {
		return nil, err
	}
	return cfg, nil
}

func ReadWSMsgAgentConfig(configFilePath string) (*WSMsgAgent, error) {
	cfg := &WSMsgAgent{}
	file, err := os.Open(configFilePath)
	if nil != err {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(cfg); nil != err {
		return nil, err
	}
	return cfg, nil
}
