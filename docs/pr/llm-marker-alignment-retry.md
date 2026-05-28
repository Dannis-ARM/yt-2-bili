# fix: LLM marker alignment with retry

## Problem

LLM 翻译字幕时偶发 `[N]` 标记乱序或丢失，而 `parseTextOutput` 仅按位置匹配、不管编号，导致译文对错条目，视频和字幕时间轴不匹配。

## Changes

### `internal/subtitle/translator.go`

**`parseTextOutput` — 按标记编号精确匹配 + placeholder 检测**
- 从位置匹配（第 i 个标记 → 索引 i）改为按 `[N]` 中的数字 N 对齐
- 返回 `*ParseResult` 替代 `[]string`，包含 `Texts`（对齐后的译文）和 `Issues`（问题列表）
- 检测三类问题：`missing`（标记缺失）、`out_of_order`（标记乱序）、`placeholder`（占位文本）
- `isPlaceholder` 用 7 个正则匹配常见占位模式（如 "这里是第N条内容"、"第N条内容" 等），防止 LLM 偷懒输出假翻译

**`TranslateSRT` — 批次级 3 次重试**
- 每批翻译后立即校验标记，有问题则带纠错 prompt 重试，最多 3 次
- 3 次后仍有问题：缺失条目回填原文，乱序按编号纠正，继续处理
- `TranslationWarnings` 收集所有批次的 warning，最终输出结构化汇总

**`buildRetryPrompt` — 纠错 prompt**
- 包含上一次 LLM 的完整输出 + 问题描述，让 LLM 修补而非重翻
- CRITICAL RULES 明确禁止 placeholder 文本
- 200K 字符截断保护

**`systemPromptSRT`**
- 新增 CRITICAL 规则：禁止使用 "这里是第N条内容" 等占位文本，每条翻译必须忠实反映原文含义

**`validateTranslatedSRT`**
- 恢复返回 error（供缓存失效逻辑使用）
- post-translation 调用点改为 warning 不中断

### `internal/subtitle/subtitle.go`

**`prepareSubtitleFiles`**
- 翻译后校验失败改为 `Warning:` 而非 `FAILED` 中止

## Runtime behavior

```
# 正常：无额外输出

# LLM 丢标 → 重试修复
Batch 2 attempt 1: 2 issue(s), retrying...
Batch 2 attempt 2: 1 issue(s), retrying...

# 3 次后仍有问题 → warning 继续
Translation completed with 5 warnings (2 missing, 1 reordered, 2 placeholder)
  - marker [12] is missing from response
  - marker [34] is missing from response
  - position 7: expected marker [7] but found [8]
  - marker [27] contains placeholder text: "这里是第27条内容。"
  - marker [28] contains placeholder text: "这里是第28条内容。"
```
