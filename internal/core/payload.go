package core

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// ResolvePayload 按优先级从 req 取载荷:
//  1. Payload(已解码字节, 由调用方填充)
//  2. PayloadHex
//  3. PayloadB64
//  4. PayloadTxt
//
// 优先级顺序: 显式字节 > Hex > B64 > Txt。
func ResolvePayload(req *Request) ([]byte, error) {
	if req.Payload != nil {
		return req.Payload, nil
	}
	if req.PayloadHex != "" {
		h := strings.TrimSpace(req.PayloadHex)
		// 容忍 "AA BB CC" / "AABBCC" / "aa:bb" 等写法
		compact := strings.NewReplacer(" ", "", ":", "", "-", "", "\n", "", "\t", "").Replace(h)
		if compact == "" {
			return nil, nil
		}
		b, err := hex.DecodeString(compact)
		if err != nil {
			return nil, errors.New("payloadHex 非法: " + err.Error() + " (期望偶数个十六进制字符)")
		}
		return b, nil
	}
	if req.PayloadB64 != "" {
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.PayloadB64))
		if err != nil {
			return nil, errors.New("payloadB64 非法: " + err.Error())
		}
		return b, nil
	}
	if req.PayloadTxt != "" {
		return []byte(req.PayloadTxt), nil
	}
	return nil, nil // 空载荷
}

// FormatDataHex 把字节格式化为空格分隔的十六进制。
func FormatDataHex(b []byte) string {
	return strings.TrimSpace(hex.EncodeToString(b))
}

// FormatDataTxt 把字节按可打印字符显示, 不可打印的转义为 \xNN。
func FormatDataTxt(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c >= 0x20 && c < 0x7f {
			sb.WriteByte(c)
		} else {
			sb.WriteString("\\x" + hex.EncodeToString([]byte{c}))
		}
	}
	return sb.String()
}
