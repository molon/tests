package v8go

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"rogchap.com/v8go"
)

// 推荐先去看 goja_test.go
// 这里的测试可对比 v8go 和 goja 的不同行为

// 文档相关
// TerminateExecution: https://v8docs.nodesource.com/node-4.8/d5/dda/classv8_1_1_isolate.html#ab33b5ea0fbd412614931733449c3d659
// CancelTerminateExecution: https://v8docs.nodesource.com/node-4.8/d5/dda/classv8_1_1_isolate.html#accd54de1bf8bdb27a541a578241d4870
// IsExecutionTerminating: https://v8docs.nodesource.com/node-4.8/d5/dda/classv8_1_1_isolate.html#adb638de648962913ddaba7d75348da27
// 以上时和 TerminateExecution 相关的方法，经过调研得出结论
//  1. 在 js 执行过程中需要回调 go 逻辑时，才需要在关键位置通过 IsExecutionTerminating 来判断是否需要 return
//  2. CancelTerminateExecution 这个 v8go 压根就没有暴露，通常我们也用不到，它是在 TerminateExecution 发出后和脚本将异常传递出来之间的这段时间才有意义，
//     即使暴露，也是需要在 js 执行过程中才需要用到做非常细粒度的控制，不太需要关心
func TestTerminateExecution(t *testing.T) {
	const SCRIPT = `
	var i = 0;
	for (;;) {
		i++;
	}
	`

	vm := v8go.NewIsolate()
	defer vm.Dispose()
	// IMPORTANT: NewContext 支持不指定参数调用，但是如果不显式指定 Isolate 的话，就很容易忘记执行 Isolate.Dispose() ，
	// 而 v8ctx.Close() 也不会帮助调用其 Isolate().Dispose() 所以最好养成好习惯
	v8ctx := v8go.NewContext(vm)
	defer v8ctx.Close()

	time.AfterFunc(100*time.Millisecond, func() {
		// 可以在另外一个线程中调用
		v8ctx.Isolate().TerminateExecution()
	})
	val, err := v8ctx.RunScript(SCRIPT, "")
	assert.Nil(t, val)
	assert.ErrorContains(t, err, "ExecutionTerminated")

	{
		// 可以看到即使上面的被中断了，下面的 v8ctx.RunScript() 仍然可以正常执行
		val, err := v8ctx.RunScript(`"hello world"`, "")
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}

	v8ctx.Isolate().TerminateExecution()

	{
		// 可以看到即使上面执行了 TerminateExecution ，下面的 v8ctx.RunScript() 仍然可以正常执行
		// IMPORTANT: 所以这个和 goja 的行为是不一样的，goja 的行为是在无脚本执行时也可标记中断，然后其会影响下一次的脚本执行
		val, err := v8ctx.RunScript(`"hello world"`, "")
		assert.Nil(t, err)
		assert.Equal(t, "hello world", val.String())
	}
}

// 结合上述特点，封装一个方法来实现
// 为什么要返回 terminated ？
// 因为有些场景下是执行过 TerminateExecution 的 Isolate 是不可恢复的，所以此处反馈给调用方
func WrapRun(
	ctx context.Context,
	v8ctx *v8go.Context,
	f func(v8ctx *v8go.Context) (*v8go.Value, error),
) (result *v8go.Value, terminated bool, err error) {
	if ctx.Err() != nil {
		return nil, false, errors.Wrap(ctx.Err(), "context already done")
	}

	stop := context.AfterFunc(ctx, func() {
		v8ctx.Isolate().TerminateExecution()
	})
	defer func() {
		if !stop() {
			terminated = true
		}
	}()

	result, err = f(v8ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, errors.Wrap(ctx.Err(), "failed to run")
		}
		return nil, false, errors.Wrap(err, "failed to run")
	}
	return result, false, nil
}

func TestWrapRun(t *testing.T) {
	const SCRIPT = `
	var i = 0;
	for (;;) {
		i++;
	}
	`

	vm := v8go.NewIsolate()
	defer vm.Dispose()
	v8ctx := v8go.NewContext(vm)
	defer v8ctx.Close()
	{
		// 可以看到 ctx 超时按预期工作
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		val, terminated, err := WrapRun(ctx, v8ctx, func(v8ctx *v8go.Context) (*v8go.Value, error) {
			return v8ctx.RunScript(SCRIPT, "")
		})
		assert.Nil(t, val)
		assert.Equal(t, true, terminated)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		since := time.Since(start)
		assert.GreaterOrEqual(t, since, 200*time.Millisecond)
		assert.Less(t, since, 205*time.Millisecond)
	}

	{
		// 可正常执行其他脚本，按预期工作
		ctx := context.Background()
		val, terminated, err := WrapRun(ctx, v8ctx, func(v8ctx *v8go.Context) (*v8go.Value, error) {
			return v8ctx.RunScript("1 + 2", "")
		})
		assert.Nil(t, err)
		assert.Equal(t, "3", val.String())
		assert.Equal(t, false, terminated)
	}

	v8ctx.Isolate().TerminateExecution()

	{
		// 可正常执行其他脚本，按预期工作
		ctx := context.Background()
		val, terminated, err := WrapRun(ctx, v8ctx, func(v8ctx *v8go.Context) (*v8go.Value, error) {
			return v8ctx.RunScript("1 + 2", "")
		})
		assert.Nil(t, err)
		assert.Equal(t, "3", val.String())
		assert.Equal(t, false, terminated)
	}
}

func TestRetainedValueCount(t *testing.T) {
	vm := v8go.NewIsolate()
	defer vm.Dispose()
	v8ctx := v8go.NewContext(vm)
	defer v8ctx.Close()
	assert.Equal(t, 0, v8ctx.RetainedValueCount())

	val, err := v8ctx.RunScript(`var a = 1`, "")
	assert.Nil(t, err)
	assert.True(t, val.IsUndefined())
	assert.Equal(t, 1, v8ctx.RetainedValueCount())

	val, err = v8ctx.RunScript(`var b = 1`, "")
	assert.Nil(t, err)
	assert.True(t, val.IsUndefined())
	assert.Equal(t, 2, v8ctx.RetainedValueCount())

	// 可以看到即使是上面已经有的变量，再次赋值也会增加 retained value
	// 所以这个 retained value 是一个脚本执行返回值的计数罢了
	val, err = v8ctx.RunScript(`a = 2`, "")
	assert.Nil(t, err)
	assert.Equal(t, int64(2), val.Integer()) // 这样是会返回 a 的值的，和 chrome console 表现一致
	assert.Equal(t, 3, v8ctx.RetainedValueCount())

	val, err = v8ctx.RunScript(`88`, "")
	assert.Nil(t, err)
	assert.Equal(t, int64(88), val.Integer())
	assert.Equal(t, 4, v8ctx.RetainedValueCount())

	// 如果执行错误，最终这个计数才不会增加
	val, err = v8ctx.RunScript(`c`, "")
	assert.ErrorContains(t, err, "ReferenceError: c is not defined")
	assert.Nil(t, val)
	assert.Equal(t, 4, v8ctx.RetainedValueCount())
}

// // TODO: 准备依赖此去测试其内存特点呢，还没测出来
// func TestStat(t *testing.T) {
// 	vm := v8go.NewIsolate()
// 	defer vm.Dispose()

// 	// v8ctx := v8go.NewContext(vm)
// 	// defer v8ctx.Close()

// 	// val, err := v8ctx.RunScript(`var a = 1`, "")
// 	// assert.Nil(t, err)
// 	// assert.True(t, val.IsUndefined())

// 	last := ""
// 	for i := 0; i < 10; i++ {
// 		v8ctx := v8go.NewContext(vm)
// 		val, err := v8ctx.RunScript(`(function() { return 2 })()`, "")
// 		assert.Nil(t, err)
// 		assert.Equal(t, int64(2), val.Integer())
// 		v8ctx.Close()

// 		dataStat, _ := json.Marshal(vm.GetHeapStatistics())
// 		stat := string(dataStat)
// 		if last != stat {
// 			log.Printf("%d: %s", i, stat)
// 			last = stat
// 		}
// 	}
// }
