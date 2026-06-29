// Package transport 承接 M01 的生产级 HTTP 客户端。
//
// Provider 调模型时继续通过 transport.Client 发请求，保留重试、context 取消、
// 连接池等基础能力。
package transport
