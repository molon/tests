package lo

import (
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestMust0(t *testing.T) {
	err := errors.New("mock err")

	must0 := func() {
		defer func() {
			if e := recover(); e != nil {
				require.NotEqual(t, err, e)
			}
		}()
		lo.Must0(err)
	}
	must0()

	panic0 := func() {
		defer func() {
			if e := recover(); e != nil {
				require.Equal(t, err, e)
			}
		}()
		if err != nil {
			panic(err)
		}
	}
	panic0()

	// 综上所述，我们通常还是希望得到原错误，而不仅仅是其 Error() 字符串，所以我们不应该使用 lo.MustXX 函数
}
