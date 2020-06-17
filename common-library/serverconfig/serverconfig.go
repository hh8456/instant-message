package serverconfig

import (
	"encoding/json"
	"io/ioutil"
)

// XXX 19.3.13 日注释,系统稳定后删除
/*//当在云服务器时由于机器上只有绑定了内网ip的网卡*/
////服务器的监听ip只能是内网ip
//type MatchConfig struct {
//ListenAddr      string
//OuterListenAddr string   //listenAddr 在对应的公网IP
//EtcdProxy       []string // etcd 代理
//RedisAddr       string   // redis地址
//PprofAddr       string   // pprof 的监听地址
//}

//type GameConfig struct {
//MysqlAddr         string // mysql ip 和端口
//MysqlAccountPwd   string // mysql 账号密码
//MysqlDataBase     string // mysql 数据库
//MysqlMaxOpenConns int32  // mysql 最大连接数
//MysqlMaxIdleConns int32  // mysql 最大空闲连接数
//UniqueId          int32
//EtcdProxy         []string // etcd 代理
//ListenAddr        string
//GMAddr            string `toml:"gm_addr"`
//AduitFlag         int32  `toml:"aduit_flag"`
//RedisAddr         string // redis地址
//PprofAddr         string // pprof 的监听地址
//ACU               int32  `toml:"ACU"` // 平均在线人数
//NgPushEnable      bool   `toml:"ngpush_enable"`
//}

//type GateConfig struct {
//ListenAddr      string
//OuterListenAddr string //listenAddr 在对应的公网IP
//UniqueId        int32
//EtcdProxy       []string // etcd 代理
//RedisAddr       string   // redis地址
//PprofAddr       string   // pprof 的监听地址
//}

//type LoginConfig struct {
//MysqlAddr         string   // mysql ip 和端口
//MysqlAccountPwd   string   // mysql 账号密码
//MysqlDataBase     string   // mysql 数据库
//MysqlMaxOpenConns int32    // mysql 最大连接数
//MysqlMaxIdleConns int32    // mysql 最大空闲连接数
//ListenAddr        string   // 内部登录使用的端口
//OuterListenAddr   string   //listenAddr 在对应的公网IP
//EtcdProxy         []string // etcd 代理
//UniqueId          int32
//HttpHost          string //http 用于更新服务状态，以便管理是否允许玩家登陆
//HttpPort          string
//HostID            int32  // 目前用于支付充值配置发货的主键
//PprofAddr         string // pprof 的监听地址
//RedisAddr         string // redis地址
//}

//type ReplyConfig struct {
//ServerID       int32 // 服务器唯一 ID
//MaxRoomNumbers int32 // 房间数上限
//// MatchAddress      string   // match 地址
//ListenPort        int32    // TCP/UDP 监听的本地端口,
//MysqlAddr         string   // mysql ip 和端口
//MysqlAccountPwd   string   // mysql 账号密码
//MysqlDataBase     string   // mysql 数据库
//MysqlMaxOpenConns int32    // mysql 最大连接数
//MysqlMaxIdleConns int32    // mysql 最大空闲连接数
//OuterListenAddr   string   //listenAddr 在对应的公网IP
//RedisAddr         string   // redis地址
//PprofAddr         string   // pprof 的监听地址
//EtcdProxy         []string // etcd 代理
//}

//type ChargeConfig struct {
//ChargeWebServerAddr string `toml:"ChargeWebServerAddr"`
//RedisAddr           string `toml:"RedisAddr"`
//MysqlAddr           string `toml:"MysqlAddr"`
//MysqlAccountPwd     string `toml:"MysqlAccountPwd"`
//MysqlDataBase       string `toml:"MysqlDataBase"`
//// 设置发货回调的地址
//ShipUrlsSetAddr string `toml:"ShipUrlsSetAddr"`
//// 设置发货回调映射关系的hostid
//HostID int32 `toml:"HostID"`
//// 发货地址
//ShipUrls string `toml:"ShipUrls"`
/*}*/

type IpConfig struct {
	Inner_ip string
	Outer_ip string
}

type SdkConfig struct {
	NgPush struct {
		Enable  bool   `json:"enable"`
		ReqAddr string `json:"req_addr"`
		Crypto  string `json:"crypto"`
		Service string `json:"service"`
	} `json:"ngpush"`
	GmSdk struct {
		ReqAddr string `json:"req_addr"`
	} `json:"gmsdk"`
	Charge struct {
		ShipUrl        string `json:"ship_url"`
		ShipUrlSetAddr string `json:"ship_url_set_addr"`
	} `json:"charge"`
}

type EtcdConfig struct {
	Etcd_url []string
}

type MysqlConfig struct {
	Mysql_addr     string
	User           string
	Passwd         string
	Database       string
	Max_open_conns int32
	Max_idle_conns int32
}

type RedisConfig struct {
	Redis_addr string
}

type ChargeConfig struct {
	Inner_listen_port int32
	EnableCharge      bool `json:"enable"`
}

type MatchInfo struct {
	Listen_port int32
	Pprof_port  int32
}

type ManagerInfo struct {
	Inner_http_port int32
	Login_server    string
}

type LoginInfo struct {
	ServerId        int32
	Inner_port      int32
	Pprof_port      int32
	Inner_http_port int32
}

type GateInfo struct {
	ServerId   int32
	Inner_port int32
	Outer_port int32
	Pprof_port int32
}

type GameInfo struct {
	ServerId   int32
	Inner_port int32
	Outer_port int32
	Pprof_port int32
	ACU        int32
	Aduit_flag int32
}

type ReplyInfo struct {
	ServerId         int32
	Inner_port       int32
	Outer_port       int32
	Pprof_port       int32
	Max_room_numbers int32
}

type ServiceConfig struct {
	Match   MatchInfo
	Manager ManagerInfo
	Login   []*LoginInfo
	Gate    []*GateInfo
	Game    []*GameInfo
	Reply   []*ReplyInfo
}

type ServerCfg struct {
	GroupId int32
	Ip      *IpConfig
	Sdk     *SdkConfig
	Etcd    *EtcdConfig
	MySql   *MysqlConfig
	Redis   *RedisConfig
	Charge  *ChargeConfig
	Service *ServiceConfig
}

func Load(filename string) (*ServerCfg, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	v := &ServerCfg{}
	err = json.Unmarshal(data, v)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (sc *ServerCfg) GetDBScourceName() string {
	dataSourceName := sc.MySql.User + ":" + sc.MySql.Passwd + "@tcp(" +
		sc.MySql.Mysql_addr + ")/" + sc.MySql.Database + "?charset=utf8mb4"

	return dataSourceName
}

func (sc *ServerCfg) GetEtcdAddr() []string {
	etcdProxy := []string{}
	for _, v := range sc.Etcd.Etcd_url {
		etcdProxy = append(etcdProxy, v)
	}

	return etcdProxy
}
