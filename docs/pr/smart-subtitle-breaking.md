# feat: 智能字幕断句（基于 Whisper JSON + 词级时间戳）

## Problem

之前 Whisper 配置了 `--max_line_width 42` 和 `--max_line_count 1`，导致字幕被切得非常碎，一句话被分割成多个短句，影响阅读体验。

## Changes

### 新增 `internal/subtitle/whisperjson` 包
**`WhisperOutput` / `Segment` / `Word` 结构体**
- 定义 Whisper JSON 输出的完整结构
- `Word` 包含 `Word` 文本、`Start` / `End` 时间戳、`Probability` 置信度
- 提供 `StartTime()` / `EndTime()` 方法转换为 `time.Duration`

**`Parse()` / `ParseFile()` 解析函数**
- 从 JSON 字节或文件解析
- `FlattenWords()` 将所有 segment 的词合并为一个扁平列表

### 重写 `internal/subtitle/breaker/breaker.go`
**`SubtitleBreaker` 接口**
- `JSONBreaker`: 基于 Whisper JSON + 词级时间戳的智能断句
- `LegacySRTBreaker`: 向后兼容，处理已有 SRT

**`JSONBreaker.Break()` — 智能合并/切分规则**
- **最大字符数**: 40 个汉字（适合中文阅读）
- **最大时长**: 5 秒
- **最大停顿间隔**: 500ms（超过则不合并）
- **句末标点**: 遇到 `。！？` 等句末标点时分块

**辅助函数**
- `endsWithSentencePunct()`: 检测是否以句末标点结尾
- `cleanWord()` / `cleanText()`: 清理文本

**新增导出函数**
- `ProcessJSONFile()`: 从 JSON 文件生成 SRT
- `ProcessJSON()`: 从 JSON 数据生成 SRT

### 重构 `internal/subtitle/whisper/whisper.go`
**`buildArgs()` — 调整 Whisper 参数**
- 移除 `--max_line_width` / `--max_line_count` 限制
- 改为 `--output_format json`（替代 `srt`）
- 保留 `--word_timestamps true`
- 保留 `--vad_filter true` / `--vad_min_silence_duration_ms 400`

**`GenerateSRT()` — JSON 中转流程**
- Whisper 输出 JSON 到视频同目录
- 通过 `breaker.ProcessJSONFile()` 转换为 SRT
- 清理临时 JSON 文件

### 更新 `internal/subtitle/subtitle.go`
- 移除 breaker 二次处理流程（现在 Whisper → JSON → breaker → SRT 是一步到位）
- 保持所有外部 API 不变

## Runtime behavior

```
# 之前：碎句太多
Generating SRT with Whisper... done (Xs) — 27 entries
Applying sentence breaking... done (Yms) — 27 → 25 entries

# 现在：智能合并，块数合理
Generating SRT with Whisper... done (Xs) — 8 entries
```

## 断句逻辑细节

1. **先展平所有词**: 遍历所有 segment，收集 `Word` 到一个列表
2. **逐个词扫描**: 对每个词检查：
   - 加入后是否超过字符限制？
   - 加入后是否超过时长限制？
   - 与前一个词的间隔是否超过 500ms？
   - 前一个词是否以句末标点结尾？
3. **满足任一条件则封块**: 保存当前块，开启新块
4. **最后一块收尾**: 循环结束后处理剩余的词
