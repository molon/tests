# v8go  

[支持 wasm](https://github.com/rogchap/v8go/issues/333) 看起来很有趣，但是没懂它的意义是什么。

## 内存问题

[内存泄漏，这个有可能确实是个问题](https://github.com/rogchap/v8go/issues/367) 并且此处有一些优化方法，例如每次 ctx 都会关闭，每当 isolate 占用内存超过一定标准后就会重建，重建完就会主动 GC
[内存泄漏，但是感觉这个是因为自己搞了太多实例了，没做池子](https://github.com/rogchap/v8go/issues/347)
[内存泄漏，这个没懂](https://github.com/rogchap/v8go/issues/400)

## 参考

[v8go相关的便捷 js 包](https://github.com/kuoruan/v8go-polyfills)