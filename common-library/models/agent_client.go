package models

type AgentClient struct {
	Id              int64 `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AgentId         int64 `json:"agent_id" xorm:"not null comment('代理 id') unique(agent_client) BIGINT(20)"`
	ThirdPlatformId int64 `json:"third_platform_id" xorm:"not null BIGINT(20)"`
	ClientId        int64 `json:"client_id" xorm:"not null comment('代理的下级客户 id') unique(agent_client) BIGINT(20)"`
}
