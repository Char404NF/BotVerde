package auto

import "encoding/json"

func jsonUnmarshal(s string, v any) bool { return json.Unmarshal([]byte(s), v) == nil }
