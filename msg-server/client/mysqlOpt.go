package client

import (
	"instant-message/common-library/log"
)

// 保存最后一条消息时间戳到 mysql
func (c *Client) SaveBaseInfo() {
	_, e := c.orm.DB().Exec("update account set nick_name = ?, last_msg_timestamp = ? where account_id = ?",
		c.account.NickName, c.account.LastMsgTimestamp, c.GetAccountId())
	if e != nil {
		log.Tracef("更新 account: %d 到数据库表 account 失败: error: %s", c.GetAccountId(), e.Error())
	}
}

// 保存联系人里列表到 mysql
func (c *Client) SaveContactInfo() {
	c.UpdateContactInfoInMemory()

	if _, err := c.orm.DB().Exec("update account set contact_list = ? where account_id = ?",
		c.account.ContactList, c.GetAccountId()); err != nil {
		log.Errorf("update account set contact_list = xxx where account_id = %d, err = %v", c.GetAccountId(), err)
	}
}

// SaveBlackListInfo 保存黑名单列表到 mysql
func (c *Client) SaveBlackListInfo() {
	c.UpdateBlackListInfoInMemory()

	if _, err := c.orm.DB().Exec("UPDATE account SET black_list = ? WHERE account_id = ?", c.account.BlackList, c.GetAccountId()); nil != err {
		log.Errorf("UPDATE account SET black_list = xxx WHERE account_id = %d, err = %v", c.GetAccountId(), err)
	}
}

func (c *Client) SaveChatGroupLastMsgTimestamp() {
	// XXX 优化点, 需要批量更新
	// 用 replace into 拼接字符串

	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("开始数据库事务失败,error:%v", err)
		return
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("更新 chat_group_member.last_msg_timestamp 字段时, 回滚事务 tx.Rollback() 出现严重错误: %v", e)
				}
			}
		}
	}()

	for chatGroupId, lastMsgTimestamp := range c.mapChatGroupLastMsgTimestamp {
		if _, err := tx.Exec("update chat_group_member set last_msg_timestamp = ? where account_id = ? and chat_group_id = ?",
			lastMsgTimestamp, c.GetAccountId(), chatGroupId); nil != err {
			errCode = 1
			log.Errorf("update chat_group_member set last_msg_timestamp = %d where account_id = %d and chat_group_id = %d, err = %v", lastMsgTimestamp, c.GetAccountId(), chatGroupId, err)
			return
		}
	}

	if err = tx.Commit(); nil != err {
		errCode = 2
		log.Errorf("更新 chat_group_member.last_msg_timestamp 字段时， 提交事务出现严重错误:%v", err)
		return
	}
}

func (c *Client) SetLastMsgTimestamp(lastMsgTimeStamp int64) {
	c.account.LastMsgTimestamp = lastMsgTimeStamp
}
