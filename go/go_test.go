package std

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNil(t *testing.T) {
	var a any
	require.True(t, a == nil)

	a = (*time.Time)(nil)
	require.False(t, a == nil)

	a = func() *time.Time { return nil }()
	require.False(t, a == nil)

	b := func() *time.Time { return nil }()
	require.True(t, b == nil)

	a = b
	require.False(t, a == nil)

	a = func() any { return nil }()
	require.True(t, a == nil)

	a = func() any { return (*time.Time)(nil) }()
	require.False(t, a == nil)

	// 综上，对于一个 any 类型变量来说，只要其值有类型，其就不是 nil
}
