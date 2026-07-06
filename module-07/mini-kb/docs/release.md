# 发布说明

v0.3.0 发布内容：

- 新增 `search_knowledge_base` 工具，用于在课程知识库中检索资料。
- 新增混合检索流程：向量召回、关键词召回、RRF 融合、重排。
- Agent 可以在回答课程问题时主动调用知识库检索工具。

v0.2.0 发布内容：

- 接入 Function Calling。
- 增加 `get_time` 和 `calc` 两个工具。
- 多轮对话会保留工具调用结果，避免出现缺少 `tool_call_id` 的消息。

v0.1.0 发布内容：

- 实现最小 OpenAI 兼容模型调用。
- 支持从环境变量读取 Base URL、API Key 和模型名。
