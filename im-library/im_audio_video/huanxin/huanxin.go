package huanxin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type huanXinConfig struct {
	Url          string `json:"url"`
	OrgName      string `json:"org_name"`
	AppName      string `json:"app_name"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type ServerError struct {
	Error            string `json:"error"`
	Timestamp        int64  `json:"timestamp"`
	Duration         int64  `json:"duration"`
	Exception        string `json:"exception"`
	ErrorDescription string `json:"error_description"`
}

var (
	huanXinCfg                  *huanXinConfig
	usersUrl                    string
	userUrl                     string
	tokenUrl                    string
	createConferencesUrl        string
	dissolveConferencesUrl      string
	kickOutConferencesMemberUrl string
	getConferencesUrl           string
)

const (
	ConferencesRoleTypeAudience  = 1  // 会议角色：观众
	ConferencesRoleTypeAnchor    = 3  // 会议角色：主播
	ConferencesRoleTypeManager   = 7  // 会议角色：管理员（拥有主播权限）
	ConferencesTypeNormal        = 10 // 会议模式：普通模式
	ConferencesTypeConferences   = 11 // 会议模式：大众模式
	ConferencesTypeDirectSeeding = 12 // 会议模式：直播模式
)

func init() {
	huanXinCfg = &huanXinConfig{}
	getHuanXinConfig(os.Getenv("IMCONFIGPATH") + "/huanxin.json")
	baseUrl := huanXinCfg.Url + "/" + huanXinCfg.OrgName + "/" + huanXinCfg.AppName
	tokenUrl = baseUrl + "/token"
	usersUrl = baseUrl + "/users"
	userUrl = baseUrl + "/users" + "/{username}"
	createConferencesUrl = baseUrl + "/conferences"
	dissolveConferencesUrl = baseUrl + "/{confrId}"
	kickOutConferencesMemberUrl = baseUrl + "/{confrId}" + "/{userName}"
	getConferencesUrl = baseUrl + "/conferences" + "/{confrId}"
}

// getHuanXinConfig 读取Im项目环信相关配置文件
func getHuanXinConfig(configFilePath string) {
	file, err := os.Open(configFilePath)
	if nil != err {
		panic(err)
	}

	decoder := json.NewDecoder(file)
	if err = decoder.Decode(huanXinCfg); nil != err {
		panic(err)
	}
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Application string `json:"application"`
}

// GetHuanXinAccessToken 获取环信会话Token
func GetHuanXinAccessToken() (*TokenResponse, error) {
	b, err := json.Marshal(&TokenRequest{
		GrantType:    "client_credentials",
		ClientId:     huanXinCfg.ClientId,
		ClientSecret: huanXinCfg.ClientSecret,
	})
	if err != nil {
		return nil, err
	}

	response, err := sendRequest(tokenUrl, bytes.NewBuffer([]byte(b)), "POST", "")
	if err != nil {
		return nil, err
	}

	tokenResponse := &TokenResponse{}
	if nil != json.Unmarshal([]byte(response), tokenResponse) {
		return nil, err
	}

	return tokenResponse, nil
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type CreateUserEntities struct {
	Uuid      string `json:"uuid"`
	Type      string `json:"type"`
	Created   int64  `json:"created"`
	Modified  int64  `json:"modified"`
	Username  string `json:"username"`
	Activated bool   `json:"activated"`
}

type CreateUserResponse struct {
	Path            string               `json:"path"`
	Uri             string               `json:"uri"`
	Timestamp       int64                `json:"timestamp"`
	Organization    string               `json:"organization"`
	Application     string               `json:"application"`
	Entities        []CreateUserEntities `json:"entities"`
	Action          string               `json:"action"`
	Duration        int64                `json:"duration"`
	ApplicationName string               `json:"applicationName"`
}

// CreateHuanXinUser 创建环信用户
func CreateHuanXinUser(token, username, password, nickName string) (string, error) {
	b, err := json.Marshal(&CreateUserRequest{
		Username: username,
		Password: password,
		Nickname: nickName,
	})
	if nil != err {
		return "", err
	}
	return sendRequest(usersUrl, bytes.NewBuffer(b), "POST", token)
}

type GetUserEntities struct {
	Created   int64  `json:"created"`
	Nickname  string `json:"nickname"`
	Modified  int64  `json:"modified"`
	Type      string `json:"type"`
	Uuid      string `json:"uuid"`
	Username  string `json:"username"`
	Activated bool   `json:"activated"`
}

type GetUserResponse struct {
	Path      string            `json:"path"`
	Uri       string            `json:"uri"`
	Timestamp int64             `json:"timestamp"`
	Entities  []GetUserEntities `json:"entities"`
	Count     int64             `json:"count"`
	Action    string            `json:"action"`
	Duration  int64             `json:"duration"`
}

// GetHuanXinUser 获取环信用户
func GetHuanXinUser(token, username string) (string, error) {
	return sendRequest(resolveTemplate(userUrl, "username", username), nil, "GET", token)
}

type GetDataBasicResponse struct {
	Cursor string `json:"cursor"`
	Count  int    `json:"count"`
}

// GetHuanXinUsers 获取环信用户
func GetHuanXinUsers(token, cursor string, limit int) (string, error) {
	url := usersUrl
	if 0 < limit {
		url += fmt.Sprintf("?limit=%s", strconv.Itoa(limit))
	}
	if "" != cursor {
		url += fmt.Sprintf("&cursor=%s", cursor)
	}

	return sendRequest(url, nil, "GET", token)
}

type CreateConferencesRequest struct {
	ConfrType         int    `json:"confrType"`         // 10: 普通模式 11: 大会议模式 12: 直播模式
	Password          string `json:"password"`          // 指定密码时，将使用此密码；不指定，将由服务端生成
	ConfrDelayMillis  int64  `json:"confrDelayMillis"`  // 会议创建后，保留时间。服务端创建了会议后，如果在confrDelayMillis之内没有人加入会议，将会被系统强制解散。但是一旦有人成功进入会后，当最后一人离开会议，会议会立即被销毁；不会再保留confrDelayMillis时间。缺省时，服务器统一配置为150秒。
	MemDefaultRole    int    `json:"memDefaultRole"`    // 会议成员默认角色。用户A通过会议 ID 密码获取加入会议后的角色就是这个 1:观众，3:主播，7:管理员（拥有主播权限）。 缺省时，根据会议类型设置，目前规则如下：普通模式默认主播；大会议模式默认主播；直播模式默认观众
	AllowAudienceTalk bool   `json:"allowAudienceTalk"` // true 允许观众上麦
	Creator           string `json:"creator"`           // 指定创建者，creator 将会成为这个会议的管理员，拥有管理员权限
	Rec               bool   `json:"rec"`               // true 此会议将被录制
	RecMerge          bool   `json:"recMerge"`          // true 此会议的所有通话将被合并到一个文件
}

type CreateConferencesResponse struct {
	Type              int    `json:"type"`              // 10: 普通模式 11: 大会议模式 12: 直播模式
	TalkerLimit       int    `json:"talkerLimit"`       // 主播上限数，大会议模式全部是是主播
	Id                string `json:"id"`                // 会议ID
	Password          string `json:"password"`          // 会议密码
	AllowAudienceTalk bool   `json:"allowAudienceTalk"` // 允许观众上麦，大会议模式时忽略此项
	AudienceLimit     int    `json:"audienceLimit"`     // 观众上限数，大会议模式无观众
	ExpireDate        string `json:"expireDate"`        // 过期时间，创建会议后，如果在 expireDate 之前没有人加入会议，将会被系统强制解散
}

// CreateHuanXinConferences 创建环信音频、视频会议
func CreateHuanXinConferences(conferencesType, memDefaultRole int, token, huanXinUserId, password string) (string, error) {
	b, err := json.Marshal(&CreateConferencesRequest{
		ConfrType: conferencesType,
		Password:  password,
		// ConfrDelayMillis:  150,
		MemDefaultRole:    memDefaultRole,
		AllowAudienceTalk: true,
		Creator:           huanXinUserId,
		Rec:               false,
		RecMerge:          false,
	})
	if err != nil {
		return "", err
	}

	return sendRequest(createConferencesUrl, bytes.NewBuffer([]byte(b)), "POST", token)
}

// DissolveHuanXinConferences 解散环信音频、视频会议
func DissolveHuanXinConferences(token, huanXinConferenceId string) (string, error) {
	return sendRequest(resolveTemplate(dissolveConferencesUrl, "confrId", huanXinConferenceId), nil, "DELETE", token)
}

// KickOutHuanXinConferencesMember 踢出环信音频、视频会议中的某一个成员
func KickOutHuanXinConferencesMember(token, huanXinConferenceId, huanXinConferenceMemberId string) (string, error) {
	kickOutConferencesMemberUrlTmp := resolveTemplate(kickOutConferencesMemberUrl, "confrId", huanXinConferenceId)
	kickOutConferencesMemberUrlTmp = resolveTemplate(kickOutConferencesMemberUrlTmp, "userName", huanXinConferenceMemberId)
	return sendRequest(kickOutConferencesMemberUrlTmp, nil, "DELETE", token)
}

type GetHuanXinConferencesResponse struct {
	Type              int    `json:"type"`              // 10: 普通模式 11: 大会议模式 12: 直播模式
	TalkerLimit       int    `json:"talkerLimit"`       // 主播上限数，大会议模式全部是是主播
	Id                string `json:"id"`                // 会议ID
	Password          string `json:"password"`          // 会议密码
	AllowAudienceTalk bool   `json:"allowAudienceTalk"` // 允许观众上麦，大会议模式时忽略此项
	AudienceLimit     int    `json:"audienceLimit"`     // 观众上限数，大会议模式无观众
	ExpireDate        string `json:"expireDate"`        // 过期时间，创建会议后，如果在 expireDate 之前没有人加入会议，将会被系统强制解散
}

// GetHuanXinConferences 通过 huanXinConferenceId 获取环信音频、视频会议
func GetHuanXinConferences(token, huanXinConferenceId string) (string, error) {
	return sendRequest(resolveTemplate(getConferencesUrl, "confrId", huanXinConferenceId), nil, "GET", token)
}

func resolveTemplate(template string, variable string, value string) string {
	return strings.Replace(template, "{"+variable+"}", value, -1)
}

// sendRequest 向环信服务器发送请求
func sendRequest(url string, body io.Reader, method string, token string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := client.Do(req)
	result, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return "", err
	}

	return string(result), nil
}
