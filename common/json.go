package common

import "dt-server/common/jsoniter"

var (
	json     = jsoniter.ConfigCompatibleWithStandardLibrary
	jsonSafe = jsoniter.ConfigSafeInt64AndNilSlice
)

func JsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func JsonMarshalToString(v interface{}) (string, error) {
	return json.MarshalToString(v)
}

// JsonMarshalSafe 会将 int64/uint64 序列化为字符串，nil slice 序列化为 '[]'
func JsonMarshalSafe(v interface{}) ([]byte, error) {
	return jsonSafe.Marshal(v)
}

// JsonMarshalToStringSafe 会将 int64/uint64 序列化为字符串，nil slice 序列化为 '[]'
func JsonMarshalToStringSafe(v interface{}) (string, error) {
	return jsonSafe.MarshalToString(v)
}

func JsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func JsonUnmarshalFromString(str string, v interface{}) error {
	return json.UnmarshalFromString(str, v)
}
