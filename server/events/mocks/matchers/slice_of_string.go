package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"
)

func AnySliceOfString() []string {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*([]string))(nil)).Elem()))
	var nullValue []string
	return nullValue
}

func EqSliceOfString(value []string) []string {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue []string
	return nullValue
}
