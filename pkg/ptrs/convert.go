package ptrs

import "k8s.io/apimachinery/pkg/util/intstr"

func Int64(v int64) *int64 {
	return &v
}

func Int(v int) *int {
	return &v
}

func Int32(v int32) *int32 {
	return &v
}

func String(v string) *string {
	return &v
}

func Bool(v bool) *bool {
	return &v
}

func True() *bool {
	return Bool(true)
}

func False() *bool {
	return Bool(false)
}

func IntOrStr(v intstr.IntOrString) *intstr.IntOrString {
	return &v
}

func IntOrStrFromStr(v string) *intstr.IntOrString {
	intorstr := intstr.FromString(v)
	return &intorstr
}

func IntOrStrFromInt(v int) *intstr.IntOrString {
	intorstr := intstr.FromInt(v)
	return &intorstr
}
