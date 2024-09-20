# v8go  

[支持 wasm](https://github.com/rogchap/v8go/issues/333) 看起来很有趣，但是没懂它的意义是什么。

## 内存问题

[内存泄漏，这个有可能确实是个问题](https://github.com/rogchap/v8go/issues/367) 并且此处有一些优化方法，例如每次 ctx 都会关闭，每当 isolate 占用内存超过一定标准后就会重建，重建完就会主动 GC
[内存泄漏，但是感觉这个是因为自己搞了太多实例了，没做池子](https://github.com/rogchap/v8go/issues/347)
[内存泄漏，这个没懂](https://github.com/rogchap/v8go/issues/400)
[为什么需要手动释放](https://github.com/rogchap/v8go/issues/105)
[长驻任务的内存问题解决方案 pr 但没有被接受](https://github.com/rogchap/v8go/pull/230)

## 参考

[v8go相关的便捷 js 包](https://github.com/kuoruan/v8go-polyfills)
[可能是个还不错的 fork](https://github.com/couchbasedeps/v8go) 从 [snej](https://github.com/rogchap/v8go/issues/105#issuecomment-1377654175) 得到的
