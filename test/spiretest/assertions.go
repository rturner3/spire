package spiretest

import (
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	protoMessageType = reflect.TypeOf((*proto.Message)(nil)).Elem()
)

func RequireErrorContains(tb testing.TB, err error, contains string) {
	tb.Helper()
	if !AssertErrorContains(tb, err, contains) {
		tb.FailNow()
	}
}

func AssertErrorContains(tb testing.TB, err error, contains string) bool {
	tb.Helper()
	if !assert.Error(tb, err) {
		return false
	}
	if !assert.Contains(tb, err.Error(), contains) {
		return false
	}
	return true
}

func RequireGRPCStatus(tb testing.TB, err error, code codes.Code, message string) {
	tb.Helper()
	if !AssertGRPCStatus(tb, err, code, message) {
		tb.FailNow()
	}
}

func AssertGRPCStatus(tb testing.TB, err error, code codes.Code, message string) bool {
	tb.Helper()
	st := status.Convert(err)
	if !assert.Equal(tb, code, st.Code(), "GRPC status code does not match") {
		return false
	}
	if !assert.Equal(tb, message, st.Message(), "GRPC status message does not match") {
		return false
	}
	return true
}

func RequireGRPCStatusContains(tb testing.TB, err error, code codes.Code, contains string) {
	tb.Helper()
	if !AssertGRPCStatusContains(tb, err, code, contains) {
		tb.FailNow()
	}
}

func AssertGRPCStatusContains(tb testing.TB, err error, code codes.Code, contains string) bool {
	tb.Helper()
	st := status.Convert(err)
	if !assert.Equal(tb, code, st.Code(), "GRPC status code does not match") {
		return false
	}
	if !assert.Contains(tb, st.Message(), contains, "GRPC status message does not contain substring") {
		return false
	}
	return true
}

func RequireProtoListEqual(tb testing.TB, expected, actual interface{}) {
	tb.Helper()
	if !AssertProtoListEqual(tb, expected, actual) {
		tb.FailNow()
	}
}

func AssertProtoListEqual(tb testing.TB, expected, actual interface{}) bool {
	tb.Helper()

	ev := reflect.ValueOf(expected)
	et := ev.Type()
	if !assertIsProtoList(tb, et, expectedType) {
		return false
	}

	av := reflect.ValueOf(actual)
	at := av.Type()
	if !assertIsProtoList(tb, at, actualType) {
		return false
	}

	if !assertProtoListSameLength(tb, ev, av, expected, actual) {
		return false
	}

	for i := 0; i < ev.Len(); i++ {
		e := ev.Index(i).Interface().(proto.Message)
		a := av.Index(i).Interface().(proto.Message)
		if !AssertProtoEqual(tb, e, a, "proto %d in list is not equal", i) {
			// get the nice output
			return assert.Equal(tb, expected, actual)
		}
	}

	return true
}

func RequireProtoListElementsMatch(tb testing.TB, expected, actual interface{}) {
	tb.Helper()
	if !AssertProtoListElementsMatch(tb, expected, actual) {
		tb.FailNow()
	}
}

func AssertProtoListElementsMatch(tb testing.TB, expected, actual interface{}) bool {
	tb.Helper()
	ev := reflect.ValueOf(expected)
	et := ev.Type()
	if !assertIsProtoList(tb, et, expectedType) {
		return false
	}

	av := reflect.ValueOf(actual)
	at := av.Type()
	if !assertIsProtoList(tb, at, actualType) {
		return false
	}

	if !assertProtoListSameLength(tb, ev, av, expected, actual) {
		return false
	}

	listLen := ev.Len()
	// Mark indexes in bValue that we already used
	visited := make([]bool, listLen)
	for i := 0; i < listLen; i++ {
		expElem := ev.Index(i).Interface().(proto.Message)
		found := false
		for j := 0; j < listLen; j++ {
			if visited[j] {
				continue
			}

			actualElem := av.Index(j).Interface().(proto.Message)
			if proto.Equal(actualElem, expElem) {
				visited[j] = true
				found = true
				break
			}
		}
		if !found {
			return assert.Fail(tb, "element %s appears more times in %s than in %s", expElem, ev, av)
		}
	}

	return true
}

func RequireProtoEqual(tb testing.TB, expected, actual proto.Message, msgAndArgs ...interface{}) {
	tb.Helper()
	if !AssertProtoEqual(tb, expected, actual, msgAndArgs...) {
		tb.FailNow()
	}
}

func AssertProtoEqual(tb testing.TB, expected, actual proto.Message, msgAndArgs ...interface{}) bool {
	tb.Helper()
	if !proto.Equal(expected, actual) {
		// we've already determined they are not equal, but this will give
		// us nice output with the contents.
		return assert.Equal(tb, expected, actual, msgAndArgs...)
	}
	return true
}

func assertIsProtoList(tb testing.TB, typ reflect.Type, tTyp testType) bool {
	if typ.Kind() != reflect.Slice {
		return assert.Fail(tb, "%s value is not a slice", tTyp)
	}
	if !typ.Elem().Implements(protoMessageType) {
		return assert.Fail(tb, "%s value is not a slice of elements that implement proto.Message", tTyp)
	}

	return true
}

func assertProtoListSameLength(tb testing.TB, ev, av reflect.Value, expected, actual interface{}) bool {
	if !assert.Equal(tb, ev.Len(), av.Len(), "expected %d elements in list; got %d", ev.Len(), av.Len()) {
		return assert.Equal(tb, expected, actual) // we already know these don't match, but get the nice output
	}

	return true
}
