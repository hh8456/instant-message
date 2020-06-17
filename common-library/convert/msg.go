package convert

import (
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"

	"instant-message/common-library/log"
	"instant-message/common-library/proto/common"
)

// TODO 这里要删除
/*// FillProtoMsg convert buffer to proto msg*/
//func FillProtoMsg(b *hbuffer.Buffer, msg proto.Message) error {
//err := proto.Unmarshal(b.GetRestOfBytes(), msg)
//if err != nil {
//return err
//}

//return nil
/*}*/

// RGBColorArrayToStr 将color数组组合成str
func RGBColorArrayToStr(colorArr []*common.RGBColor) string {
	var ret string

	for _, v := range colorArr {
		colorV := convertRGBColorToInt64(v)
		if colorV < 0 {
			ret = ""
			break
		}
		ret += strconv.FormatInt(colorV, 10) + ","
	}

	return ret
}

// convertRGBColorToInt64 将color值组合为一个int64的值
func convertRGBColorToInt64(color *common.RGBColor) int64 {
	var ret int64

	for once := true; once; once = false {
		r := color.GetColor_R()
		if r > 255 || r < 0 { // 8位，取值范围0 - 255
			ret = -10
			break
		}
		g := color.GetColor_G()
		if g > 255 || g < 0 {
			ret = -11
			break
		}
		b := color.GetColor_B()
		if b > 255 || b < 0 {
			ret = -12
			break
		}
		alpha := color.GetColor_Alpha()
		if alpha > 100 || alpha < 0 { // [0 - 100]
			ret = -13
			break
		}

		var combineNum int64
		combineNum |= int64(r)
		combineNum = combineNum<<8 | int64(g)
		combineNum = combineNum<<8 | int64(b)
		combineNum = combineNum<<8 | int64(alpha)
		ret = combineNum
	}

	return ret
}

// convertInt64ToRGBColor 将int64的值转换为RGBColor
func convertInt64ToRGBColor(combineNum int64) *common.RGBColor {
	var ret *common.RGBColor

	for once := true; once; once = false {
		r := combineNum >> 24 & 0xFF
		g := combineNum >> 16 & 0xFF
		b := combineNum >> 8 & 0xFF
		alpha := combineNum & 0xFF

		ret = &common.RGBColor{
			Color_R:     proto.Int32(int32(r)),
			Color_G:     proto.Int32(int32(g)),
			Color_B:     proto.Int32(int32(b)),
			Color_Alpha: proto.Int32(int32(alpha)),
		}
	}

	return ret
}

// StrToRGBColorArray 将str转换为RGBColor数组
func StrToRGBColorArray(str string) []*common.RGBColor {
	if str == "" {
		return nil
	}

	ret := make([]*common.RGBColor, 0)
	f := func(c rune) bool {
		return c == ','
	}
	valueStrArr := strings.FieldsFunc(str, f)
	for _, valueStr := range valueStrArr {
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			log.Tracef("wrong color data: %v", valueStr)
			return nil
		}
		ret = append(ret, convertInt64ToRGBColor(value))
	}

	return ret
}
