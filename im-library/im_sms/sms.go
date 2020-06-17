package im_sms

import (
	"encoding/json"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type SMSConfig struct {
	Url                   string `json:"url"`
	UsernameDomestic      string `json:"username_domestic"`
	PasswordDomestic      string `json:"password_domestic"`
	UsernameForeign       string `json:"username_foreign"`
	PasswordForeign       string `json:"password_foreign"`
	MessageDomesticBefore string `json:"message_domestic_before"`
	MessageDomesticAfter  string `json:"message_domestic_after"`
	MessageForeignBefore  string `json:"message_foreign_before"`
	MessageForeignAfter   string `json:"message_foreign_after"`
}

var (
	ChengLiXinCfg *SMSConfig
)

func init() {
	ChengLiXinCfg = &SMSConfig{}
	getChengLiXinConfig(os.Getenv("IMCONFIGPATH") + "/sms.json")
}

// getChengLiXinConfig 读取Im项目诚立信短信平台相关配置文件
func getChengLiXinConfig(configFilePath string) {
	file, err := os.Open(configFilePath)
	if nil != err {
		panic(err)
	}

	decoder := json.NewDecoder(file)
	if err = decoder.Decode(ChengLiXinCfg); nil != err {
		panic(err)
	}
}

type SmsParam struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Content  string `json:"content"`
	Mobile   string `json:"mobile"`
}

func GetValidateCode(width int) string {
	numeric := [10]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	r := len(numeric)
	rand.Seed(time.Now().UnixNano())

	var sb strings.Builder
	for i := 0; i < width; i++ {
		fmt.Fprintf(&sb, "%d", numeric[rand.Intn(r)])
	}
	return sb.String()
}

// SendSms 调用诚立信第三方接口，发送验证码
func SendSms(phoneNum, verifyCode string, bForeign bool) (string, error) {
	var (
		username string
		password string
		content  string
	)
	if bForeign {
		content = fmt.Sprintf("%s%s%s", ChengLiXinCfg.MessageDomesticBefore, verifyCode, ChengLiXinCfg.MessageDomesticAfter)
		username = ChengLiXinCfg.UsernameDomestic
		password = ChengLiXinCfg.PasswordDomestic
	} else {
		content = fmt.Sprintf("%s%s%s", ChengLiXinCfg.MessageForeignBefore, verifyCode, ChengLiXinCfg.MessageForeignAfter)
		username = ChengLiXinCfg.UsernameForeign
		password = ChengLiXinCfg.PasswordForeign
	}
	values := url.Values{}
	Url, err := url.Parse(ChengLiXinCfg.Url)
	if nil != err {
		log.Errorf("短信url.Parse()出错%v, 诚立信Url:%s", err, ChengLiXinCfg.Url)
		return "", err
	}
	values.Add("username", username)
	values.Add("content", content)
	values.Add("password", function.Md5_32bit(username+function.Md5_32bit(password)))
	values.Add("mobile", phoneNum)
	Url.RawQuery = values.Encode()
	urlPath := Url.String()
	resp, err := http.Get(urlPath)
	if nil != err {
		log.Errorf("调用http.get()短信接口出错%v, urlPath:=%s", err, urlPath)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Errorf("")
		return "", err
	}
	return string(bodyBytes), nil
}
