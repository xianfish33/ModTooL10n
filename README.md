# ModTooL10n

Minecraft 模组自动汉化工具 - 基于 LLM 的智能翻译解决方案

## 简介

ModTooL10n 是一款基于大语言模型的 Minecraft 模组自动本地化工具。它能够自动提取模组中的英语语言文件，通过 LLM 进行智能翻译，并将翻译结果重新注入到模组中。

## 功能特性

- **自动翻译**：自动提取、翻译、注入 Minecraft 模组语言文件
- **智能分块**：将语言文件拆分为可管理的块，逐块翻译
- **多模组格式**：支持 Fabric、Forge、NeoForge 模组格式
- **批量处理**：可同时处理多个模组，最多并发 3 个翻译任务
- **流式翻译**：实时显示翻译进度，支持流式输出
- **智能验证**：自动验证翻译结果的结构完整性和键完整性
- **自动重试**：翻译失败时自动重试，最多 3 次
- **提供商管理**：支持添加、编辑、删除多个 LLM 提供商
- **模型选择**：可从提供商获取可用模型列表，灵活选择

## 技术栈

| 技术 | 说明 |
|------|------|
| Go 1.25.0 | 主语言 |
| [Bubbletea](https://github.com/charmbracelet/bubbletea) | TUI 框架 |
| [Lipgloss](https://github.com/charmbracelet/lipgloss) | 终端样式 |
| [Bubbles](https://github.com/charmbracelet/bubbles) | TUI 组件 |
| [BurntSushi/toml](https://github.com/BurntSushi/toml) | TOML 解析（Forge/NeoForge 元数据） |

## 快速开始

```bash
# 编译
go build -o ModTooL10n.exe .

# 运行
./ModTooL10n.exe
```

## 目录结构

```
ModTooL10n/
├── main.go           # 程序入口
├── config/
│   └── config.go     # 配置管理
├── engine/
│   ├── engine.go     # 核心编排
│   ├── types.go      # 数据类型定义
│   ├── translator.go # 翻译逻辑
│   ├── llm.go        # LLM API 客户端
│   ├── jar.go        # JAR 文件处理
│   ├── lang.go       # 语言文件处理
│   └── validator.go  # 验证工具
├── tui/
│   ├── model.go      # TUI 状态管理
│   ├── menu.go       # 主菜单
│   ├── translate.go  # 单个翻译界面
│   ├── batch.go      # 批量翻译界面
│   ├── providers.go  # 提供商管理界面
│   └── styles.go     # 样式定义
└── docs/
    └── 使用指南.md    # 详细使用文档
```

## 使用文档

详见 [docs/使用指南.md](docs/使用指南.md)

## 致谢

- [opencode](https://opencode.ai) - 免费模型额度支持

## 许可证

MIT License
