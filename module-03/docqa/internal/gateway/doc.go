// Package gateway 是从 M02 搬入的多模型网关。
//
// M03 的文档问答命令通过这个包统一调用大模型，上层代码只处理 prompt、
// 上下文和结果展示，Provider 选择、OpenAI 兼容请求和故障转移都收拢在网关中。
package gateway
