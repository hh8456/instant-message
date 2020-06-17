package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"instant-message/common-library/log"
	"instant-message/im-library/im_audio_video/huanxin"
)

/* 注意，环信http请求返回正常，也有可能出错，返回的是错误描述等 */

// GetHuanXinToken 获取环信Token
func GetHuanXinToken() *huanxin.TokenResponse {
	accessToken, err := huanxin.GetHuanXinAccessToken()
	if nil != err {
		log.Errorf("获取环信Token失败:", err)
		return nil
	}

	return accessToken
}

// GetHuanXinAccount 获取环信用户 （查询不到密码）
func GetHuanXinAccount(token, username string) (*huanxin.GetUserResponse, error) {
	response, err := huanxin.GetHuanXinUser(token, username)
	if nil != err {
		return nil, err
	}

	getUserResponse := &huanxin.GetUserResponse{}
	err = json.Unmarshal([]byte(response), getUserResponse)
	if nil == err && nil != getUserResponse.Entities {
		return getUserResponse, nil
	}

	serverError := &huanxin.ServerError{}
	err = json.Unmarshal([]byte(response), serverError)
	if nil == err {
		return nil, errors.New(fmt.Sprintf("获取环信用户时，获取http请求正常，返回错误：%v", serverError))
	}
	return nil, errors.New(fmt.Sprintf("获取环信用户时，出现未知错误"))
}

// CreateHuanXinAccount 创建环信账户
func CreateHuanXinAccount(token, username, password, nickName string) (*huanxin.CreateUserResponse, error) {
	response, err := huanxin.CreateHuanXinUser(token, username, password, nickName)
	if nil != err {
		return nil, err
	}

	createUserResponse := &huanxin.CreateUserResponse{}
	err = json.Unmarshal([]byte(response), createUserResponse)
	if nil == err && nil != createUserResponse.Entities {
		return createUserResponse, nil
	}

	serverError := &huanxin.ServerError{}
	err = json.Unmarshal([]byte(response), serverError)
	if nil == err {
		return nil, errors.New(fmt.Sprintf("注册环信用户时，获取http请求正常，返回错误：%v", serverError))
	}
	return nil, errors.New(fmt.Sprintf("注册环信用户时，出现未知错误"))
}

// CreateHuanXinConferences 创建环信音频、视频会议
func CreateHuanXinConferences(conferencesType, memDefaultRole int, token, huanXinUserId, password string) (*huanxin.CreateConferencesResponse, error) {
	response, err := huanxin.CreateHuanXinConferences(conferencesType, memDefaultRole, token, huanXinUserId, password)
	if nil != err {
		return nil, err
	}

	createConferencesResponse := &huanxin.CreateConferencesResponse{}
	err = json.Unmarshal([]byte(response), createConferencesResponse)
	if nil == err {
		return createConferencesResponse, nil
	}

	serverError := &huanxin.ServerError{}
	err = json.Unmarshal([]byte(response), serverError)
	if nil == err {
		return nil, errors.New(fmt.Sprintf("创建环信音频、视频会议时，获取http请求正常，返回错误：%v", serverError))
	}
	return nil, errors.New(fmt.Sprintf("创建环信音频、视频会议时，出现未知错误"))
}

// DissolveHuanXinConferences 解散环信音频、视频会议
func DissolveHuanXinConferences(token, huanXinConferenceId string) (*huanxin.CreateConferencesResponse, error) {
	response, err := huanxin.DissolveHuanXinConferences(token, huanXinConferenceId)
	if nil != err {
		return nil, err
	}

	createConferencesResponse := &huanxin.CreateConferencesResponse{}
	err = json.Unmarshal([]byte(response), createConferencesResponse)
	if nil == err {
		return createConferencesResponse, nil
	}

	serverError := &huanxin.ServerError{}
	err = json.Unmarshal([]byte(response), serverError)
	if nil == err {
		return nil, errors.New(fmt.Sprintf("解散环信音频、视频会议时，获取http请求正常，返回错误：%v", serverError))
	}
	return nil, errors.New(fmt.Sprintf("解散环信音频、视频会议时，出现未知错误"))
}
