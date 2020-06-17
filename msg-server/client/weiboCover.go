package client

import (
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"strconv"
	"time"
)

func init() {
	// 用户设置朋友圈相册封面图片
	registerLogicCall(msg_id.NetMsgId_C2SSetWeiboCover, C2SSetWeiboCover)
	// 用户点赞好友朋友圈相册封面图片
	registerLogicCall(msg_id.NetMsgId_C2SLikeFriendWeiboCover, C2SLikeFriendWeiboCover)
	// 获取某人朋友圈相册封面（可以包括自己)
	registerLogicCall(msg_id.NetMsgId_C2SGetWeiboCover, C2SGetWeiboCover)
}

// C2SSetWeiboCover 用户设置朋友圈相册封面图片
func C2SSetWeiboCover(c *Client, msg []byte) {
	setWeiboCover := &msg_struct.C2SSetWeiboCover{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], setWeiboCover, "msg_struct.C2SSetWeiboCover") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SSetWeiboCover_has_error, 0)
		return
	}

	/* 朋友圈相册封面不能为空 */
	if "" == setWeiboCover.WeiboCoverPic {
		c.SendErrorCode(msg_err_code.MsgErrCode_set_weibo_cover_cover_picture_is_empty, 0)
		return
	}

	/* 朋友圈相册封面存入redis */
	if err := setWeiboCoverRds(c, setWeiboCover.WeiboCoverPic); nil != err {
		log.Errorf("%d设置朋友圈相册首页图片%s存入redis失败，err:%v", c.GetAccountId(), setWeiboCover.WeiboCoverPic, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_set_weibo_cover_cover_picture_is_empty, 0)
		return
	}

	/* 朋友圈相册封面存入数据库 */
	if err := setWeiboCoverDB(c, setWeiboCover.WeiboCoverPic); nil != err {
		log.Errorf("%d设置朋友圈相册首页图片%s存入数据库失败，err:%v", c.GetAccountId(), setWeiboCover.WeiboCoverPic, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_set_weibo_cover_cover_picture_is_empty, 0)
		return
	}

	/* 用户设置或更换朋友圈相册封面，不需通知任何人，直接回复 */
	c.SendPbMsg(msg_id.NetMsgId_S2CSetWeiboCover, &msg_struct.S2CSetWeiboCover{
		Ret: "Success",
	})

}

// C2SLikeFriendWeiboCover 用户点赞好友朋友圈相册封面图片
func C2SLikeFriendWeiboCover(c *Client, msg []byte) {
	likeFriendWeiboCover := &msg_struct.C2SLikeFriendWeiboCover{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], likeFriendWeiboCover, "msg_struct.C2SLikeFriendWeiboCover") {
		return
	}

	/* 判断用户是否已经为此朋友圈相册封面点赞 */

	if err := likeWeiboCoverDB(c); nil != err {
		return
	}

	if err := likeWeiboCoverRds(c, likeFriendWeiboCover.FriendId); nil != err {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CLikeFriendWeiboCover, &msg_struct.S2CLikeFriendWeiboCover{
		Ret: "Success",
	})
}

// C2SGetWeiboCover 获取某人朋友圈相册封面（可以包括自己)
func C2SGetWeiboCover(c *Client, msg []byte) {
	getWeiboCover := &msg_struct.C2SGetWeiboCover{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], getWeiboCover, "msg_struct.C2SGetWeiboCover") {
		return
	}

	picUrl, _ := GetWeiboCoverRds(c, getWeiboCover.UserId)
	bLiked, _ := DoesLikeFriendWeiboCover(c, getWeiboCover.UserId)
	c.SendPbMsg(msg_id.NetMsgId_S2CGetWeiboCover, &msg_struct.S2CGetWeiboCover{
		StrWeiboCoverPicUrl: picUrl,
		IsLikeWeiboCover:    bLiked,
		Ret:                 "Success",
	})
}

/* 数据库 */
// setWeiboCoverDB 朋友圈相册封面，存入数据库
func setWeiboCoverDB(c *Client, picAddr string) error {
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("用户:%d设置朋友圈相册封面时，开始数据库事务失败,error:%v", c.GetAccountId(), err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("用户:%d设置朋友圈相册封面时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), e)
				}
			}
		}
	}()

	weiboCover := &models.WeiboCover{}
	bGet, err := tx.Where("account_id = ?", c.GetAccountId()).Get(weiboCover)
	if nil != err {
		errCode = 1
		log.Errorf("用户:%d设置更换朋友圈相册封面时，从数据库获取原朋友圈相册封面首页 models.WeiboCover 失败:%v", c.GetAccountId(), err)
		return err
	}

	if bGet {
		// 更换朋友圈相册封面
		insertNum, err := tx.Insert(&models.WeiboCoverDeleteLog{
			AccountId:    weiboCover.AccountId,
			CoverUrl:     weiboCover.CoverUrl,
			DateTime:     weiboCover.DateTime,
			ThumbugTimes: weiboCover.ThumbupTimes,
		})
		if 1 != insertNum || nil != err {
			errCode = 2
			log.Errorf("用户:%d更换朋友圈相册封面时，存入表 weibo_cover_delete_log 失败，原相册封面详情:%v,err:%v", c.GetAccountId(), weiboCover, err)
			return err
		}

		_, err = tx.DB().Exec("DELETE FROM weibo_cover WHERE account_id = ?", c.GetAccountId())
		if nil != err {
			errCode = 3
			log.Errorf("用户:%d更换朋友圈相册封面时，从表 weibo_cover 删除失败，原相册封面详情:%v,err:%v", c.GetAccountId(), weiboCover, err)
			return err
		}
	}

	_, err = tx.Insert(&models.WeiboCover{
		AccountId: c.GetAccountId(),
		CoverUrl:  picAddr,
		DateTime:  time.Now(),
	})
	if nil != err {
		errCode = 4
		log.Errorf("用户:%d更换朋友圈相册封面时，存入表 WeiboCover 失败err:%v", c.GetAccountId(), err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("用户:%d更换朋友圈相册封面时, 提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), err)
	}

	return err
}

// likeWeiboCoverDB 点赞朋友圈相册，存入数据库
func likeWeiboCoverDB(c *Client) error {
	_, err := c.orm.DB().Exec("UPDATE weibo_cover SET thumbup_times = thumbup_times + 1")
	return err
}

// setWeiboCoverRds 朋友圈相册封面存入redis
func setWeiboCoverRds(c *Client, picAddr string) error {
	/* 从 redis 删除以前点赞朋友圈相册封面的用户id */
	_, err := c.rdsWeiboCoverLiker.Del(c.GetStrAccount())
	if nil != err {
		return err
	}

	/* 用户设置或更换朋友圈相册封面存入redis */
	return c.rdsWeiboCover.Set(c.GetStrAccount(), picAddr)
}

/* redis */
// likeWeiboCoverRds 点赞朋友圈相册封面，存入redis
func likeWeiboCoverRds(c *Client, friendId int64) error {
	_, err := c.rdsWeiboCoverLiker.AddSetMembers(strconv.FormatInt(friendId, 10), c.GetAccountId())
	return err
}

// GetWeiboCoverRds 获取用户userId朋友圈相册封面
func GetWeiboCoverRds(c *Client, userId int64) (string, error) {
	return c.rdsWeiboCover.Get(strconv.FormatInt(userId, 10))
}

// doesLikeFriendWeiboCover “我”是否为好友朋友圈相册封面点过赞
func DoesLikeFriendWeiboCover(c *Client, friendId int64) (bool, error) {
	num, err := c.rdsWeiboCoverLiker.IsSetMember(strconv.FormatInt(friendId, 10), c.GetStrAccount())
	return num == 1, err
}
