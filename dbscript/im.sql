/*
Navicat MySQL Data Transfer

Source Server         : ubtSrv-dev
Source Server Version : 50727
Source Host           : 117.50.95.182:3306
Source Database       : im

Target Server Type    : MYSQL
Target Server Version : 50727
File Encoding         : 65001

Date: 2019-11-11 17:07:41
*/

SET FOREIGN_KEY_CHECKS=0;

-- ----------------------------
-- Table structure for `account`
-- ----------------------------
DROP TABLE IF EXISTS `account`;
CREATE TABLE `account` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL DEFAULT 0 ,
`nick_name`  varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT 'noname' COMMENT '昵称' ,
`contact_list`  blob NULL COMMENT '联系人列表' ,
`last_msg_timestamp`  bigint(20) NOT NULL DEFAULT 0 COMMENT '收到的最后一条消息的时间戳' ,
`creation_time`  datetime(6) NOT NULL COMMENT '角色创建时间' ,
`pwd`  varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL ,
PRIMARY KEY (`id`),
UNIQUE INDEX `account` (`account_id`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1754

;

-- ----------------------------
-- Table structure for `chat_group`
-- ----------------------------
DROP TABLE IF EXISTS `chat_group`;
CREATE TABLE `chat_group` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`chat_group_id`  bigint(20) NOT NULL COMMENT '聊天群唯一 id' ,
`creator`  bigint(20) NOT NULL COMMENT '创建者 id' ,
`name`  varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '群名字' ,
`pic`  varbinary(2048) NOT NULL COMMENT '群头像' ,
PRIMARY KEY (`id`),
UNIQUE INDEX `chat_group_id` (`chat_group_id`) USING BTREE ,
INDEX `creator` (`creator`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=106

;

-- ----------------------------
-- Table structure for `chat_group_create_log`
-- ----------------------------
DROP TABLE IF EXISTS `chat_group_create_log`;
CREATE TABLE `chat_group_create_log` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`group_id`  bigint(20) NOT NULL ,
`creator`  bigint(20) NOT NULL ,
`create_datetime`  datetime(6) NOT NULL ,
PRIMARY KEY (`id`),
INDEX `groupid` (`group_id`) USING BTREE ,
INDEX `creator` (`creator`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=108

;

-- ----------------------------
-- Table structure for `chat_group_destroy_log`
-- ----------------------------
DROP TABLE IF EXISTS `chat_group_destroy_log`;
CREATE TABLE `chat_group_destroy_log` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增id' ,
`group_id`  bigint(20) NOT NULL COMMENT '群id' ,
`creater`  bigint(20) NOT NULL COMMENT '群主id' ,
`destroy_datetime`  datetime(6) NOT NULL COMMENT '解散群时间' ,
PRIMARY KEY (`id`),
INDEX `groupId` (`group_id`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=13

;

-- ----------------------------
-- Table structure for `chat_group_member`
-- ----------------------------
DROP TABLE IF EXISTS `chat_group_member`;
CREATE TABLE `chat_group_member` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL ,
`chat_group_id`  bigint(20) NOT NULL ,
`nick_name`  varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '群里的昵称' ,
`join_chat_group_datetime`  datetime(6) NOT NULL COMMENT '加入群的时间' ,
`last_msg_timestamp`  bigint(20) NOT NULL COMMENT '收到的最后一条群消息时间戳' ,
PRIMARY KEY (`id`),
UNIQUE INDEX `account_group` (`account_id`, `chat_group_id`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=302

;

-- ----------------------------
-- Table structure for `chat_group_req_join_log`
-- ----------------------------
DROP TABLE IF EXISTS `chat_group_req_join_log`;
CREATE TABLE `chat_group_req_join_log` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`chat_group_id`  bigint(20) NOT NULL ,
`requester_id`  bigint(20) NOT NULL ,
`request_message`  varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '请求信息' ,
`processed`  int(6) NOT NULL COMMENT '0 是未处理, 1 是处理过; 拒绝或者同意加群,都算处理过' ,
`req_uid`  bigint(20) NOT NULL ,
`req_date`  datetime(6) NOT NULL ,
PRIMARY KEY (`id`),
INDEX `requester_id` (`requester_id`) USING BTREE ,
INDEX `chat_group_id` (`chat_group_id`) USING BTREE ,
INDEX `req_uid` (`req_uid`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `friend_req`
-- ----------------------------
DROP TABLE IF EXISTS `friend_req`;
CREATE TABLE `friend_req` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`receiver_id`  bigint(20) NOT NULL ,
`sender_id`  bigint(20) NOT NULL ,
`req_datetime`  datetime(6) NOT NULL ,
PRIMARY KEY (`id`),
UNIQUE INDEX `UQE_friend_req_pk` (`receiver_id`, `sender_id`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=312

;

-- ----------------------------
-- Table structure for `friend_verification`
-- ----------------------------
DROP TABLE IF EXISTS `friend_verification`;
CREATE TABLE `friend_verification` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`friend_id`  bigint(20) NOT NULL ,
`account_id`  bigint(20) NOT NULL ,
`become_friend_datetime`  datetime(6) NOT NULL ,
PRIMARY KEY (`id`),
UNIQUE INDEX `UQE_friend_verification_pk` (`friend_id`, `account_id`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=129

;

-- ----------------------------
-- Table structure for `private_chat_record`
-- ----------------------------
DROP TABLE IF EXISTS `private_chat_record`;
CREATE TABLE `private_chat_record` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`sender_id`  bigint(20) NOT NULL ,
`receiver_id`  bigint(20) NOT NULL ,
`srv_timestamp`  bigint(20) NOT NULL ,
`msg_datetime`  datetime(6) NOT NULL ,
`msg_content`  varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL ,
`msg_type`  varchar(12) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL ,
PRIMARY KEY (`id`),
INDEX `senderId_receiverId_datetime` (`sender_id`, `receiver_id`, `msg_datetime`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `test`
-- ----------------------------
DROP TABLE IF EXISTS `test`;
CREATE TABLE `test` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`name`  int(10) NOT NULL ,
PRIMARY KEY (`id`, `name`)
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `weibo`
-- ----------------------------
DROP TABLE IF EXISTS `weibo`;
CREATE TABLE `weibo` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL ,
`photos_bin`  blob NULL ,
`msg_content`  varchar(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL ,
`srv_timestamp`  bigint(20) NOT NULL ,
`msg_datetime`  datetime(6) NOT NULL ,
`thumbup_times`  int(10) NOT NULL DEFAULT 0 COMMENT '点赞次数' ,
PRIMARY KEY (`id`),
INDEX `accountId_msgDatetime` (`account_id`, `msg_datetime`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `weibo_action_log`
-- ----------------------------
DROP TABLE IF EXISTS `weibo_action_log`;
CREATE TABLE `weibo_action_log` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL ,
`srv_timestamp`  bigint(20) NOT NULL COMMENT '用户发微博,点赞,在别人微博下留言时的时间戳' ,
`action`  int(6) NOT NULL DEFAULT 0 COMMENT '0 发微博, 1 点赞, 2 在某个朋友的微博下留言' ,
`friend_id`  bigint(20) NULL DEFAULT 0 COMMENT '只有为别人点赞或留言时, friend_id 才有意义' ,
`friend_srv_timestamp`  bigint(20) NULL DEFAULT NULL COMMENT '只有为别人点赞或留言时, friend_srv_timestamp 是对方的微博 id' ,
`action_datetime`  datetime(6) NOT NULL COMMENT '个人动态产生的时间' ,
PRIMARY KEY (`id`),
INDEX `accountId_srvTimestamp` (`account_id`, `srv_timestamp`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `weibo_deleted`
-- ----------------------------
DROP TABLE IF EXISTS `weibo_deleted`;
CREATE TABLE `weibo_deleted` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL ,
`photos_bin`  blob NULL ,
`msg_content`  varchar(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL ,
`srv_timestamp`  bigint(20) NOT NULL ,
`msg_datetime`  datetime(6) NOT NULL ,
`thumbup_times`  int(10) NOT NULL DEFAULT 0 COMMENT '点赞次数' ,
`del_datetime`  datetime(6) NOT NULL ,
PRIMARY KEY (`id`),
INDEX `accountId_msgDatetime_delDatetime` (`account_id`, `msg_datetime`, `del_datetime`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `weibo_like`
-- ----------------------------
DROP TABLE IF EXISTS `weibo_like`;
CREATE TABLE `weibo_like` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL ,
`srv_timestamp`  bigint(20) NOT NULL ,
`liker_id`  bigint(20) NOT NULL COMMENT '点赞者信息集合' ,
`thumbup_timestamp`  datetime(6) NOT NULL COMMENT '点赞时刻' ,
PRIMARY KEY (`id`),
INDEX `accountId_srvTimestamp_thumbupTimestamp` (`account_id`, `srv_timestamp`, `thumbup_timestamp`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Table structure for `weibo_reply`
-- ----------------------------
DROP TABLE IF EXISTS `weibo_reply`;
CREATE TABLE `weibo_reply` (
`id`  bigint(20) NOT NULL AUTO_INCREMENT ,
`account_id`  bigint(20) NOT NULL ,
`srv_timestamp`  bigint(20) NOT NULL ,
`sender_id`  bigint(20) NOT NULL ,
`receiver_id`  bigint(20) NULL DEFAULT NULL ,
`msg_content`  varchar(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL ,
`reply_datetime`  datetime(6) NOT NULL ,
PRIMARY KEY (`id`),
INDEX `accountId_srvTimestamp` (`account_id`, `srv_timestamp`) USING BTREE 
)
ENGINE=InnoDB
DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
AUTO_INCREMENT=1

;

-- ----------------------------
-- Auto increment value for `account`
-- ----------------------------
ALTER TABLE `account` AUTO_INCREMENT=1754;

-- ----------------------------
-- Auto increment value for `chat_group`
-- ----------------------------
ALTER TABLE `chat_group` AUTO_INCREMENT=106;

-- ----------------------------
-- Auto increment value for `chat_group_create_log`
-- ----------------------------
ALTER TABLE `chat_group_create_log` AUTO_INCREMENT=108;

-- ----------------------------
-- Auto increment value for `chat_group_destroy_log`
-- ----------------------------
ALTER TABLE `chat_group_destroy_log` AUTO_INCREMENT=13;

-- ----------------------------
-- Auto increment value for `chat_group_member`
-- ----------------------------
ALTER TABLE `chat_group_member` AUTO_INCREMENT=302;

-- ----------------------------
-- Auto increment value for `chat_group_req_join_log`
-- ----------------------------
ALTER TABLE `chat_group_req_join_log` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `friend_req`
-- ----------------------------
ALTER TABLE `friend_req` AUTO_INCREMENT=312;

-- ----------------------------
-- Auto increment value for `friend_verification`
-- ----------------------------
ALTER TABLE `friend_verification` AUTO_INCREMENT=129;

-- ----------------------------
-- Auto increment value for `private_chat_record`
-- ----------------------------
ALTER TABLE `private_chat_record` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `test`
-- ----------------------------
ALTER TABLE `test` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `weibo`
-- ----------------------------
ALTER TABLE `weibo` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `weibo_action_log`
-- ----------------------------
ALTER TABLE `weibo_action_log` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `weibo_deleted`
-- ----------------------------
ALTER TABLE `weibo_deleted` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `weibo_like`
-- ----------------------------
ALTER TABLE `weibo_like` AUTO_INCREMENT=1;

-- ----------------------------
-- Auto increment value for `weibo_reply`
-- ----------------------------
ALTER TABLE `weibo_reply` AUTO_INCREMENT=1;
